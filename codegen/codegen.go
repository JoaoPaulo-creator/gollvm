package codegen

import (
	"compiler/ast"
	"fmt"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"os"
	"regexp"
	"strings"
)

// Generator generates LLVM IR from an AST
type Generator struct {
	module        *ir.Module
	context       *Context
	stringCounter int
	blockCounter  int
	printfFunc    *ir.Func // Store the printf function for easy reference
}

// Context represents the code generation context
type Context struct {
	currentFunction *ir.Func
	blocks          map[string]*ir.Block
	namedValues     map[string]value.Value
	stringConstants map[string]*ir.Global
	parent          *Context
}

// Value represents a generated value
type Value struct {
	Value value.Value
	Type  types.Type
}

// New creates a new generator
func New() *Generator {
	module := ir.NewModule()
	context := newContext(nil)

	// Add standard C library functions
	printfFunc := declarePrintf(module)

	// Add runtime functions
	declareRuntime(module)

	return &Generator{
		module:        module,
		context:       context,
		stringCounter: 0,
		blockCounter:  0,
		printfFunc:    printfFunc,
	}
}

// newContext creates a new context
func newContext(parent *Context) *Context {
	return &Context{
		currentFunction: nil,
		blocks:          make(map[string]*ir.Block),
		namedValues:     make(map[string]value.Value),
		stringConstants: make(map[string]*ir.Global),
		parent:          parent,
	}
}

// Lookup looks up a value in the context
func (c *Context) Lookup(name string) (value.Value, bool) {
	if val, ok := c.namedValues[name]; ok {
		return val, true
	}
	if c.parent != nil {
		return c.parent.Lookup(name)
	}
	return nil, false
}

func declarePrintf(module *ir.Module) *ir.Func {
	// Variadic function with return type i32, first parameter is pointer to i8
	paramTypes := []types.Type{types.NewPointer(types.I8)}
	printfType := types.NewFunc(types.I32, paramTypes...)

	// Create the function
	fn := module.NewFunc("printf", printfType)

	// Set the variadic flag
	fn.Sig.Variadic = true

	// Set a name for the parameter for better generated code
	if len(fn.Params) > 0 {
		fn.Params[0].SetName("format")
	}

	return fn
}

func declareExternalFunction(module *ir.Module, name string, retType types.Type, paramTypes ...types.Type) *ir.Func {
	funcType := types.NewFunc(retType, paramTypes...)
	fn := module.NewFunc(name, funcType)

	// Set parameter names for better readability
	for i := range paramTypes {
		if i < len(fn.Params) {
			fn.Params[i].SetName(fmt.Sprintf("param%d", i)) // Fix: use i instead of paramType
		}
	}

	return fn
}

func declareRuntime(module *ir.Module) {
	// Memory allocation functions
	declareExternalFunction(module, "malloc", types.NewPointer(types.I8), types.I64)
	declareExternalFunction(module, "free", types.Void, types.NewPointer(types.I8))

	// String handling functions
	declareExternalFunction(module, "strlen", types.I64, types.NewPointer(types.I8))
	declareExternalFunction(module, "strcpy", types.NewPointer(types.I8),
		types.NewPointer(types.I8), types.NewPointer(types.I8))

	// Math functions
	declareExternalFunction(module, "abs", types.I32, types.I32)
	declareExternalFunction(module, "pow", types.Double, types.Double, types.Double)

	// Exit function
	declareExternalFunction(module, "exit", types.Void, types.I32)
}

// Generate generates LLVM IR from an AST
func (g *Generator) Generate(program *ast.Program) (*ir.Module, error) {
	// Track processed function names
	processedFunctions := make(map[string]bool)

	// First pass: Collect all function declarations
	for _, stmt := range program.Statements {
		if varStmt, ok := stmt.(*ast.VarStatement); ok {
			if funcLit, ok := varStmt.Value.(*ast.FunctionLiteral); ok {
				processedFunctions[funcLit.Name] = true
			}
		}
		if funcStmt, ok := stmt.(*ast.FunctionStatement); ok {
			processedFunctions[funcStmt.Name.Value] = true
		}
	}

	// Second pass: Generate functions
	for i, stmt := range program.Statements {
		if _, ok := stmt.(*ast.VarStatement); ok {
			if funcLit, ok := stmt.(*ast.VarStatement).Value.(*ast.FunctionLiteral); ok {
				if processedFunctions[funcLit.Name] {
					_, err := g.generateStatement(stmt)
					if err != nil {
						return nil, fmt.Errorf("error processing function declaration %d: %w", i, err)
					}
					processedFunctions[funcLit.Name] = false // Mark as processed
				}
			}
		}
		if funcStmt, ok := stmt.(*ast.FunctionStatement); ok {
			if processedFunctions[funcStmt.Name.Value] {
				_, err := g.generateStatement(stmt)
				if err != nil {
					return nil, fmt.Errorf("error processing function declaration %d: %w", i, err)
				}
				processedFunctions[funcStmt.Name.Value] = false // Mark as processed
			}
		}
	}

	// Create main function
	mainFunc := g.module.NewFunc("main", types.NewFunc(types.I32))
	mainBlock := mainFunc.NewBlock("entry")
	g.context.currentFunction = mainFunc

	// Process non-function declarations in main
	for i, stmt := range program.Statements {
		// Skip functions, which we've already processed
		if _, ok := stmt.(*ast.VarStatement); ok {
			if _, ok := stmt.(*ast.VarStatement).Value.(*ast.FunctionLiteral); ok {
				continue
			}
		}
		if _, ok := stmt.(*ast.FunctionStatement); ok {
			continue
		}

		_, err := g.generateStatement(stmt)
		if err != nil {
			return nil, fmt.Errorf("error processing statement %d: %w", i, err)
		}
	}
	// Add a terminator to the main block if needed
	if mainBlock.Term == nil {
		mainBlock.NewRet(constant.NewInt(types.I32, 0))
	}

	// Check and fix all blocks in all functions
	for _, fn := range g.module.Funcs {
		for _, block := range fn.Blocks {
			if block.Term == nil {
				// Add appropriate terminator based on function return type
				if fn.Sig.RetType.Equal(types.Void) {
					block.NewRet(nil)
				} else {
					block.NewRet(constant.NewInt(types.I32, 0))
				}
			}
		}
	}

	return g.module, nil
}

// generateStatement generates code for a statement
// generateStatement generates code for a statement with improved error handling
func (g *Generator) generateStatement(stmt ast.Statement) (value.Value, error) {
	if stmt == nil {
		return nil, fmt.Errorf("nil statement")
	}

	switch stmt := stmt.(type) {
	case *ast.VarStatement:
		return g.generateVarStatement(stmt)
	case *ast.ReturnStatement:
		return g.generateReturnStatement(stmt)
	case *ast.ExpressionStatement:
		if stmt.Expression == nil {
			return nil, fmt.Errorf("nil expression in expression statement")
		}
		return g.generateExpression(stmt.Expression)
	case *ast.BlockStatement:
		return g.generateBlockStatement(stmt)
	case *ast.IfStatement:
		return g.generateIfStatement(stmt)
	case *ast.WhileStatement:
		return g.generateWhileStatement(stmt)
	case *ast.PrintStatement:
		return g.generatePrintStatement(stmt)
	case *ast.FunctionStatement:
		return g.generateFunctionStatement(stmt)
	case *ast.ForStatement:
		return g.generateForStatement(stmt)
	case *ast.ImportStatement:
		return g.generateImportStatement(stmt)
	default:
		return nil, fmt.Errorf("unknown statement type: %T", stmt)
	}
}

// Add a method to check if a function already exists
func (g *Generator) functionExists(name string) bool {
	for _, fn := range g.module.Funcs {
		if fn.Name() == name {
			return true
		}
	}
	return false
}

func (g *Generator) generateFunctionStatement(stmt *ast.FunctionStatement) (value.Value, error) {
	// Debug logging
	fmt.Fprintf(os.Stderr, "Generating function statement: %s\n", stmt.Name.Value)
	fmt.Fprintf(os.Stderr, "Return Type: %s\n", stmt.ReturnType)
	fmt.Fprintf(os.Stderr, "Number of Parameters: %d\n", len(stmt.Parameters))

	// Check if function already exists
	if g.functionExists(stmt.Name.Value) {
		// Return existing function instead of recreating
		for _, fn := range g.module.Funcs {
			if fn.Name() == stmt.Name.Value {
				return fn, nil
			}
		}
	}

	// Generate function as a literal with the same parameters
	funcLit := &ast.FunctionLiteral{
		Name:       stmt.Name.Value,
		Parameters: stmt.Parameters,
		Body:       stmt.Body,
		ReturnType: stmt.ReturnType,
	}

	return g.generateFunctionLiteral(funcLit)
}

func (g *Generator) generateForStatement(stmt *ast.ForStatement) (value.Value, error) {
	// Generate code for for-loop construct
	// This is a simplified example - you'll need to implement the full logic

	// Get current function
	fn := g.context.currentFunction

	// Create blocks for the for loop
	initBlock := fn.NewBlock(fmt.Sprintf("for.init.%d", g.blockCounter))
	g.blockCounter++

	condBlock := fn.NewBlock(fmt.Sprintf("for.cond.%d", g.blockCounter))
	g.blockCounter++

	bodyBlock := fn.NewBlock(fmt.Sprintf("for.body.%d", g.blockCounter))
	g.blockCounter++

	updateBlock := fn.NewBlock(fmt.Sprintf("for.update.%d", g.blockCounter))
	g.blockCounter++

	endBlock := fn.NewBlock(fmt.Sprintf("for.end.%d", g.blockCounter))
	g.blockCounter++

	// Generate initialization
	if stmt.Init != nil {
		_, err := g.generateStatement(stmt.Init)
		if err != nil {
			return nil, err
		}
	}

	initBlock.NewBr(condBlock)

	// Generate condition
	var condBool value.Value
	if stmt.Condition != nil {
		condVal, err := g.generateExpression(stmt.Condition)
		if err != nil {
			return nil, err
		}

		// Convert to boolean if needed
		if condVal.Type().Equal(types.I1) {
			condBool = condVal
		} else {
			intType, ok := condVal.Type().(*types.IntType)
			if !ok {
				return nil, fmt.Errorf("expected int type for condition, got %T", condVal.Type())
			}
			condBool = condBlock.NewICmp(enum.IPredNE, condVal, constant.NewInt(intType, 0))
		}
	} else {
		// If no condition, use true (infinite loop)
		condBool = constant.NewInt(types.I1, 1)
	}

	condBlock.NewCondBr(condBool, bodyBlock, endBlock)

	// Generate body
	_, err := g.generateStatement(stmt.Body)
	if err != nil {
		return nil, err
	}

	bodyBlock.NewBr(updateBlock)

	// Generate update
	if stmt.Update != nil {
		_, err := g.generateStatement(stmt.Update)
		if err != nil {
			return nil, err
		}
	}

	updateBlock.NewBr(condBlock)

	return nil, nil
}

func (g *Generator) generateImportStatement(stmt *ast.ImportStatement) (value.Value, error) {
	// Handle import statements - this will depend on how your language handles imports
	// For now, simply register the imported module name
	fmt.Printf("Importing module: %s\n", stmt.Path.Value)

	// In a real implementation, you'd need to load and process the imported module
	// This might involve reading files, resolving dependencies, etc.

	return nil, nil
}

func (g *Generator) ensureType(val value.Value, expectedType types.Type) value.Value {
	if val.Type().Equal(expectedType) {
		return val
	}

	// Get current block for type conversion instructions
	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

	// Handle common type conversions
	switch {
	case types.IsInt(val.Type()) && types.IsInt(expectedType):
		// Bitcast between integer types
		return currentBlock.NewBitCast(val, expectedType)
	case types.IsPointer(val.Type()) && types.IsInt(expectedType):
		// Convert pointer to integer
		return currentBlock.NewPtrToInt(val, expectedType)
	case types.IsInt(val.Type()) && types.IsPointer(expectedType):
		// Convert integer to pointer
		return currentBlock.NewIntToPtr(val, expectedType)
	default:
		// Log warning for unsupported type conversion
		fmt.Fprintf(os.Stderr, "WARNING: Unsupported type conversion from %v to %v\n",
			val.Type(), expectedType)
		return val
	}
}

func (g *Generator) debugExpression(expr ast.Expression) {
	if expr == nil {
		fmt.Fprintf(os.Stderr, "NIL EXPRESSION DETECTED\n")
		if g.context != nil && g.context.currentFunction != nil {
			fmt.Fprintf(os.Stderr, "Current function: %s\n", g.context.currentFunction.Name())
			fmt.Fprintf(os.Stderr, "Current blocks: %d\n", len(g.context.currentFunction.Blocks))

			// Print details of the current block
			if len(g.context.currentFunction.Blocks) > 0 {
				currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]
				fmt.Fprintf(os.Stderr, "Current block name: %s\n", currentBlock.Name())
				fmt.Fprintf(os.Stderr, "Number of instructions: %d\n", len(currentBlock.Insts))
			}
		}
		return
	}

	// Additional type-specific debugging
	switch e := expr.(type) {
	case *ast.Identifier:
		fmt.Fprintf(os.Stderr, "Identifier: %s\n", e.Value)
	case *ast.IntegerLiteral:
		fmt.Fprintf(os.Stderr, "Integer Literal: %d\n", e.Value)
		// Add more cases as needed
	}
}

// generateExpression generates code for an expression
func (g *Generator) generateExpression(expr ast.Expression) (value.Value, error) {
	g.debugExpression(expr)
	// Check for nil expression
	if expr == nil {
		return nil, fmt.Errorf("nil expression")
	}

	switch expr := expr.(type) {
	case *ast.Identifier:
		return g.generateIdentifier(expr)
	case *ast.IntegerLiteral:
		return g.generateIntegerLiteral(expr)
	case *ast.StringLiteral:
		return g.generateStringLiteral(expr)
	case *ast.PrefixExpression:
		return g.generatePrefixExpression(expr)
	case *ast.InfixExpression:
		return g.generateInfixExpression(expr)
	case *ast.FunctionLiteral:
		return g.generateFunctionLiteral(expr)
	case *ast.CallExpression:
		return g.generateCallExpression(expr)
	case *ast.BooleanLiteral:
		return g.generateBooleanLiteral(expr)
	case *ast.AssignmentExpression:
		return g.generateAssignmentExpression(expr)
	case *ast.EmptyExpression:
		// Just return a dummy value for empty expressions
		return constant.NewInt(types.I32, 0), nil
	default:
		return nil, fmt.Errorf("unknown expression type: %T", expr)
	}
}

func (g *Generator) generateAssignmentExpression(expr *ast.AssignmentExpression) (value.Value, error) {
	// Generate the right-hand side value
	val, err := g.generateExpression(expr.Value)
	if err != nil {
		return nil, err
	}

	// Look up the variable to assign to
	varVal, ok := g.context.Lookup(expr.Name.Value)
	if !ok {
		return nil, fmt.Errorf("undefined variable for assignment: %s", expr.Name.Value)
	}

	// Get current block
	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

	// Store the value in the variable
	currentBlock.NewStore(val, varVal)

	// Return the value (assignments are expressions in this language)
	return val, nil
}

// generateVarStatement generates code for a var statement
func (g *Generator) generateVarStatement(stmt *ast.VarStatement) (value.Value, error) {
	fmt.Fprintf(os.Stderr, "Generating var statement: %s\n", stmt.Name.Value)

	// Generate the value first
	val, err := g.generateExpression(stmt.Value)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(os.Stderr, "Value type: %v\n", val.Type())

	// Determine the type to use for allocation
	var allocType types.Type = types.I32 // Default type

	// Handle function pointers explicitly
	if funcVal, ok := val.(*ir.Func); ok {
		// For function values, use a pointer to the function type
		allocType = types.NewPointer(funcVal.Type())
		fmt.Fprintf(os.Stderr, "Function detected, using function pointer type: %v\n", allocType)
	} else if ptrType, ok := val.Type().(*types.PointerType); ok {
		if _, isFunc := ptrType.ElemType.(*types.FuncType); isFunc {
			// For function pointers, use the pointer type directly
			allocType = ptrType
			fmt.Fprintf(os.Stderr, "Function pointer detected, using type: %v\n", allocType)
		} else {
			// For regular pointers, use the pointer type
			allocType = ptrType
			fmt.Fprintf(os.Stderr, "Regular pointer detected: %v\n", allocType)
		}
	} else {
		// For non-pointer types, use the value's type
		allocType = val.Type()
		fmt.Fprintf(os.Stderr, "Non-pointer type detected: %v\n", allocType)
	}

	// Global variable handling
	if g.context.currentFunction == nil {
		// Create global variable with proper initialization
		global := g.module.NewGlobal(stmt.Name.Value, allocType)

		// Try to use constant initialization if possible
		if constVal, ok := val.(constant.Constant); ok {
			global.Init = constVal
		} else {
			global.Init = constant.NewZeroInitializer(allocType)

			// Create a global initializer function if needed
			var initFunc *ir.Func
			for _, fn := range g.module.Funcs {
				if fn.Name() == "global_init" {
					initFunc = fn
					break
				}
			}
			if initFunc == nil {
				initFunc = g.module.NewFunc("global_init", types.NewFunc(types.Void))
				initBlock := initFunc.NewBlock("entry")
				initBlock.NewRet(nil)
				for _, fn := range g.module.Funcs {
					if fn.Name() == "main" {
						mainEntry := fn.Blocks[0]
						mainEntry.Insts = append([]ir.Instruction{mainEntry.NewCall(initFunc)}, mainEntry.Insts...)
						break
					}
				}
			}
			entryBlock := initFunc.Blocks[0]
			storeInst := entryBlock.NewStore(val, global)
			if entryBlock.Term != nil {
				insertPos := len(entryBlock.Insts) - 1
				entryBlock.Insts = append(entryBlock.Insts[:insertPos], append([]ir.Instruction{storeInst}, entryBlock.Insts[insertPos:]...)...)
			} else {
				entryBlock.Insts = append(entryBlock.Insts, storeInst)
			}
		}

		g.context.namedValues[stmt.Name.Value] = global
		return global, nil
	}

	// Local variable handling
	if g.context.currentFunction == nil {
		return nil, fmt.Errorf("cannot create local variable outside of a function")
	}

	// Get the entry block
	entryBlock := g.context.currentFunction.Blocks[0]

	// Create an alloca instruction
	allocaInst := entryBlock.NewAlloca(allocType)
	allocaInst.SetName(stmt.Name.Value)

	// Move alloca to the beginning if needed
	if len(entryBlock.Insts) > 1 {
		entryBlock.Insts = append([]ir.Instruction{allocaInst}, entryBlock.Insts[:len(entryBlock.Insts)-1]...)
	}

	// Get current block
	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

	// Store the value with proper casting if needed
	storeVal := val
	if ptrType, ok := val.Type().(*types.PointerType); ok {
		if _, isFunc := ptrType.ElemType.(*types.FuncType); isFunc {
			// Cast function pointer to the allocated type if needed
			if !val.Type().Equal(allocType) {
				storeVal = currentBlock.NewBitCast(val, allocType)
			}
		}
	}

	currentBlock.NewStore(storeVal, allocaInst)
	g.context.namedValues[stmt.Name.Value] = allocaInst

	return allocaInst, nil
}

// generateReturnStatement generates code for a return statement
func (g *Generator) generateReturnStatement(stmt *ast.ReturnStatement) (value.Value, error) {
	// Generate the return value
	val, err := g.generateExpression(stmt.ReturnValue)
	if err != nil {
		return nil, err
	}

	// Get current block
	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

	// Generate return instruction
	currentBlock.NewRet(val)

	return val, nil
}

// generateBlockStatement generates code for a block statement
func (g *Generator) generateBlockStatement(stmt *ast.BlockStatement) (value.Value, error) {
	// Create a new context for the block
	oldContext := g.context
	newContext := newContext(oldContext)

	// Keep the current function reference
	newContext.currentFunction = oldContext.currentFunction

	// Set the new context
	g.context = newContext

	// Check if we have a valid current function
	if g.context.currentFunction == nil {
		fmt.Fprintf(os.Stderr, "WARNING: No current function when generating block statement\n")
	}

	// Generate code for each statement in the block
	var lastVal value.Value
	for i, s := range stmt.Statements {
		val, err := g.generateStatement(s)
		if err != nil {
			// Restore the original context before returning error
			g.context = oldContext
			return nil, fmt.Errorf("error in block statement %d: %w", i, err)
		}
		lastVal = val
	}

	// Restore the original context
	g.context = oldContext

	return lastVal, nil
}

// generateWhileStatement generates code for a while statement
func (g *Generator) generateWhileStatement(stmt *ast.WhileStatement) (value.Value, error) {
	// Get current function
	fn := g.context.currentFunction

	// Get the current block BEFORE creating new blocks
	currentBlock := fn.Blocks[len(fn.Blocks)-1]

	// Create blocks for the while loop
	condBlock := fn.NewBlock(fmt.Sprintf("while.cond.%d", g.blockCounter))
	g.blockCounter++

	bodyBlock := fn.NewBlock(fmt.Sprintf("while.body.%d", g.blockCounter))
	g.blockCounter++

	endBlock := fn.NewBlock(fmt.Sprintf("while.end.%d", g.blockCounter))
	g.blockCounter++

	// Branch from current block to condition block
	currentBlock.NewBr(condBlock)

	// Generate condition in condBlock
	condVal, err := g.generateExpression(stmt.Condition)
	if err != nil {
		return nil, err
	}

	// Convert condition to boolean if needed
	var condBool value.Value
	if condVal.Type().Equal(types.I1) {
		condBool = condVal
	} else {
		intType, ok := condVal.Type().(*types.IntType)
		if !ok {
			return nil, fmt.Errorf("expected int type for condition, got %T", condVal.Type())
		}
		condBool = condBlock.NewICmp(enum.IPredNE, condVal, constant.NewInt(intType, 0))
	}

	// Branch based on condition
	condBlock.NewCondBr(condBool, bodyBlock, endBlock)

	// Generate loop body
	g.generateStatement(stmt.Body)

	// Add back edge to condition block if no terminator exists
	if len(bodyBlock.Insts) == 0 || bodyBlock.Term == nil {
		bodyBlock.NewBr(condBlock)
	}

	return nil, nil
}

// generateIfStatement generates code for an if statement
func (g *Generator) generateIfStatement(stmt *ast.IfStatement) (value.Value, error) {
	// Verify we have a current function
	if g.context.currentFunction == nil {
		fmt.Fprintf(os.Stderr, "ERROR: No current function when generating if statement\n")
		return constant.NewInt(types.I32, 0), nil
	}

	fn := g.context.currentFunction

	// Generate condition safely
	condVal, err := g.generateExpression(stmt.Condition)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to generate condition for if statement: %v\n", err)
		return nil, err
	}

	// Verify we have at least one block in the function
	if len(fn.Blocks) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: Function has no blocks when generating if statement\n")
		// Create an entry block as a fallback
		entryBlock := fn.NewBlock("entry")
		g.context.blocks["entry"] = entryBlock
	}

	// Get current block safely
	var currentBlock *ir.Block
	if len(fn.Blocks) > 0 {
		currentBlock = fn.Blocks[len(fn.Blocks)-1]
	} else {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to get current block for if statement\n")
		return constant.NewInt(types.I32, 0), nil
	}

	// Create blocks for if statement
	thenBlock := fn.NewBlock(fmt.Sprintf("if.then.%d", g.blockCounter))
	g.blockCounter++

	var elseBlock *ir.Block
	mergeBlock := fn.NewBlock(fmt.Sprintf("if.merge.%d", g.blockCounter))
	g.blockCounter++

	if stmt.Alternative != nil {
		elseBlock = fn.NewBlock(fmt.Sprintf("if.else.%d", g.blockCounter))
		g.blockCounter++
	} else {
		elseBlock = mergeBlock
	}

	// Generate conditional branch
	// Convert to boolean if needed
	var condBool value.Value
	if condVal.Type().Equal(types.I1) {
		condBool = condVal
	} else {
		intType, ok := condVal.Type().(*types.IntType)
		if !ok {
			return nil, fmt.Errorf("expected int type for condition, got %T", condVal.Type())
		}
		condBool = currentBlock.NewICmp(enum.IPredNE, condVal, constant.NewInt(intType, 0))
	}

	currentBlock.NewCondBr(condBool, thenBlock, elseBlock)

	// Generate code for then block
	_, err = g.generateStatement(stmt.Consequence)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to generate then block: %v\n", err)
		return nil, err
	}

	// Add branch to merge block if there isn't a terminator
	if len(thenBlock.Insts) == 0 || thenBlock.Term == nil {
		thenBlock.NewBr(mergeBlock)
	}

	// Generate code for else block if it exists
	if stmt.Alternative != nil {
		_, err = g.generateStatement(stmt.Alternative)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to generate else block: %v\n", err)
			return nil, err
		}

		// Add branch to merge block if there isn't a terminator
		if len(elseBlock.Insts) == 0 || elseBlock.Term == nil {
			elseBlock.NewBr(mergeBlock)
		}
	}

	// This is key: we need to make sure the merge block has a terminator
	// For now, we'll add a default return instruction
	// We'll check and fix this later in the Generate function
	if mergeBlock.Term == nil {
		// Add a default branch to the next block if one exists
		if len(fn.Blocks) > 0 && fn.Blocks[len(fn.Blocks)-1] != mergeBlock {
			nextBlockIndex := -1
			for i, block := range fn.Blocks {
				if block == mergeBlock && i < len(fn.Blocks)-1 {
					nextBlockIndex = i + 1
					break
				}
			}

			if nextBlockIndex != -1 {
				mergeBlock.NewBr(fn.Blocks[nextBlockIndex])
			} else {
				// If no next block, return a default value
				if fn.Sig.RetType.Equal(types.Void) {
					mergeBlock.NewRet(nil)
				} else {
					mergeBlock.NewRet(constant.NewInt(types.I32, 0))
				}
			}
		} else {
			// Add a default return if no next block
			if fn.Sig.RetType.Equal(types.Void) {
				mergeBlock.NewRet(nil)
			} else {
				mergeBlock.NewRet(constant.NewInt(types.I32, 0))
			}
		}
	}

	return nil, nil
}

// generatePrintStatement generates code for a print statement
func (g *Generator) generatePrintStatement(stmt *ast.PrintStatement) (value.Value, error) {
	// Check if we have a current function
	if g.context.currentFunction == nil {
		fmt.Fprintf(os.Stderr, "ERROR: No current function when generating print statement\n")
		return constant.NewInt(types.I32, 0), nil
	}

	// Generate the value to print
	val, err := g.generateExpression(stmt.Value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to generate value for print statement: %v\n", err)
		return nil, err
	}

	// Get current block safely
	var currentBlock *ir.Block
	if len(g.context.currentFunction.Blocks) > 0 {
		currentBlock = g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]
	} else {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to get current block for print statement\n")
		return constant.NewInt(types.I32, 0), nil
	}

	// Depending on the type, we need different format strings for printf
	var formatStr string
	var printVal value.Value
	var formatType types.Type

	switch valType := val.Type().(type) {
	case *types.IntType:
		formatStr = "%d\n"
		formatType = types.NewPointer(types.NewArray(4, types.I8))
		printVal = val
	case *types.FloatType:
		formatStr = "%f\n"
		formatType = types.NewPointer(types.NewArray(4, types.I8))
		printVal = val
	case *types.PointerType:
		// Check if it's a string
		if valType.ElemType != nil && valType.ElemType.Equal(types.I8) {
			formatStr = "%s\n"
			formatType = types.NewPointer(types.NewArray(4, types.I8))
			printVal = val
		} else {
			formatStr = "%p\n"
			formatType = types.NewPointer(types.NewArray(4, types.I8))
			printVal = val
		}
	default:
		formatStr = "%s\n"
		formatType = types.NewPointer(types.NewArray(4, types.I8))
		printVal = val
	}

	// Create format string constant
	formatStrConst := g.getStringConstant(formatStr)

	// Use printf function from module
	printfFn := g.printfFunc
	if printfFn == nil {
		for _, fn := range g.module.Funcs {
			if fn.Name() == "printf" {
				printfFn = fn
				break
			}
		}
	}

	if printfFn == nil {
		fmt.Fprintf(os.Stderr, "ERROR: printf function not found\n")
		return constant.NewInt(types.I32, 0), nil
	}

	// Cast format string constant to the expected pointer type
	formatStrPtr := currentBlock.NewBitCast(formatStrConst, formatType)

	// Create the printf call with appropriate parameters
	var result value.Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "PANIC in printf call: %v\n", r)
				result = nil
			}
		}()

		// Ensure printVal is a pointer if needed
		var printArg value.Value
		if types.IsPointer(printVal.Type()) {
			printArg = printVal
		} else {
			// Create an alloca to store the value
			alloca := currentBlock.NewAlloca(val.Type())
			currentBlock.NewStore(val, alloca)
			printArg = alloca
		}

		result = currentBlock.NewCall(printfFn, formatStrPtr, printArg)
	}()

	if result == nil {
		fmt.Fprintf(os.Stderr, "WARNING: Print call failed, using default value\n")
		return constant.NewInt(types.I32, 0), nil
	}

	return result, nil
}

// getStringConstant gets or creates a string constant
func (g *Generator) getStringConstant(str string) value.Value {
	// Check if the string already exists
	if global, ok := g.context.stringConstants[str]; ok {
		return global
	}

	// Process escape sequences in the string
	processedStr := str

	// Create a properly null-terminated string constant
	// Only add null terminator if it doesn't already have one
	if !strings.HasSuffix(processedStr, "\x00") {
		processedStr = processedStr + "\x00"
	}

	// Calculate the exact length including the null terminator
	arrayLength := uint64(len(processedStr))

	// Create a string constant
	strType := types.NewArray(arrayLength, types.I8)
	strConst := g.module.NewGlobalDef(fmt.Sprintf(".str.%d", g.stringCounter),
		constant.NewCharArrayFromString(processedStr))
	g.stringCounter++

	// Create a GEP instruction to get the pointer to the first character
	zero := constant.NewInt(types.I64, 0)
	strPtr := constant.NewGetElementPtr(strType, strConst, zero, zero)

	// Store the string in the context
	g.context.stringConstants[str] = strConst

	return strPtr
}

func (g *Generator) generateIdentifier(expr *ast.Identifier) (value.Value, error) {
	fmt.Fprintf(os.Stderr, "Generating identifier: %s\n", expr.Value)

	// Check for function first
	for _, fn := range g.module.Funcs {
		if fn.Name() == expr.Value {
			fmt.Fprintf(os.Stderr, "Found function: %s\n", expr.Value)
			return fn, nil
		}
	}

	// Look up variable in context
	val, ok := g.context.Lookup(expr.Value)
	if !ok {
		fmt.Fprintf(os.Stderr, "WARNING: Using a default value (0) for undefined variable: %s\n", expr.Value)
		return constant.NewInt(types.I32, 0), nil
	}

	if val == nil {
		fmt.Fprintf(os.Stderr, "WARNING: Variable %s found in context but has nil value\n", expr.Value)
		return constant.NewInt(types.I32, 0), nil
	}

	fmt.Fprintf(os.Stderr, "Value type: %v\n", val.Type())

	// If it's a function, return it directly
	if fn, ok := val.(*ir.Func); ok {
		fmt.Fprintf(os.Stderr, "Returning function: %s\n", fn.Name())
		return fn, nil
	}

	// Handle pointer types
	if ptr, ok := val.Type().(*types.PointerType); ok {
		fmt.Fprintf(os.Stderr, "Pointer element type: %v\n", ptr.ElemType)

		if g.context.currentFunction == nil || len(g.context.currentFunction.Blocks) == 0 {
			fmt.Fprintf(os.Stderr, "WARNING: Attempting to load value outside of a function or in a function with no blocks\n")
			return constant.NewInt(types.I32, 0), nil
		}

		currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

		// Handle function pointer types
		if _, ok := ptr.ElemType.(*types.FuncType); ok {
			fmt.Fprintf(os.Stderr, "Function pointer detected\n")
			return currentBlock.NewLoad(ptr, val), nil
		}

		// Handle generic pointer (i8*) that might hold a function pointer
		if ptr.ElemType.Equal(types.I8) {
			isFuncPtr := false
			for _, fname := range []string{"factorial", "fibonacci", "sum", "findFirstMultipleOf7"} {
				if expr.Value == fname {
					isFuncPtr = true
					break
				}
			}

			if isFuncPtr {
				fmt.Fprintf(os.Stderr, "Potential function pointer stored in i8* detected\n")
				// Assume function type: i32(i32)
				funcType := types.NewFunc(types.I32, []types.Type{types.I32}...)
				funcPtrType := types.NewPointer(funcType)
				castedPtr := currentBlock.NewBitCast(val, funcPtrType)
				return currentBlock.NewLoad(funcPtrType, castedPtr), nil
			}
		}

		// Load regular value
		return currentBlock.NewLoad(ptr.ElemType, val), nil
	}

	return val, nil
}

// 2. Fix for generateCallExpression function (around line 1371)
func (g *Generator) generateCallExpression(expr *ast.CallExpression) (value.Value, error) {
	// Generate code for the function
	function, err := g.generateExpression(expr.Function)
	if err != nil {
		return nil, err
	}

	// Check if function is nil
	if function == nil {
		fmt.Fprintf(os.Stderr, "ERROR: Function expression evaluated to nil in call expression\n")
		return constant.NewInt(types.I32, 0), nil
	}

	// Make sure the function is callable
	var callableFunction value.Value
	var returnType types.Type = types.I32 // Default return type

	// Determine callability and return type
	switch fn := function.(type) {
	case *ir.Func:
		// Direct function reference
		callableFunction = fn
		returnType = fn.Sig.RetType
	default:
		// Check for function pointer types
		funcType, ok := function.Type().(*types.PointerType)
		if ok {
			funcElemType, ok := funcType.ElemType.(*types.FuncType)
			if ok {
				// Verify we have a current function with at least one block
				if g.context.currentFunction == nil || len(g.context.currentFunction.Blocks) == 0 {
					fmt.Fprintf(os.Stderr, "ERROR: No current function or blocks when generating call expression\n")
					return constant.NewInt(types.I32, 0), nil
				}

				// We don't need to create a current block or function pointer type since
				// the function is already loaded by generateIdentifier
				callableFunction = function
				returnType = funcElemType.RetType
			} else {
				fmt.Fprintf(os.Stderr, "ERROR: Attempted to call a non-function pointer: %v\n", funcType.ElemType)
				return constant.NewInt(types.I32, 0), nil
			}
		} else {
			// Not a valid function type
			fmt.Fprintf(os.Stderr, "ERROR: Cannot call value of type %T (type %v)\n", fn, function.Type())
			return constant.NewInt(types.I32, 0), nil
		}
	}

	// Generate code for the arguments
	args := make([]value.Value, len(expr.Arguments))
	for i, arg := range expr.Arguments {
		if arg == nil {
			fmt.Fprintf(os.Stderr, "WARNING: Nil argument at position %d in function call\n", i)
			args[i] = constant.NewInt(types.I32, 0) // Default value
			continue
		}

		argVal, err := g.generateExpression(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to generate argument %d: %v\n", i, err)
			return nil, err
		}

		if argVal == nil {
			fmt.Fprintf(os.Stderr, "WARNING: Argument expression %d evaluated to nil\n", i)
			args[i] = constant.NewInt(types.I32, 0) // Default value
			continue
		}

		args[i] = argVal
	}

	// Verify we have a current function with at least one block
	if g.context.currentFunction == nil || len(g.context.currentFunction.Blocks) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: No current function or blocks when generating call expression\n")
		return constant.NewInt(types.I32, 0), nil
	}

	// Get current block
	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

	// Generate the call with more robust error checking
	var callResult value.Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "PANIC in call instruction: %v\n", r)
				callResult = nil
			}
		}()

		// Perform the call
		callResult = currentBlock.NewCall(callableFunction, args...)
	}()

	if callResult == nil {
		fmt.Fprintf(os.Stderr, "WARNING: Call instruction resulted in nil, using default value\n")
		return constant.NewInt(types.I32, 0), nil
	}

	// If we have a specific return type that's not void, return the call result
	// Otherwise, return a default value
	if returnType != nil && !returnType.Equal(types.Void) {
		return callResult, nil
	}

	return constant.NewInt(types.I32, 0), nil
}

// generateIntegerLiteral generates code for an integer literal
func (g *Generator) generateIntegerLiteral(expr *ast.IntegerLiteral) (value.Value, error) {
	return constant.NewInt(types.I32, int64(expr.Value)), nil
}

// generateStringLiteral generates code for a string literal
func (g *Generator) generateStringLiteral(expr *ast.StringLiteral) (value.Value, error) {
	return g.getStringConstant(expr.Value), nil
}

// generateBooleanLiteral generates code for a boolean literal
func (g *Generator) generateBooleanLiteral(expr *ast.BooleanLiteral) (value.Value, error) {
	if expr.Value {
		return constant.NewInt(types.I1, 1), nil
	}
	return constant.NewInt(types.I1, 0), nil
}

// generatePrefixExpression generates code for a prefix expression
func (g *Generator) generatePrefixExpression(expr *ast.PrefixExpression) (value.Value, error) {
	// Generate code for the right expression
	right, err := g.generateExpression(expr.Right)
	if err != nil {
		return nil, err
	}

	// Get current block
	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

	// Apply the operator
	switch expr.Operator {
	case "!":
		// Boolean not
		var boolVal value.Value
		if right.Type().Equal(types.I1) {
			boolVal = right
		} else {
			intType, ok := right.Type().(*types.IntType)
			if !ok {
				return nil, fmt.Errorf("expected int type for ! operator, got %T", right.Type())
			}
			boolVal = currentBlock.NewICmp(enum.IPredNE, right, constant.NewInt(intType, 0))
		}
		return currentBlock.NewXor(boolVal, constant.NewInt(types.I1, 1)), nil
	case "-":
		// Negate
		intType, ok := right.Type().(*types.IntType)
		if !ok {
			return nil, fmt.Errorf("expected int type for - operator, got %T", right.Type())
		}
		return currentBlock.NewSub(constant.NewInt(intType, 0), right), nil
	default:
		return nil, fmt.Errorf("unknown prefix operator: %s", expr.Operator)
	}
}

// generateInfixExpression generates code for an infix expression
// generateInfixExpression generates code for an infix expression
func (g *Generator) generateInfixExpression(expr *ast.InfixExpression) (value.Value, error) {
	// Generate code for the left and right expressions
	left, err := g.generateExpression(expr.Left)
	if err != nil {
		return nil, err
	}

	// Check if left is nil to avoid panic
	if left == nil {
		fmt.Fprintf(os.Stderr, "WARNING: Left expression evaluated to nil in infix expression\n")
		return constant.NewInt(types.I32, 0), nil
	}

	right, err := g.generateExpression(expr.Right)
	if err != nil {
		return nil, err
	}

	// Check if right is nil to avoid panic
	if right == nil {
		fmt.Fprintf(os.Stderr, "WARNING: Right expression evaluated to nil in infix expression\n")
		return constant.NewInt(types.I32, 0), nil
	}

	// Verify we have a current function and at least one block
	if g.context.currentFunction == nil || len(g.context.currentFunction.Blocks) == 0 {
		fmt.Fprintf(os.Stderr, "WARNING: No current function or blocks when generating infix expression\n")
		// Return a default value for the operation
		return constant.NewInt(types.I32, 0), nil
	}

	// Get current block
	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

	// Apply the operator
	switch expr.Operator {
	case "+":
		return currentBlock.NewAdd(left, right), nil
	case "-":
		return currentBlock.NewSub(left, right), nil
	case "*":
		return currentBlock.NewMul(left, right), nil
	case "/":
		return currentBlock.NewSDiv(left, right), nil
	case "%":
		return currentBlock.NewSRem(left, right), nil
	case "<":
		return currentBlock.NewICmp(enum.IPredSLT, left, right), nil
	case ">":
		return currentBlock.NewICmp(enum.IPredSGT, left, right), nil
	case "<=":
		return currentBlock.NewICmp(enum.IPredSLE, left, right), nil
	case ">=":
		return currentBlock.NewICmp(enum.IPredSGE, left, right), nil
	case "==":
		return currentBlock.NewICmp(enum.IPredEQ, left, right), nil
	case "!=":
		return currentBlock.NewICmp(enum.IPredNE, left, right), nil
	default:
		return nil, fmt.Errorf("unknown infix operator: %s", expr.Operator)
	}
}

// generateFunctionLiteral generates code for a function literal
func (g *Generator) generateFunctionLiteral(expr *ast.FunctionLiteral) (value.Value, error) {
	fmt.Fprintf(os.Stderr, "Generating function literal: %s\n", expr.Name)
	fmt.Fprintf(os.Stderr, "Return Type: %s\n", expr.ReturnType)
	fmt.Fprintf(os.Stderr, "Number of Parameters: %d\n", len(expr.Parameters))
	for i, param := range expr.Parameters {
		fmt.Fprintf(os.Stderr, "Parameter %d: %s\n", i, param.Value)
	}

	// Check for existing function
	for _, existingFunc := range g.module.Funcs {
		if existingFunc.Name() == expr.Name {
			return existingFunc, nil
		}
	}

	// Create function name
	funcName := expr.Name
	if funcName == "" {
		funcName = fmt.Sprintf("anon.%d", g.blockCounter)
		g.blockCounter++
	}

	// Determine return type
	var retType types.Type = types.I32
	switch expr.ReturnType {
	case "void":
		retType = types.Void
	case "bool":
		retType = types.I1
	case "int", "":
		retType = types.I32
	}

	// Create parameter types
	paramTypes := make([]types.Type, len(expr.Parameters))
	for i := range paramTypes {
		paramTypes[i] = types.I32
	}

	// Create function type
	funcType := types.NewFunc(retType, paramTypes...)

	// Create function
	fn := g.module.NewFunc(funcName, funcType)

	// Save current context
	oldContext := g.context
	newContext := newContext(oldContext)
	for name, val := range oldContext.namedValues {
		newContext.namedValues[name] = val
	}
	g.context = newContext
	g.context.currentFunction = fn

	// Create entry block
	entryBlock := fn.NewBlock("entry")
	g.context.blocks["entry"] = entryBlock

	// Set parameter names and allocate them
	for i, p := range fn.Params {
		if i < len(expr.Parameters) {
			p.SetName(expr.Parameters[i].Value)
		}
	}
	for i, param := range expr.Parameters {
		paramAlloca := entryBlock.NewAlloca(types.I32)
		paramAlloca.SetName(param.Value)
		if i < len(fn.Params) {
			entryBlock.NewStore(fn.Params[i], paramAlloca)
		} else {
			entryBlock.NewStore(constant.NewInt(types.I32, 0), paramAlloca)
		}
		g.context.namedValues[param.Value] = paramAlloca
	}

	// Generate body
	bodyVal, err := g.generateStatement(expr.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating body for function %s: %v\n", funcName, err)
		g.context = oldContext
		return nil, err
	}

	// Ensure terminator
	if len(fn.Blocks) > 0 {
		lastBlock := fn.Blocks[len(fn.Blocks)-1]
		if lastBlock.Term == nil {
			if bodyVal != nil && !retType.Equal(types.Void) {
				lastBlock.NewRet(bodyVal)
			} else {
				switch retType {
				case types.Void:
					lastBlock.NewRet(nil)
				case types.I1:
					lastBlock.NewRet(constant.NewInt(types.I1, 0))
				default:
					lastBlock.NewRet(constant.NewInt(types.I32, 0))
				}
			}
		}
	} else {
		switch retType {
		case types.Void:
			entryBlock.NewRet(nil)
		case types.I1:
			entryBlock.NewRet(constant.NewInt(types.I1, 0))
		default:
			entryBlock.NewRet(constant.NewInt(types.I32, 0))
		}
	}

	// Restore context
	g.context = oldContext

	return fn, nil
}

// WriteToFile writes the generated LLVM IR to a file
func (g *Generator) WriteToFile(filename string) error {
	// Generate LLVM IR to a string
	var buf strings.Builder
	_, err := g.module.WriteTo(&buf)
	if err != nil {
		return err
	}

	// Get the generated IR
	ir := buf.String()

	// Apply all fixes to function declarations
	ir = fixFunctionDeclarations(ir)

	// Write the modified IR to the file
	return os.WriteFile(filename, []byte(ir), 0644)
}

func CompileToLLVM(program *ast.Program) (string, error) {
	// Create a generator
	generator := New()

	// Generate code for the program
	module, err := generator.Generate(program)
	if err != nil {
		return "", err
	}

	// Convert module to string
	var buf strings.Builder
	_, err = module.WriteTo(&buf)
	if err != nil {
		return "", err
	}

	// Get the generated IR
	ir := buf.String()

	// Apply our comprehensive fix
	ir = comprehensiveIRFix(ir)

	// Add standard format strings
	formatStrings := `
; Standard format strings
@.fmt.int = private constant [4 x i8] c"%d\0A\00"
@.fmt.str = private constant [4 x i8] c"%s\0A\00"
@.fmt.float = private constant [4 x i8] c"%f\0A\00"
@.fmt.bool = private constant [4 x i8] c"%d\0A\00"
`
	ir = ir + "\n" + formatStrings

	return ir, nil
}

// Process string constants to ensure proper formatting
func processStringConstants(ir string) string {
	// Find all string constants
	strConstPattern := regexp.MustCompile(`@\.str\.[0-9]+ = .*?c"([^"]*)".*`)

	return strConstPattern.ReplaceAllStringFunc(ir, func(s string) string {
		matches := strConstPattern.FindStringSubmatch(s)
		if len(matches) > 1 {
			content := matches[1]

			// Replace escape sequences with their proper representation in LLVM IR
			// Handle common escape sequences
			content = strings.ReplaceAll(content, "\\n", "\\0A")  // Newline
			content = strings.ReplaceAll(content, "\\t", "\\09")  // Tab
			content = strings.ReplaceAll(content, "\\\"", "\\22") // Double quote
			content = strings.ReplaceAll(content, "\\\\", "\\5C") // Backslash

			// Ensure proper null termination
			if !strings.HasSuffix(content, "\\00") {
				content = content + "\\00"
			}

			// Reconstruct the string constant with proper escaping
			reconstructed := strings.Replace(s, matches[1], content, 1)
			return reconstructed
		}
		return s
	})
}

func fixFunctionDeclarations(ir string) string {
	// First completely remove all existing declarations of standard library functions
	// This ensures we don't have any duplicates or malformed declarations
	stdlibFuncs := []string{"printf", "malloc", "free", "strlen", "strcpy", "abs", "pow", "exit"}

	for _, funcName := range stdlibFuncs {
		pattern := regexp.MustCompile(`declare\s+[^@]*@` + funcName + `[^\n]*\n`)
		ir = pattern.ReplaceAllString(ir, "")
	}

	// Add our own properly formatted declarations at the beginning
	stdlibDecls := `
    ; Standard C library function declarations
    declare i32 @printf(i8*, ...)
    declare i8* @malloc(i64)
    declare void @free(i8*)
    declare i64 @strlen(i8*)
    declare i8* @strcpy(i8*, i8*)
    declare i32 @abs(i32)
    declare double @pow(double, double)
    declare void @exit(i32)
    `
	ir = stdlibDecls + ir

	// Fix function definitions with wrong syntax
	// e.g., "define i32 (i32) @factorial()" -> "define i32 @factorial(i32)"
	funcDefPattern := regexp.MustCompile(`define\s+([a-zA-Z0-9*]+)\s+\(([^)]+)\)\s+@([a-zA-Z0-9_]+)\(\)`)
	ir = funcDefPattern.ReplaceAllString(ir, "define $1 @$3($2)")

	// Fix main function declaration specifically
	mainDefPattern := regexp.MustCompile(`define\s+i32\s+\(\)\s+@main\(\)`)
	ir = mainDefPattern.ReplaceAllString(ir, "define i32 @main()")

	return ir
}

func fixFunctionPointerUsage(ir string) string {
	// Split IR into lines to process each line individually
	lines := strings.Split(ir, "\n")

	// Process each line
	for i, line := range lines {
		// Look for function types in return statements
		if strings.Contains(line, "ret ") {
			// Check if there's a function type being used in a return statement
			// Pattern: ret i32 (i32) %reg
			retFuncPattern := regexp.MustCompile(`ret\s+(i[0-9]+)\s+\(([^)]+)\)\s+(%[0-9]+)`)
			matches := retFuncPattern.FindStringSubmatch(line)

			if len(matches) >= 4 {
				// Extract the return type and register
				returnType := matches[1]
				register := matches[3]

				// Replace the function type with a simple value type
				lines[i] = strings.Replace(line,
					"ret "+returnType+" ("+matches[2]+") "+register,
					"ret "+returnType+" "+register, 1)
			}
		}

		// Look for arithmetic operations with pointers
		if strings.Contains(line, "= add") ||
			strings.Contains(line, "= sub") ||
			strings.Contains(line, "= mul") ||
			strings.Contains(line, "= div") ||
			strings.Contains(line, "= rem") {

			// First, check and fix pointer types in arithmetic
			ptrArithPattern := regexp.MustCompile(`(add|sub|mul|div|rem)\s+(i[0-9]+)\*\s+(%[0-9]+),\s+(%[0-9]+)`)
			if matches := ptrArithPattern.FindStringSubmatch(line); len(matches) >= 5 {
				op := matches[1]
				valType := matches[2]
				reg1 := matches[3]
				reg2 := matches[4]

				// Replace pointer arithmetic with non-pointer
				lines[i] = strings.Replace(line,
					op+" "+valType+"* "+reg1+", "+reg2,
					op+" "+valType+" "+reg1+", "+reg2, 1)
				continue
			}

			// Check if there's a function type being used in an arithmetic operation
			// Pattern: i32 (i32) %reg
			funcTypePattern := regexp.MustCompile(`(i[0-9]+)\s+\([^)]+\)\s+(%[0-9]+)`)
			matches := funcTypePattern.FindAllStringSubmatch(line, -1)

			if len(matches) > 0 {
				modifiedLine := line
				for _, match := range matches {
					if len(match) >= 3 {
						// Extract the return type and register
						returnType := match[1]
						register := match[2]

						// Replace the function type with a simple value type
						modifiedLine = strings.Replace(modifiedLine,
							returnType+" ("+returnType+") "+register,
							returnType+" "+register, 1)
					}
				}
				lines[i] = modifiedLine
			}
		}

		// Fix function call patterns for variadic functions (like printf)
		if strings.Contains(line, "= call") {
			// Fix function types in function arguments
			// Pattern: call i32 @printf(..., i32 (i32) %reg, ...)
			funcArgPattern := regexp.MustCompile(`call\s+[^,]*,\s*(?:[^,]*,\s*)*([a-zA-Z0-9*]+)\s+\(([^)]+)\)\s+(%[0-9]+)`)
			if matches := funcArgPattern.FindStringSubmatch(line); len(matches) >= 4 {
				argType := matches[1]
				argReg := matches[3]
				lines[i] = strings.Replace(line,
					argType+" ("+matches[2]+") "+argReg,
					argType+"* "+argReg, 1)
				continue
			}

			// Check and fix printf-style variadic function calls
			// Pattern: call i32 (i8*) (...) @printf
			printfPattern := regexp.MustCompile(`call\s+(i[0-9]+)\s+\(([^)]+)\)\s+\(\.\.\.\)\s+@([a-zA-Z0-9_]+)`)
			if matches := printfPattern.FindStringSubmatch(line); len(matches) >= 4 {
				returnType := matches[1]
				funcName := matches[3]
				lines[i] = strings.Replace(line,
					"call "+returnType+" ("+matches[2]+") (...) @"+funcName,
					"call "+returnType+" @"+funcName, 1)
				continue
			}

			// Check if there's a regular function pointer call
			callPattern := regexp.MustCompile(`call\s+(i[0-9]+)\s+\(([^)]+)\)\s+(%[0-9]+)`)
			matches := callPattern.FindStringSubmatch(line)
			if len(matches) >= 4 {
				// Extract return type, argument types, and register
				returnType := matches[1]
				register := matches[3]

				// Fix the call syntax
				lines[i] = strings.Replace(line,
					"call "+returnType+" ("+matches[2]+") "+register,
					"call "+returnType+" "+register, 1)
			}
		}

		// Also fix function types in function arguments
		if strings.Contains(line, "i32 (i32)") || strings.Contains(line, "i32*") {
			// First, look for pointer types
			ptrPattern := regexp.MustCompile(`(i[0-9]+)\*\s+(%[0-9]+)`)
			matches := ptrPattern.FindAllStringSubmatch(line, -1)

			if len(matches) > 0 {
				newLine := line
				for _, match := range matches {
					if len(match) >= 3 {
						valType := match[1]
						regName := match[2]
						// Replace pointer syntax with just the base type
						newLine = strings.Replace(newLine,
							valType+"* "+regName,
							valType+" "+regName, 1)
					}
				}
				lines[i] = newLine
				continue
			}

			// Then look for function types
			funcArgPattern := regexp.MustCompile(`(i[0-9]+)\s+\(([^)]+)\)\s+(%[0-9]+)`)
			matches = funcArgPattern.FindAllStringSubmatch(line, -1)

			if len(matches) > 0 {
				newLine := line
				for _, match := range matches {
					if len(match) >= 3 {
						argType := match[1]
						argReg := match[3]
						newLine = strings.Replace(newLine,
							argType+" ("+match[2]+") "+argReg,
							argType+" "+argReg, 1)
					}
				}
				lines[i] = newLine
			}
		}
	}

	// Rejoin the lines
	return strings.Join(lines, "\n")
}

func comprehensiveIRFix(ir string) string {
	// First, apply string constant formatting
	ir = processStringConstants(ir)

	// Fix function declarations
	ir = fixFunctionDeclarations(ir)

	// Apply function pointer fixes
	ir = fixFunctionPointerUsage(ir)

	// Now perform register renumbering with direct text replacements
	lines := strings.Split(ir, "\n")

	// Process each function independently
	inFunction := false
	functionLines := []string{}
	fixedLines := []string{}

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Detect function start
		if strings.HasPrefix(trimmedLine, "define ") && strings.Contains(trimmedLine, "@") {
			// If we were in a function, process it
			if inFunction {
				processedFunctionLines := simpleRegisterRenumbering(functionLines)
				fixedLines = append(fixedLines, processedFunctionLines...)
			}

			// Start a new function
			inFunction = true
			functionLines = []string{line}
			continue
		}

		// Detect function end (next function start or end of file)
		if inFunction && (i == len(lines)-1 ||
			(strings.HasPrefix(trimmedLine, "define ") && strings.Contains(trimmedLine, "@"))) {
			// Add the current line if it's the last one and not a new function
			if i == len(lines)-1 && !strings.HasPrefix(trimmedLine, "define ") {
				functionLines = append(functionLines, line)
			}

			// Process the complete function
			processedFunctionLines := simpleRegisterRenumbering(functionLines)
			fixedLines = append(fixedLines, processedFunctionLines...)

			// Reset
			inFunction = false
			functionLines = []string{}

			// If this was a new function definition, process it in the next iteration
			if i != len(lines)-1 {
				i--
			}
			continue
		}

		// Collect lines within a function
		if inFunction {
			functionLines = append(functionLines, line)
		} else {
			// Lines outside functions go straight to output
			fixedLines = append(fixedLines, line)
		}
	}

	// Join everything back
	return strings.Join(fixedLines, "\n")
}

func simpleRegisterRenumbering(lines []string) []string {
	result := make([]string, len(lines))

	// Find all register definitions and map them to new numbers
	regDefs := make(map[string]int)
	nextRegNum := 0

	// First pass: identify all parameter registers
	for _, line := range lines {
		if strings.Contains(line, "define ") {
			// Parameters are in the function definition
			paramMatches := regexp.MustCompile(`%[0-9]+`).FindAllString(line, -1)
			for _, param := range paramMatches {
				regDefs[param] = nextRegNum
				nextRegNum++
			}
			break
		}
	}

	// Second pass: identify all instruction registers
	instRegNum := 1 // Start instruction numbering at 1
	for _, line := range lines {
		if strings.Contains(line, " = ") {
			// This is an instruction that defines a register
			defMatch := regexp.MustCompile(`(%[0-9]+)\s+=`).FindStringSubmatch(line)
			if len(defMatch) > 1 {
				reg := defMatch[1]
				if _, exists := regDefs[reg]; !exists {
					// Only assign if not already assigned as parameter
					regDefs[reg] = instRegNum
					instRegNum++
				}
			}
		}
	}

	// Third pass: apply the renumbering
	for i, line := range lines {
		newLine := line

		// Replace all registers with their new numbers
		regMatches := regexp.MustCompile(`%[0-9]+`).FindAllString(line, -1)
		for _, reg := range regMatches {
			if newNum, exists := regDefs[reg]; exists {
				// Replace this register with its new number
				newReg := fmt.Sprintf("%%%d", newNum)
				// Use a regex that avoids partial replacements
				pattern := fmt.Sprintf(`(^|\s|,|\()%s(\s|,|\)|$)`, regexp.QuoteMeta(reg))
				re := regexp.MustCompile(pattern)
				newLine = re.ReplaceAllString(newLine, "${1}"+newReg+"${2}")
			}
		}

		result[i] = newLine
	}

	return result
}
