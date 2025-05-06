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
func (g *Generator) generateStatement(stmt ast.Statement) (value.Value, error) {
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
		formatStr = "%d\n" // Use actual newline, not escape sequence
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
			formatStr = "%s\n" // No escape sequence for newlines
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

	// Generate printf call - find printf function
	var printfFn *ir.Func
	for _, fn := range g.module.Funcs {
		if fn.Name() == "printf" {
			printfFn = fn
			break
		}
	}

	printfFn = g.printfFunc

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

	// Process escapes properly - Convert \n to actual newlines, etc.
	processedStr := strings.ReplaceAll(str, "\\n", "\n")

	// Create a new string constant - use proper null termination
	strType := types.NewArray(uint64(len(processedStr)+1), types.I8)
	strConst := g.module.NewGlobalDef(fmt.Sprintf(".str.%d", g.stringCounter),
		constant.NewCharArrayFromString(processedStr+"\x00"))
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
			// Get current block
			if g.context.currentFunction == nil || len(g.context.currentFunction.Blocks) == 0 {
				fmt.Fprintf(os.Stderr, "WARNING: No current function or blocks when generating identifier\n")
				return constant.NewInt(types.I32, 0), nil
			}
			currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]
			
			// Create a proper function pointer type
			funcType := fn.Type()
			funcPtrType := types.NewPointer(funcType)
			
			// Create a pointer to the function
			return currentBlock.NewBitCast(fn, funcPtrType), nil
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
	if fn, ok := val.(*ir.Func); ok {
		// Get current block
		if g.context.currentFunction == nil || len(g.context.currentFunction.Blocks) == 0 {
			fmt.Fprintf(os.Stderr, "WARNING: No current function or blocks when generating identifier\n")
			return constant.NewInt(types.I32, 0), nil
		}
		currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]
		
		// Create a proper function pointer type
		funcType := fn.Type()
		funcPtrType := types.NewPointer(funcType)
		
		// Create a pointer to the function
		return currentBlock.NewBitCast(fn, funcPtrType), nil
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
		if funcType, ok := pointerType.ElemType.(*types.FuncType); ok {
			// For function pointers, load the value
			return currentBlock.NewLoad(funcType, val), nil
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

	// Convert function references to pointers if needed
	if _, ok := left.(*ir.Func); ok {
		// Create a pointer to the function
		funcPtrType := types.NewPointer(left.Type())
		left = currentBlock.NewBitCast(left, funcPtrType)
	}
	if _, ok := right.(*ir.Func); ok {
		// Create a pointer to the function
		funcPtrType := types.NewPointer(right.Type())
		right = currentBlock.NewBitCast(right, funcPtrType)
	}

	// For arithmetic operations, we need to ensure both operands are integers
	if expr.Operator == "+" || expr.Operator == "-" || expr.Operator == "*" || expr.Operator == "/" || expr.Operator == "%" {
		// Check if either operand is a function pointer
		if _, ok := left.Type().(*types.PointerType); ok {
			fmt.Fprintf(os.Stderr, "WARNING: Cannot perform arithmetic on function pointer\n")
			return constant.NewInt(types.I32, 0), nil
		}
		if _, ok := right.Type().(*types.PointerType); ok {
			fmt.Fprintf(os.Stderr, "WARNING: Cannot perform arithmetic on function pointer\n")
			return constant.NewInt(types.I32, 0), nil
		}
	}

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

	// Get current block
	if g.context.currentFunction == nil || len(g.context.currentFunction.Blocks) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: No current function or blocks when generating call expression\n")
		return constant.NewInt(types.I32, 0), nil
	}
	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

	// Handle function pointer types
	var callableFunction value.Value
	if funcPtr, ok := function.Type().(*types.PointerType); ok {
		if _, ok := funcPtr.ElemType.(*types.FuncType); ok {
			// It's a function pointer, use it directly
			callableFunction = function
		} else {
			// Try to load the function pointer
			callableFunction = currentBlock.NewLoad(funcPtr.ElemType, function)
		}
	} else if fn, ok := function.(*ir.Func); ok {
		// It's a direct function reference, create a pointer to it
		funcPtrType := types.NewPointer(fn.Type())
		callableFunction = currentBlock.NewBitCast(fn, funcPtrType)
	} else {
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

	// Generate the call
	var callResult value.Value
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

	// Create function with explicit type
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

	// Create a pointer to the function
	if g.context.currentFunction != nil && len(g.context.currentFunction.Blocks) > 0 {
		currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]
		funcPtrType := types.NewPointer(fn.Type())
		return currentBlock.NewBitCast(fn, funcPtrType), nil
	}

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

	// Apply fixes to function declarations
	ir = fixFunctionDeclarations(ir)

	// Remove any duplicate function declarations
	ir = removeDuplicateDeclarations(ir)

	return ir, nil
}

func getStandardFunctionDeclarations() string {
	return `
; Standard C library function declarations
declare i32 @printf(i8*, ...)
declare i8* @malloc(i64)
declare void @free(i8*)
declare i64 @strlen(i8*)
declare i8* @strcpy(i8*, i8*)
declare i32 @abs(i32)
declare double @pow(double, double)
declare void @exit(i32)

; Format strings for clean output
@.fmt.int = private constant [4 x i8] c"%d\0A\00"
`
}

func fixFunctionDeclarations(ir string) string {
	// Fix the specific issue with printf having duplicate variadic markers
	ir = strings.Replace(ir,
		"declare i32 @printf(i8*, ...)(...).",
		"declare i32 @printf(i8*, ...).",
		-1)

	// Fix printf declaration (handling all possible patterns)
	patterns := []string{
		"declare i32 @printf(i8*)",
		"declare i32 (i8*) @printf",
		"declare i32 (i8*) @printf(...)",
		"declare i32 @printf(i8*, ...)(...)", // This is the problematic pattern!
	}

	for _, pattern := range patterns {
		ir = strings.Replace(ir, pattern, "declare i32 @printf(i8*, ...)", -1)
	}

	// Fix other C library functions
	functionFixes := map[string]string{
		"declare i8* (i64) @malloc()":            "declare i8* @malloc(i64)",
		"declare void (i8*) @free()":             "declare void @free(i8*)",
		"declare i64 (i8*) @strlen()":            "declare i64 @strlen(i8*)",
		"declare i8* (i8*, i8*) @strcpy()":       "declare i8* @strcpy(i8*, i8*)",
		"declare i32 (i32) @abs()":               "declare i32 @abs(i32)",
		"declare double (double, double) @pow()": "declare double @pow(double, double)",
		"declare void (i32) @exit()":             "declare void @exit(i32)",
	}

	for pattern, replacement := range functionFixes {
		ir = strings.Replace(ir, pattern, replacement, -1)
	}

	// Fix function definitions with incorrect return types
	// This regex matches function definitions with incorrect return type format
	funcDefPattern := regexp.MustCompile(`define\s+(\w+)\s*\(([^)]*)\)\s*@(\w+)\s*\(([^)]*)\)\s*{`)
	ir = funcDefPattern.ReplaceAllStringFunc(ir, func(match string) string {
		parts := funcDefPattern.FindStringSubmatch(match)
		if len(parts) != 5 {
			return match
		}
		returnType := parts[1]
		funcName := parts[3]
		params := parts[4]
		return fmt.Sprintf("define %s @%s(%s) {", returnType, funcName, params)
	})

	// Fix function pointer types
	funcPtrPattern := regexp.MustCompile(`i32\s*\(i32\)\s*\(\)\*`)
	ir = funcPtrPattern.ReplaceAllStringFunc(ir, func(match string) string {
		return "i32 (i32)*"
	})

	// Fix function pointer casts
	funcCastPattern := regexp.MustCompile(`bitcast\s+i32\s*\(i32\)\s*\(\)\*\s*@(\w+)\s+to\s+i32\s*\(i32\)\s*\(\)\*\*`)
	ir = funcCastPattern.ReplaceAllStringFunc(ir, func(match string) string {
		parts := funcCastPattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		funcName := parts[1]
		return fmt.Sprintf("bitcast i32 (i32)* @%s to i32 (i32)**", funcName)
	})

	// Fix function references in arithmetic operations
	funcArithPattern := regexp.MustCompile(`(\w+)\s+i32\s*\(i32\)\s*@(\w+)`)
	ir = funcArithPattern.ReplaceAllStringFunc(ir, func(match string) string {
		parts := funcArithPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		op := parts[1]
		funcName := parts[2]
		// Replace the function reference with a pointer to the function
		return fmt.Sprintf("%s i32 (i32)* @%s", op, funcName)
	})

	constPattern := regexp.MustCompile(`@\.str\.[0-9]+ = .*c"([^"]*)".*`)
	ir = constPattern.ReplaceAllStringFunc(ir, func(s string) string {
		matches := constPattern.FindStringSubmatch(s)
		if len(matches) > 1 {
			// Process escape sequences properly
			content := strings.ReplaceAll(matches[1], "\\n", "\n")
			content = strings.ReplaceAll(content, "\\00", "\x00")
			// Recreate the string with proper escaping
			return strings.Replace(s, matches[1], content, 1)
		}
		return s
	})

	return ir
}

// removeDuplicateDeclarations removes duplicate function declarations from the IR
func removeDuplicateDeclarations(ir string) string {
	// Split the IR into lines
	lines := strings.Split(ir, "\n")

	// Keep track of seen declarations
	seenDecls := make(map[string]bool)
	var result []string

	for _, line := range lines {
		// Check if this is a function declaration
		if strings.HasPrefix(strings.TrimSpace(line), "declare") {
			// Extract the function name
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				funcName := parts[len(parts)-1]
				// Remove the @ symbol if present
				funcName = strings.TrimPrefix(funcName, "@")
				// Remove any trailing parameters
				if idx := strings.Index(funcName, "("); idx != -1 {
					funcName = funcName[:idx]
				}

				// If we haven't seen this declaration before, keep it
				if !seenDecls[funcName] {
					seenDecls[funcName] = true
					result = append(result, line)
				}
			} else {
				result = append(result, line)
			}
		} else {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}
