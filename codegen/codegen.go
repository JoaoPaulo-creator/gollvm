package codegen

import (
	"compiler/ast"
	"fmt"
	"os"
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
	declarePrintf(module)

	// Add runtime functions
	declareRuntime(module)

	return &Generator{
		module:        module,
		context:       context,
		stringCounter: 0,
		blockCounter:  0,
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

// declarePrintf declares the printf function
func declarePrintf(module *ir.Module) {
	// int printf(const char *format, ...);
	printfType := types.NewFunc(types.I32, types.NewPointer(types.I8))
	fn := module.NewFunc("printf", printfType)
	fn.Sig.Variadic = true
}

// declareRuntime declares runtime functions
func declareRuntime(module *ir.Module) {
	// Memory allocation functions
	mallocType := types.NewFunc(types.NewPointer(types.I8), types.I64)
	module.NewFunc("malloc", mallocType)

	freeType := types.NewFunc(types.Void, types.NewPointer(types.I8))
	module.NewFunc("free", freeType)

	// String handling functions
	strlenType := types.NewFunc(types.I64, types.NewPointer(types.I8))
	module.NewFunc("strlen", strlenType)

	strcpyType := types.NewFunc(types.NewPointer(types.I8), types.NewPointer(types.I8), types.NewPointer(types.I8))
	module.NewFunc("strcpy", strcpyType)

	// Math functions that might be useful
	absType := types.NewFunc(types.I32, types.I32)
	module.NewFunc("abs", absType)

	powType := types.NewFunc(types.Double, types.Double, types.Double)
	module.NewFunc("pow", powType)

	// Exit function
	exitType := types.NewFunc(types.Void, types.I32)
	module.NewFunc("exit", exitType)
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

		fmt.Fprintf(os.Stderr, "Processing statement %d in main: %T\n", i, stmt)
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
				fmt.Fprintf(os.Stderr, "Adding terminator to block %s in function %s\n",
					block.Name, fn.Name())

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
func (g *Generator) generateStatement(stmt ast.Statement) (value.Value, error) {
	switch stmt := stmt.(type) {
	case *ast.VarStatement:
		return g.generateVarStatement(stmt)
	case *ast.ReturnStatement:
		return g.generateReturnStatement(stmt)
	case *ast.ExpressionStatement:
		fmt.Fprintf(os.Stderr, "Processing expression statement with expression: %T\n", stmt.Expression)
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
	default:
		return nil, fmt.Errorf("unknown statement type: %T", stmt)
	}
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

	fmt.Fprintf(os.Stderr, "Expression type: %T\n", expr)
}

// func (g *Generator) debugExpression(expr ast.Expression) {
// 	if expr == nil {
// 		fmt.Fprintf(os.Stderr, "NIL EXPRESSION DETECTED\n")
// 		// Print stack trace
// 		buf := make([]byte, 1024)
// 		n := runtime.Stack(buf, false)
// 		fmt.Fprintf(os.Stderr, "Stack trace: %s\n", buf[:n])
// 		return
// 	}
//
// 	fmt.Fprintf(os.Stderr, "Expression type: %T\n", expr)
// }

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
		var initFunc *ir.Func
		var initBlock *ir.Block

		// Check if global_init exists
		for _, fn := range g.module.Funcs {
			if fn.Name() == "global_init" {
				initFunc = fn
				break
			}
		}

		// Create global_init if not exists
		if initFunc == nil {
			initFunc = g.module.NewFunc("global_init", types.Void)
			initBlock = initFunc.NewBlock("entry")
			initBlock.NewRet(nil)
		} else {
			if len(initFunc.Blocks) > 0 {
				initBlock = initFunc.Blocks[0]
			} else {
				initBlock = initFunc.NewBlock("entry")
				initBlock.NewRet(nil)
			}
		}

		// Generate the value
		val, err := g.generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}

		// Handle function declarations specially
		if funcVal, ok := val.(*ir.Func); ok {
			fmt.Fprintf(os.Stderr, "Found function declaration for %s\n", stmt.Name.Value)

			// Simply use the function directly
			g.context.namedValues[stmt.Name.Value] = funcVal

			// If needed, you can rename the function
			if funcVal.Name() != stmt.Name.Value && len(funcVal.Blocks) == 0 {
				// Only rename empty functions (declarations)
				funcVal.SetName(stmt.Name.Value)
			}

			return funcVal, nil
		} else {
			// Create global variable with zero value
			global := g.module.NewGlobal(stmt.Name.Value, val.Type())
			global.Init = constant.NewZeroInitializer(val.Type())
			initBlock.NewStore(val, global)
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
		fmt.Fprintf(os.Stderr, "Generating statement %d in block: %T\n", i, s)
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
// generatePrintStatement generates code for a print statement
func (g *Generator) generatePrintStatement(stmt *ast.PrintStatement) (value.Value, error) {
	// Check if we have a current function
	if g.context.currentFunction == nil {
		fmt.Fprintf(os.Stderr, "ERROR: No current function when generating print statement\n")
		return constant.NewInt(types.I32, 0), nil
	}

	// Check if we have any blocks in the function
	if len(g.context.currentFunction.Blocks) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: Function has no blocks when generating print statement\n")
		// Create an entry block as a fallback
		entryBlock := g.context.currentFunction.NewBlock("entry")
		g.context.blocks["entry"] = entryBlock
	}

	// Generate the value to print
	val, err := g.generateExpression(stmt.Value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to generate value for print statement: %v\n", err)
		return nil, err
	}

	// Check if value is nil
	if val == nil {
		fmt.Fprintf(os.Stderr, "WARNING: Print expression evaluated to nil\n")
		val = constant.NewInt(types.I32, 0) // Default value
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
		formatStr = "%d\\n"
		printVal = val
	case *types.FloatType:
		formatStr = "%f\\n"
		printVal = val
	case *types.PointerType:
		// Check if it's a string
		pointerType, ok := val.Type().(*types.PointerType)
		if !ok {
			return nil, fmt.Errorf("expected pointer type, got %T", val.Type())
		}
		if pointerType.ElemType != nil && pointerType.ElemType.Equal(types.I8) {
			formatStr = "%s\\n"
			printVal = val
		} else {
			formatStr = "%p\\n"
			printVal = val
		}
	default:
		// For any other type, convert to string
		formatStr = "%s\\n"
		printVal = val
	}

	// Create format string constant
	formatStrConst := g.getStringConstant(formatStr)

	// Generate printf call - find printf function
	var printfFn *ir.Func
	for _, fn := range g.module.Funcs {
		if fn.Name() == "printf" {
			printfFn = fn
			break
		}
	}

	if printfFn == nil {
		fmt.Fprintf(os.Stderr, "ERROR: printf function not found\n")
		return constant.NewInt(types.I32, 0), nil
	}

	// Create the call with panic protection
	var result value.Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "PANIC in printf call: %v\n", r)
				result = nil
			}
		}()

		switch val.Type().(type) {
		case *types.IntType:
			result = currentBlock.NewCall(printfFn, formatStrConst, printVal)
		case *types.FloatType:
			result = currentBlock.NewCall(printfFn, formatStrConst, printVal)
		case *types.PointerType:
			result = currentBlock.NewCall(printfFn, formatStrConst, printVal)
		default:
			// Use a generic string format for other types
			result = currentBlock.NewCall(printfFn, formatStrConst)
		}
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

	// Create a new string constant
	strType := types.NewArray(uint64(len(str)+1), types.I8)
	strConst := g.module.NewGlobalDef(fmt.Sprintf(".str.%d", g.stringCounter), constant.NewCharArrayFromString(str+"\\00"))
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
	// Debug the lookup
	fmt.Fprintf(os.Stderr, "Looking up identifier: %s\n", expr.Value)

	// Special handling for function identifiers - try to find the function in the module first
	for _, fn := range g.module.Funcs {
		if fn.Name() == expr.Value {
			fmt.Fprintf(os.Stderr, "Found function %s as global identifier\n", expr.Value)
			return fn, nil
		}
	}

	// Then look up the variable in the context
	val, ok := g.context.Lookup(expr.Value)
	if !ok {
		// Debug output to help diagnose the issue
		fmt.Fprintf(os.Stderr, "Undefined variable: %s\n", expr.Value)
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

	// Debug output
	fmt.Fprintf(os.Stderr, "Creating function: %s with %d parameters\n", funcName, len(expr.Parameters))

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

	// Debug the current function state
	fmt.Fprintf(os.Stderr, "Function %s created with entry block, function has %d blocks now\n",
		funcName, len(fn.Blocks))

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

		// Add to named values - this is critical to fix the lookup issue
		fmt.Fprintf(os.Stderr, "Adding parameter %s to named values for function %s\n", param.Value, funcName)
		g.context.namedValues[param.Value] = paramAlloca
	}

	// Generate code for the body
	fmt.Fprintf(os.Stderr, "Generating body for function: %s, function has %d blocks\n",
		funcName, len(fn.Blocks))
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

	// Debug output when function is complete
	fmt.Fprintf(os.Stderr, "Successfully generated function: %s\n", funcName)

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

	// Debug the function type
	fmt.Fprintf(os.Stderr, "Function type in call: %T - %v\n", function.Type(), function.Type())

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
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = g.module.WriteTo(f)
	return err
}

// CompileToLLVM compiles a program to LLVM IR
func CompileToLLVM(program *ast.Program) (*ir.Module, error) {
	// Create a generator
	generator := New()

	// Generate code for the program
	module, err := generator.Generate(program)
	if err != nil {
		return nil, err
	}

	return module, nil
}
