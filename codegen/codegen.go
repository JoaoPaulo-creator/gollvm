package codegen

import (
	"compiler/ast"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
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
	// Create the function type for printf (returns i32, takes pointer to i8)
	printfType := types.NewFunc(types.I32, types.NewPointer(types.I8))

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
	// Process global definitions (functions)
	for i, stmt := range program.Statements {
		if varStmt, ok := stmt.(*ast.VarStatement); ok {
			if _, ok := varStmt.Value.(*ast.FunctionLiteral); ok {
				// Process function definition
				_, err := g.generateStatement(stmt)
				if err != nil {
					return nil, fmt.Errorf("error processing function declaration %d: %w", i, err)
				}
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
		if varStmt, ok := stmt.(*ast.VarStatement); ok {
			if _, ok := varStmt.Value.(*ast.FunctionLiteral); ok {
				continue
			}
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

// Add new statement handlers for advanced language features
func (g *Generator) generateFunctionStatement(stmt *ast.FunctionStatement) (value.Value, error) {
	// Generate a function from a function statement
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

// Add support for user-defined types
func (g *Generator) generateStructDefinition(expr *ast.StructDefinition) (value.Value, error) {
	// Create a new struct type
	fields := make([]types.Type, len(expr.Fields))

	for i, field := range expr.Fields {
		// Map field type to LLVM type
		switch field.Type {
		case "int":
			fields[i] = types.I32
		case "string":
			fields[i] = types.NewPointer(types.I8)
		case "bool":
			fields[i] = types.I1
		default:
			// For custom types, we'd need to look them up in a type registry
			fields[i] = types.I32 // Default to int for now
		}
	}

	// Create the struct type
	structType := types.NewStruct(fields...)

	// Register the type with a name
	g.module.NewTypeDef(expr.Name.Value, structType)

	return nil, nil
}

// Add support for array literals
func (g *Generator) generateArrayLiteral(expr *ast.ArrayLiteral) (value.Value, error) {
	if len(expr.Elements) == 0 {
		return nil, fmt.Errorf("empty array literals not supported yet")
	}

	// Generate code for the first element to determine array type
	firstElem, err := g.generateExpression(expr.Elements[0])
	if err != nil {
		return nil, err
	}

	// Create an array type
	arrayType := types.NewArray(uint64(len(expr.Elements)), firstElem.Type())

	// Generate all elements
	elements := make([]constant.Constant, len(expr.Elements))

	for i, elem := range expr.Elements {
		elemVal, err := g.generateExpression(elem)
		if err != nil {
			return nil, err
		}

		// Convert to constant if needed
		elemConst, ok := elemVal.(constant.Constant)
		if !ok {
			return nil, fmt.Errorf("array elements must be constants")
		}

		elements[i] = elemConst
	}

	// Create array constant
	arrayConst := constant.NewArray(arrayType, elements...)

	// Get current function and block
	fn := g.context.currentFunction
	currentBlock := fn.Blocks[len(fn.Blocks)-1]

	// Allocate space for the array
	arrayAlloca := currentBlock.NewAlloca(arrayType)

	// Store the array constant
	currentBlock.NewStore(arrayConst, arrayAlloca)

	return arrayAlloca, nil
}

func (g *Generator) debugExpression(expr ast.Expression) {
	if expr == nil {
		fmt.Fprintf(os.Stderr, "NIL EXPRESSION DETECTED\n")
		// Print context information instead of stack trace
		if g.context != nil && g.context.currentFunction != nil {
			fmt.Fprintf(os.Stderr, "Current function: %s\n", g.context.currentFunction.Name())
		}
		return
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
	if g.context.currentFunction == nil {
		// Global variable handling

		// Generate the value
		val, err := g.generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}

		// Handle function declarations specially
		if funcVal, ok := val.(*ir.Func); ok {
			// Simply use the function directly
			g.context.namedValues[stmt.Name.Value] = funcVal

			// If needed, rename the function
			if funcVal.Name() != stmt.Name.Value && len(funcVal.Blocks) == 0 {
				// Only rename empty functions (declarations)
				funcVal.SetName(stmt.Name.Value)
			}

			return funcVal, nil
		} else {
			// Create global variable with proper initialization
			global := g.module.NewGlobal(stmt.Name.Value, val.Type())

			// Try to use constant initialization if possible
			if constVal, ok := val.(constant.Constant); ok {
				global.Init = constVal
			} else {
				global.Init = constant.NewZeroInitializer(val.Type())

				// Create a global initializer function if needed for non-constant initializers
				var initFunc *ir.Func

				// Look for existing global initializer
				for _, fn := range g.module.Funcs {
					if fn.Name() == "global_init" {
						initFunc = fn
						break
					}
				}

				// Create global_init if it doesn't exist
				if initFunc == nil {
					initFunc = g.module.NewFunc("global_init", types.NewFunc(types.Void))
					initBlock := initFunc.NewBlock("entry")
					initBlock.NewRet(nil)

					// Add a call to global_init from main
					for _, fn := range g.module.Funcs {
						if fn.Name() == "main" {
							mainEntry := fn.Blocks[0]
							mainEntry.Insts = append([]ir.Instruction{mainEntry.NewCall(initFunc)},
								mainEntry.Insts...)
							break
						}
					}
				}

				// Get the entry block
				entryBlock := initFunc.Blocks[0]

				// Add store instruction to initialize the global
				storeInst := entryBlock.NewStore(val, global)

				// Insert before return instruction
				if entryBlock.Term != nil {
					insertPos := len(entryBlock.Insts) - 1
					entryBlock.Insts = append(entryBlock.Insts[:insertPos],
						append([]ir.Instruction{storeInst},
							entryBlock.Insts[insertPos:]...)...)
				} else {
					entryBlock.Insts = append(entryBlock.Insts, storeInst)
				}
			}

			g.context.namedValues[stmt.Name.Value] = global
			return global, nil
		}
	}

	// For local variables, we need to allocate stack space
	// Generate the value
	val, err := g.generateExpression(stmt.Value)
	if err != nil {
		return nil, err
	}

	// Get the entry block
	entryBlock := g.context.currentFunction.Blocks[0]

	// Create an alloca instruction at the beginning of the entry block
	allocaInst := entryBlock.NewAlloca(val.Type())
	if len(entryBlock.Insts) > 1 {
		// Move the alloca to the beginning
		entryBlock.Insts = append([]ir.Instruction{allocaInst}, entryBlock.Insts[:len(entryBlock.Insts)-1]...)
	}

	// Get current block
	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

	// Store the value in the allocated space
	currentBlock.NewStore(val, allocaInst)

	// Add to named values
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

	switch val.Type().(type) {
	case *types.IntType:
		formatStr = "%d\n"
		printVal = val
	case *types.FloatType:
		formatStr = "%f\n"
		printVal = val
	case *types.PointerType:
		// Check if it's a string
		pointerType, ok := val.Type().(*types.PointerType)
		if !ok {
			return nil, fmt.Errorf("expected pointer type, got %T", val.Type())
		}
		if pointerType.ElemType != nil && pointerType.ElemType.Equal(types.I8) {
			formatStr = "%s\n"
			printVal = val
		} else {
			formatStr = "%p\n"
			printVal = val
		}
	default:
		formatStr = "%s\n"
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

	// Create the printf call with appropriate parameters
	var result value.Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "PANIC in printf call: %v\n", r)
				result = nil
			}
		}()

		result = currentBlock.NewCall(printfFn, formatStrConst, printVal)
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

// generateIdentifier generates code for an identifier
func (g *Generator) generateIdentifier(expr *ast.Identifier) (value.Value, error) {
	// Special handling for function identifiers - try to find the function in the module first
	for _, fn := range g.module.Funcs {
		if fn.Name() == expr.Value {
			return fn, nil
		}
	}

	// Then look up the variable in the context
	val, ok := g.context.Lookup(expr.Value)
	if !ok {
		// Return a default value instead of crashing
		fmt.Fprintf(os.Stderr, "WARNING: Using a default value (0) for undefined variable: %s\n", expr.Value)
		return constant.NewInt(types.I32, 0), nil
	}

	// Check if val is nil before proceeding
	if val == nil {
		fmt.Fprintf(os.Stderr, "WARNING: Variable %s found in context but has nil value\n", expr.Value)
		return constant.NewInt(types.I32, 0), nil
	}

	// If it's already a function, just return it
	if _, ok := val.(*ir.Func); ok {
		return val, nil
	}

	// If it's a pointer to a function, load it properly
	pointerType, ok := val.Type().(*types.PointerType)
	if ok {
		// Ensure we're in a function with blocks
		if g.context.currentFunction == nil || len(g.context.currentFunction.Blocks) == 0 {
			fmt.Fprintf(os.Stderr, "WARNING: Attempting to load value outside of a function or in a function with no blocks\n")
			return constant.NewInt(types.I32, 0), nil
		}

		// Get current block
		currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

		// Check if pointer type element type is valid
		if pointerType.ElemType == nil {
			fmt.Fprintf(os.Stderr, "WARNING: Pointer type has nil element type for variable: %s\n", expr.Value)
			return constant.NewInt(types.I32, 0), nil
		}

		// If it's a function pointer, load it
		if strings.HasPrefix(pointerType.ElemType.String(), "func") {
			// For function pointers, load the value
			return currentBlock.NewLoad(pointerType.ElemType, val), nil
		}

		// For other pointer types, load as usual
		if pointerType.ElemType.Equal(types.I32) ||
			pointerType.ElemType.Equal(types.I64) ||
			pointerType.ElemType.Equal(types.I8) ||
			(pointerType.ElemType.String() == "*i8") {
			// Load the value
			return currentBlock.NewLoad(pointerType.ElemType, val), nil
		}
	}

	return val, nil
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
	// Save the current context
	oldContext := g.context

	// Create a new context for the function with the parent context
	newContext := newContext(oldContext)

	// Make sure to copy over any global values from the parent context
	for name, val := range oldContext.namedValues {
		newContext.namedValues[name] = val
	}

	g.context = newContext

	// Get the function name
	funcName := expr.Name
	if funcName == "" {
		funcName = fmt.Sprintf("anon.%d", g.blockCounter)
		g.blockCounter++
	}

	// Create parameter types (all i32 for now)
	paramTypes := make([]types.Type, len(expr.Parameters))
	for i := range paramTypes {
		paramTypes[i] = types.I32
	}

	// Create function type (returns i32 for now)
	funcType := types.NewFunc(types.I32, paramTypes...)

	// Create function
	fn := g.module.NewFunc(funcName, funcType)

	// Set current function before creating blocks
	g.context.currentFunction = fn

	// Create entry block immediately
	entryBlock := fn.NewBlock("entry")

	// Store the entry block in the context
	g.context.blocks["entry"] = entryBlock

	// Give names to the parameters for debugging
	for i, p := range fn.Params {
		if i < len(expr.Parameters) {
			p.SetName(expr.Parameters[i].Value)
		}
	}

	// Add parameters to named values
	for i, param := range expr.Parameters {
		// Create an alloca for the parameter in the entry block
		paramAlloca := entryBlock.NewAlloca(types.I32)
		paramAlloca.SetName(param.Value)

		// Store the parameter value safely
		if i < len(fn.Params) {
			entryBlock.NewStore(fn.Params[i], paramAlloca)
		} else {
			// Use a default value if the parameter is missing
			defaultValue := constant.NewInt(types.I32, 0)
			entryBlock.NewStore(defaultValue, paramAlloca)
		}

		g.context.namedValues[param.Value] = paramAlloca
	}

	// Generate code for the body
	bodyVal, err := g.generateStatement(expr.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating body for function %s: %v\n", funcName, err)

		// Still restore context before returning
		g.context = oldContext
		return nil, err
	}

	// Ensure the last block has a terminator
	if len(fn.Blocks) > 0 {
		lastBlock := fn.Blocks[len(fn.Blocks)-1]
		if lastBlock.Term == nil {
			// If we have a return value from the body, use it
			if bodyVal != nil {
				lastBlock.NewRet(bodyVal)
			} else {
				lastBlock.NewRet(constant.NewInt(types.I32, 0))
			}
		}
	} else {
		// Add a terminator to the entry block if no blocks were created during body generation
		entryBlock.NewRet(constant.NewInt(types.I32, 0))
	}

	// Restore context
	g.context = oldContext

	return fn, nil
}

// generateCallExpression generates code for a function call
// generateCallExpression generates code for a function call
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
	// Check if we have a function pointer (common case for function calls)
	if _, ok := function.Type().(*types.IntType); ok {
		fmt.Fprintf(os.Stderr, "WARNING: Attempting to call an integer as a function. This likely means the function lookup failed.\n")
		fmt.Fprintf(os.Stderr, "Function name being called: %s\n", expr.Function.(*ast.Identifier).Value)
		return constant.NewInt(types.I32, 0), nil
	}

	// Make sure the function is callable (either a Function or a pointer to a function)
	var callableFunction value.Value
	if funcPtr, ok := function.Type().(*types.PointerType); ok {
		if _, ok := funcPtr.ElemType.(*types.FuncType); ok {
			// Already a function pointer, use as is
			callableFunction = function
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: Attempted to call a non-function pointer: %v\n", funcPtr.ElemType)
			return constant.NewInt(types.I32, 0), nil
		}
	} else if _, ok := function.(*ir.Func); ok {
		// Direct function reference
		callableFunction = function
	} else {
		// Not a valid function type
		fmt.Fprintf(os.Stderr, "ERROR: Cannot call value of type %T\n", function.Type())
		return constant.NewInt(types.I32, 0), nil
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
	// var callErr error

	// Use a recovery function to prevent panics
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "PANIC in call instruction: %v\n", r)
				callResult = constant.NewInt(types.I32, 0)
			}
		}()

		callResult = currentBlock.NewCall(callableFunction, args...)
	}()

	if callResult == nil {
		fmt.Fprintf(os.Stderr, "WARNING: Call instruction resulted in nil, using default value\n")
		return constant.NewInt(types.I32, 0), nil
	}

	return callResult, nil

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

func getStandardFunctionDeclarations(shouldIncludePrintf bool) string {
	var decls strings.Builder

	decls.WriteString("\n; Standard C library function declarations\n")

	// Only include printf if requested
	if shouldIncludePrintf {
		decls.WriteString("declare i32 @printf(i8*, ...)\n")
	}

	decls.WriteString("declare i8* @malloc(i64)\n")
	decls.WriteString("declare void @free(i8*)\n")
	decls.WriteString("declare i64 @strlen(i8*)\n")
	decls.WriteString("declare i8* @strcpy(i8*, i8*)\n")
	decls.WriteString("declare i32 @abs(i32)\n")
	decls.WriteString("declare double @pow(double, double)\n")
	decls.WriteString("declare void @exit(i32)\n")

	decls.WriteString("\n; Standard format strings\n")
	decls.WriteString("@.fmt.int = private constant [4 x i8] c\"%d\\0A\\00\"\n")
	decls.WriteString("@.fmt.str = private constant [4 x i8] c\"%s\\0A\\00\"\n")
	decls.WriteString("@.fmt.float = private constant [4 x i8] c\"%f\\0A\\00\"\n")
	decls.WriteString("@.fmt.bool = private constant [4 x i8] c\"%d\\0A\\00\"\n")

	return decls.String()
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

	return ir
}

func fixInstructionNumbering(ir string) string {
	// Split IR into lines to process each line individually
	lines := strings.Split(ir, "\n")

	// Process each function separately
	currentFunction := ""
	currentBlock := ""
	blocksInFunction := make(map[string][]string)      // Maps function name to list of blocks
	registerMaps := make(map[string]map[string]string) // Maps function name -> old register -> new register
	nextRegisterNumber := make(map[string]int)         // Maps function name to next register number

	// First pass: Identify functions and blocks
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Detect function definition
		if strings.HasPrefix(trimmedLine, "define ") && strings.Contains(trimmedLine, "@") {
			funcNamePattern := regexp.MustCompile(`@([a-zA-Z0-9_.]+)`)
			matches := funcNamePattern.FindStringSubmatch(trimmedLine)
			if len(matches) >= 2 {
				currentFunction = matches[1]
				blocksInFunction[currentFunction] = []string{}
				registerMaps[currentFunction] = make(map[string]string)
				nextRegisterNumber[currentFunction] = 1 // Start with %1
			}
		}

		// Detect block labels
		if strings.HasSuffix(trimmedLine, ":") && currentFunction != "" {
			blockName := strings.TrimSuffix(trimmedLine, ":")
			currentBlock = blockName
			blocksInFunction[currentFunction] = append(blocksInFunction[currentFunction], currentBlock)
		}
	}

	// Second pass: Process and rename registers across all blocks in each function
	currentFunction = ""

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Update current function when we detect a function definition
		if strings.HasPrefix(trimmedLine, "define ") && strings.Contains(trimmedLine, "@") {
			funcNamePattern := regexp.MustCompile(`@([a-zA-Z0-9_.]+)`)
			matches := funcNamePattern.FindStringSubmatch(trimmedLine)
			if len(matches) >= 2 {
				currentFunction = matches[1]
			}
		}

		// Skip if not in a function
		if currentFunction == "" {
			continue
		}

		// Process register definitions
		if strings.Contains(trimmedLine, " = ") {
			regDefPattern := regexp.MustCompile(`\s*(%[0-9]+)\s*=`)
			matches := regDefPattern.FindStringSubmatch(trimmedLine)
			if len(matches) >= 2 {
				oldReg := matches[1]

				// Check if we already have a mapping for this register
				if _, exists := registerMaps[currentFunction][oldReg]; !exists {
					// Create a new register name
					newReg := fmt.Sprintf("%%%d", nextRegisterNumber[currentFunction])
					nextRegisterNumber[currentFunction]++

					// Store the mapping
					registerMaps[currentFunction][oldReg] = newReg
				}

				// Replace the register definition
				newReg := registerMaps[currentFunction][oldReg]
				lines[i] = strings.Replace(line, oldReg+" =", newReg+" =", 1)
			}
		}

		// Replace register usages (but not in definitions)
		for oldReg, newReg := range registerMaps[currentFunction] {
			if !strings.Contains(line, oldReg+" =") {
				// This ensures we only replace complete register names, not partial matches
				pattern := regexp.MustCompile(`(^|[^%0-9])` + regexp.QuoteMeta(oldReg) + `($|[^0-9])`)
				lines[i] = pattern.ReplaceAllString(lines[i], "${1}"+newReg+"${2}")
			}
		}
	}

	// Rejoin the lines
	return strings.Join(lines, "\n")
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

		// Look for arithmetic operations on function types
		if strings.Contains(line, "= add") ||
			strings.Contains(line, "= sub") ||
			strings.Contains(line, "= mul") ||
			strings.Contains(line, "= div") ||
			strings.Contains(line, "= rem") {

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
						// This is a simplification - in a real compiler, you'd need to handle this properly
						// by creating a proper function pointer variable
						modifiedLine = strings.Replace(modifiedLine,
							returnType+" ("+returnType+") "+register,
							returnType+" "+register, 1)
					}
				}
				lines[i] = modifiedLine
			}
		}

		// Fix function call patterns if needed
		if strings.Contains(line, "= call") {
			// Check if there's a function pointer call
			callPattern := regexp.MustCompile(`call\s+(i[0-9]+)\s+\(([^)]+)\)\s+(%[0-9]+)`)
			matches := callPattern.FindStringSubmatch(line)

			if len(matches) >= 4 {
				// Extract return type, argument types, and register
				returnType := matches[1]
				argTypes := matches[2]
				register := matches[3]

				// Fix the call syntax
				lines[i] = strings.Replace(line,
					"call "+returnType+" ("+argTypes+") "+register,
					"call "+returnType+" "+register, 1)
			}
		}
	}

	// Rejoin the lines
	return strings.Join(lines, "\n")
}

func fixCommonIRIssues(ir string) string {
	// Fix function pointers in arithmetic operations, returns, and calls
	ir = fixFunctionPointerUsage(ir)

	// Fix function pointers in comparisons
	comparisonPattern := regexp.MustCompile(`icmp\s+([a-z]+)\s+(i[0-9]+)\s+\(([^)]+)\)\s+(%[0-9]+)`)
	ir = comparisonPattern.ReplaceAllString(ir, `icmp $1 $2 $4`)

	// Fix function pointers in memory operations
	memoryPattern := regexp.MustCompile(`(load|store)\s+(i[0-9]+)\s+\(([^)]+)\)\s*,\s*(i[0-9]+)\s*\*\s*(%[0-9]+)`)
	ir = memoryPattern.ReplaceAllString(ir, `$1 $2, $4* $5`)

	// Fix function pointers in phi nodes
	phiPattern := regexp.MustCompile(`phi\s+(i[0-9]+)\s+\(([^)]+)\)\s+\[([^,]+),\s*(%[a-zA-Z0-9.]+)\]`)
	ir = phiPattern.ReplaceAllString(ir, `phi $1 [$3, $4]`)

	// Fix function pointers in bitcast operations
	bitcastPattern := regexp.MustCompile(`bitcast\s+(i[0-9]+)\s+\(([^)]+)\)\s+(%[0-9]+)\s+to`)
	ir = bitcastPattern.ReplaceAllString(ir, `bitcast $1 $3 to`)

	// Fix common syntax errors in function declarations
	fnDeclPattern := regexp.MustCompile(`declare\s+(i[0-9]+|void)\s+\(([^)]+)\)\s+@([a-zA-Z0-9_]+)`)
	ir = fnDeclPattern.ReplaceAllString(ir, `declare $1 @$3($2)`)

	// Fix function definitions with wrong syntax
	// e.g., "define i32 (i32) @factorial()" -> "define i32 @factorial(i32)"
	funcDefPattern := regexp.MustCompile(`define\s+([a-zA-Z0-9*]+)\s+\(([^)]+)\)\s+@([a-zA-Z0-9_]+)\(\)`)
	ir = funcDefPattern.ReplaceAllString(ir, `define $1 @$3($2)`)

	// Remove duplicate function declarations
	lines := strings.Split(ir, "\n")
	declaredFunctions := make(map[string]bool)
	cleanedLines := make([]string, 0, len(lines))

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "declare ") {
			// Extract function name
			namePattern := regexp.MustCompile(`@([a-zA-Z0-9_]+)`)
			matches := namePattern.FindStringSubmatch(line)

			if len(matches) >= 2 {
				name := matches[1]
				if declaredFunctions[name] {
					// Skip duplicate declaration
					continue
				}
				declaredFunctions[name] = true
			}
		}

		cleanedLines = append(cleanedLines, line)
	}

	return strings.Join(cleanedLines, "\n")
}

func fixAdvancedInstructionOrdering(ir string) string {
	// Split IR into lines to process each line individually
	lines := strings.Split(ir, "\n")

	// Process each function separately
	currentFunction := ""
	functions := make(map[string][]string) // Maps function name to all lines in that function
	functionStarts := make(map[string]int) // Maps function name to starting line index
	functionEnds := make(map[string]int)   // Maps function name to ending line index

	// First pass: Identify function boundaries
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Detect function definition
		if strings.HasPrefix(trimmedLine, "define ") && strings.Contains(trimmedLine, "@") {
			funcNamePattern := regexp.MustCompile(`@([a-zA-Z0-9_.]+)`)
			matches := funcNamePattern.FindStringSubmatch(trimmedLine)
			if len(matches) >= 2 {
				if currentFunction != "" {
					// End previous function
					functionEnds[currentFunction] = i - 1
				}

				currentFunction = matches[1]
				functionStarts[currentFunction] = i
				functions[currentFunction] = []string{}
			}
		}

		// Add the line to the current function if we're in one
		if currentFunction != "" {
			functions[currentFunction] = append(functions[currentFunction], line)
		}
	}

	// End the last function if there is one
	if currentFunction != "" {
		functionEnds[currentFunction] = len(lines) - 1
	}

	// Second pass: Process each function individually
	for funcName, funcLines := range functions {
		// Create a map of register definitions and usages
		registerDefs := make(map[string]string)  // Maps register to its definition line
		registerTypes := make(map[string]string) // Maps register to its type
		registerUsages := make(map[string][]int) // Maps register to the line numbers where it's used
		nextRegisterNumber := 1

		// Scan each line in the function to identify register definitions and usages
		for i, line := range funcLines {
			trimmedLine := strings.TrimSpace(line)

			// Find register definitions (lines containing " = ")
			if strings.Contains(trimmedLine, " = ") {
				defPattern := regexp.MustCompile(`\s*(%[0-9]+)\s*=\s*([^,]+)`)
				matches := defPattern.FindStringSubmatch(trimmedLine)
				if len(matches) >= 3 {
					reg := matches[1]
					opType := strings.TrimSpace(matches[2])

					// Extract the type of the operation result
					typePattern := regexp.MustCompile(`(alloca|load|store|add|sub|mul|div|rem|icmp|call|phi|select|bitcast|ptrtoint|inttoptr|getelementptr|zext|sext|trunc|fadd|fsub|fmul|fdiv)\s+([^ ,]+)`)
					typeMatches := typePattern.FindStringSubmatch(opType)
					if len(typeMatches) >= 3 {
						registerDefs[reg] = line
						registerTypes[reg] = typeMatches[2]
					}
				}
			}

			// Find register usages
			// Instead of negative lookahead, we'll parse all registers and filter them
			usagePattern := regexp.MustCompile(`%[0-9]+`)
			usageMatches := usagePattern.FindAllString(trimmedLine, -1)

			// Extract the register being defined in this line (if any)
			var definedReg string
			if strings.Contains(trimmedLine, " = ") {
				defMatch := regexp.MustCompile(`\s*(%[0-9]+)\s*=`).FindStringSubmatch(trimmedLine)
				if len(defMatch) >= 2 {
					definedReg = defMatch[1]
				}
			}

			// Process all register references except the one being defined
			for _, reg := range usageMatches {
				// Skip if this is the register being defined on this line
				if reg == definedReg {
					continue
				}

				// If the register hasn't been seen before, record the usage
				if _, exists := registerUsages[reg]; !exists {
					registerUsages[reg] = []int{}
				}
				registerUsages[reg] = append(registerUsages[reg], i)
			}
		}

		// Process register renaming to ensure proper ordering
		registerMap := make(map[string]string) // Maps old register to new register

		// First rename registers that are defined before they're used
		for oldReg := range registerDefs {
			newReg := fmt.Sprintf("%%%d", nextRegisterNumber)
			nextRegisterNumber++
			registerMap[oldReg] = newReg
		}

		// Now apply the renaming to the function lines
		for i, line := range funcLines {
			// Replace register definitions
			if strings.Contains(line, " = ") {
				defPattern := regexp.MustCompile(`\s*(%[0-9]+)\s*=`)
				matches := defPattern.FindStringSubmatch(line)
				if len(matches) >= 2 {
					oldReg := matches[1]
					if newReg, exists := registerMap[oldReg]; exists {
						funcLines[i] = strings.Replace(line, oldReg+" =", newReg+" =", 1)
					}
				}
			}

			// Replace register usages (but not in definitions)
			for oldReg, newReg := range registerMap {
				// Skip if this line defines this register (we already handled that above)
				if strings.Contains(line, oldReg+" =") {
					continue
				}

				// This ensures we only replace complete register names, not partial matches
				pattern := regexp.MustCompile(`(^|[^%0-9])` + regexp.QuoteMeta(oldReg) + `($|[^0-9])`)
				funcLines[i] = pattern.ReplaceAllString(funcLines[i], "${1}"+newReg+"${2}")
			}
		}

		// Update the original lines with the processed function
		startIdx := functionStarts[funcName]
		endIdx := functionEnds[funcName]
		for i := 0; i < endIdx-startIdx+1 && i < len(funcLines); i++ {
			lines[startIdx+i] = funcLines[i]
		}
	}

	// Third pass: Fix function pointers
	ir = strings.Join(lines, "\n")
	ir = fixFunctionPointerUsage(ir)

	return ir
}

// Enhanced fix for all types of LLVM IR issues
func completeIRFix(ir string) string {
	// Process string constants
	ir = processStringConstants(ir)

	// Fix instruction numbering with advanced type and order handling
	ir = fixAdvancedInstructionOrdering(ir)

	// Fix function pointers and other common issues
	ir = fixCommonIRIssues(ir)

	// Fix function declarations
	ir = fixFunctionDeclarations(ir)

	// Remove extraneous whitespace from IR
	ir = regularizeWhitespace(ir)

	return ir
}

// Function to regularize whitespace in the IR for better readability
func regularizeWhitespace(ir string) string {
	lines := strings.Split(ir, "\n")
	for i, line := range lines {
		// Trim leading/trailing whitespace
		line = strings.TrimSpace(line)

		// Ensure consistent spacing around operators
		line = regexp.MustCompile(`\s+`).ReplaceAllString(line, " ")
		line = regexp.MustCompile(`(\s*=\s*)`).ReplaceAllString(line, " = ")
		line = regexp.MustCompile(`(\s*,\s*)`).ReplaceAllString(line, ", ")

		lines[i] = line
	}
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
				processedFunctionLines := rewriteRegisters(functionLines)
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
			processedFunctionLines := rewriteRegisters(functionLines)
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

func rewriteRegisters(lines []string) []string {
	// First, identify all register definitions and their dependencies
	registers := make(map[string]struct{})
	registerDefs := make(map[string]int)   // Maps register to the line where it's defined
	registerUses := make(map[string][]int) // Maps register to lines where it's used

	// First pass - identify all registers
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Find register definitions
		if strings.Contains(trimmedLine, " = ") {
			defPattern := regexp.MustCompile(`\s*(%[0-9]+)\s*=`)
			matches := defPattern.FindStringSubmatch(trimmedLine)
			if len(matches) >= 2 {
				reg := matches[1]
				registers[reg] = struct{}{}
				registerDefs[reg] = i
			}
		}

		// Find all register usages on this line
		usePattern := regexp.MustCompile(`%[0-9]+`)
		useMatches := usePattern.FindAllString(trimmedLine, -1)
		for _, reg := range useMatches {
			registers[reg] = struct{}{}

			// Only track uses, not definitions
			if !strings.Contains(trimmedLine, reg+" =") {
				registerUses[reg] = append(registerUses[reg], i)
			}
		}
	}

	// Build a dependency graph
	dependencies := make(map[string][]string) // register -> registers it depends on
	for reg := range registers {
		dependencies[reg] = []string{}
	}

	// For each register use, track which definition it depends on
	for reg, useLines := range registerUses {
		for _, useLine := range useLines {
			// Find which register definition this use depends on
			for defReg, defLine := range registerDefs {
				if defLine < useLine { // Definition must come before use
					dependencies[reg] = append(dependencies[reg], defReg)
				}
			}
		}
	}

	// Create a new mapping for registers
	newRegisterMap := make(map[string]string)
	nextRegNum := 1

	// Process all registers in order of their appearance in the function
	for _, line := range lines {
		// First process definitions
		if strings.Contains(line, " = ") {
			defPattern := regexp.MustCompile(`\s*(%[0-9]+)\s*=`)
			matches := defPattern.FindStringSubmatch(line)
			if len(matches) >= 2 {
				oldReg := matches[1]
				if _, exists := newRegisterMap[oldReg]; !exists {
					newRegisterMap[oldReg] = fmt.Sprintf("%%%d", nextRegNum)
					nextRegNum++
				}
			}
		}

		// Then process uses
		usePattern := regexp.MustCompile(`%[0-9]+`)
		useMatches := usePattern.FindAllString(line, -1)
		for _, oldReg := range useMatches {
			if _, exists := newRegisterMap[oldReg]; !exists {
				// If we see a use before a definition, still assign it a new number
				newRegisterMap[oldReg] = fmt.Sprintf("%%%d", nextRegNum)
				nextRegNum++
			}
		}
	}

	// Apply the mapping to rewrite the function
	rewrittenLines := make([]string, len(lines))
	for i, line := range lines {
		rewrittenLine := line

		// First replace occurrences that are not definitions (to avoid partial matches)
		for oldReg, newReg := range newRegisterMap {
			// Skip if line contains the definition of this register
			if strings.Contains(line, oldReg+" =") {
				continue
			}

			// Replace the register, being careful about partial matches
			pattern := regexp.MustCompile(`(^|[^%0-9])` + regexp.QuoteMeta(oldReg) + `($|[^0-9])`)
			rewrittenLine = pattern.ReplaceAllString(rewrittenLine, "${1}"+newReg+"${2}")
		}

		// Then replace definitions
		if strings.Contains(rewrittenLine, " = ") {
			for oldReg, newReg := range newRegisterMap {
				if strings.Contains(rewrittenLine, oldReg+" =") {
					rewrittenLine = strings.Replace(rewrittenLine, oldReg+" =", newReg+" =", 1)
					break // Only one definition per line
				}
			}
		}

		rewrittenLines[i] = rewrittenLine
	}

	return rewrittenLines
}

func fixLLVMRegisters(ir string) string {
	// Split into lines
	lines := strings.Split(ir, "\n")
	processingFunction := false
	functionLines := []string{}
	resultLines := []string{}

	for _, line := range lines {
		// Detect function boundaries
		if strings.HasPrefix(strings.TrimSpace(line), "define ") {
			// If we were processing a function, finalize it
			if processingFunction {
				// Process the previous function
				processedLines := renumberRegistersInFunction(functionLines)
				resultLines = append(resultLines, processedLines...)
				functionLines = []string{}
			}

			// Start a new function
			processingFunction = true
			functionLines = append(functionLines, line)
		} else if processingFunction && strings.HasPrefix(strings.TrimSpace(line), "}") {
			// End of function
			functionLines = append(functionLines, line)
			processedLines := renumberRegistersInFunction(functionLines)
			resultLines = append(resultLines, processedLines...)
			processingFunction = false
			functionLines = []string{}
		} else if processingFunction {
			// Collecting lines within a function
			functionLines = append(functionLines, line)
		} else {
			// Outside functions, pass through
			resultLines = append(resultLines, line)
		}
	}

	// Handle any remaining function
	if processingFunction && len(functionLines) > 0 {
		processedLines := renumberRegistersInFunction(functionLines)
		resultLines = append(resultLines, processedLines...)
	}

	return strings.Join(resultLines, "\n")
}

func renumberRegistersInFunction(lines []string) []string {
	// Map of old register names to new ones
	registerMap := make(map[string]string)
	nextRegister := 1

	// Process each line
	processedLines := make([]string, len(lines))

	// Two-pass approach: first collect all register definitions
	for i, line := range lines {
		// Copy the line initially
		processedLines[i] = line

		// Look for register definitions
		if strings.Contains(line, " = ") {
			regDefPattern := regexp.MustCompile(`\s*(%[0-9]+)\s*=`)
			matches := regDefPattern.FindStringSubmatch(line)
			if len(matches) >= 2 {
				oldReg := matches[1]
				if _, exists := registerMap[oldReg]; !exists {
					registerMap[oldReg] = fmt.Sprintf("%%%d", nextRegister)
					nextRegister++
				}
			}
		}

		// Look for register usages (including in phi nodes, etc.)
		regUsePattern := regexp.MustCompile(`%[0-9]+`)
		matches := regUsePattern.FindAllString(line, -1)
		for _, oldReg := range matches {
			if oldReg != "" && !strings.Contains(line, oldReg+" =") {
				if _, exists := registerMap[oldReg]; !exists {
					registerMap[oldReg] = fmt.Sprintf("%%%d", nextRegister)
					nextRegister++
				}
			}
		}
	}

	// Second pass: apply the register mapping
	for i, line := range lines {
		newLine := line

		// Replace register definitions
		if strings.Contains(line, " = ") {
			regDefPattern := regexp.MustCompile(`\s*(%[0-9]+)\s*=`)
			matches := regDefPattern.FindStringSubmatch(line)
			if len(matches) >= 2 {
				oldReg := matches[1]
				if newReg, exists := registerMap[oldReg]; exists {
					newLine = strings.Replace(newLine, oldReg+" =", newReg+" =", 1)
				}
			}
		}

		// Replace register usages
		for oldReg, newReg := range registerMap {
			// Don't replace in definitions
			if !strings.Contains(newLine, oldReg+" =") {
				// Ensure we only replace whole register names
				pattern := regexp.MustCompile(`(^|[^%0-9])` + regexp.QuoteMeta(oldReg) + `($|[^0-9])`)
				newLine = pattern.ReplaceAllString(newLine, "${1}"+newReg+"${2}")
			}
		}

		processedLines[i] = newLine
	}

	return processedLines
}
