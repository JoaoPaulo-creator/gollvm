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
	printfFunc    *ir.Func
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
	// Create printf type: i32 (i8*, ...)
	printfType := types.NewFunc(types.I32, types.NewPointer(types.I8))
	fn := module.NewFunc("printf", printfType)
	fn.Linkage = enum.LinkageExternal
	fn.Sig.Variadic = true

	// Add metadata to indicate this is a C varargs function
	fn.CallingConv = enum.CallingConvC

	// Name the format parameter
	if len(fn.Params) > 0 {
		fn.Params[0].SetName("format")
	}
	return fn
}

func declareExternalFunction(module *ir.Module, name string, retType types.Type, paramTypes ...types.Type) *ir.Func {
	funcType := types.NewFunc(retType, paramTypes...)
	fn := module.NewFunc(name, funcType)
	for i := range paramTypes {
		if i < len(fn.Params) {
			fn.Params[i].SetName(fmt.Sprintf("param%d", i))
		}
	}
	return fn
}

func declareRuntime(module *ir.Module) {
	declareExternalFunction(module, "malloc", types.NewPointer(types.I8), types.I64)
	declareExternalFunction(module, "free", types.Void, types.NewPointer(types.I8))
	declareExternalFunction(module, "strlen", types.I64, types.NewPointer(types.I8))
	declareExternalFunction(module, "strcpy", types.NewPointer(types.I8),
		types.NewPointer(types.I8), types.NewPointer(types.I8))
	declareExternalFunction(module, "abs", types.I32, types.I32)
	declareExternalFunction(module, "pow", types.Double, types.Double, types.Double)
	declareExternalFunction(module, "exit", types.Void, types.I32)
}

func (g *Generator) Generate(program *ast.Program) (*ir.Module, error) {
	// First pass: process function declarations
	fmt.Fprintf(os.Stderr, "DEBUG: Starting Generate method\n")
	fmt.Fprintf(os.Stderr, "DEBUG: Total statements: %d\n", len(program.Statements))

	// Process function declarations first
	for i, stmt := range program.Statements {
		if varStmt, ok := stmt.(*ast.VarStatement); ok {
			if _, ok := varStmt.Value.(*ast.FunctionLiteral); ok {
				_, err := g.generateStatement(stmt)
				if err != nil {
					return nil, fmt.Errorf("error processing function declaration %d: %w", i, err)
				}
			}
		} else if exprStmt, ok := stmt.(*ast.ExpressionStatement); ok {
			if funcLit, ok := exprStmt.Expression.(*ast.FunctionLiteral); ok {
				_, err := g.generateExpression(funcLit)
				if err != nil {
					return nil, fmt.Errorf("error processing function declaration %d: %w", i, err)
				}
			}
		}
	}

	// Create main function
	fmt.Fprintf(os.Stderr, "DEBUG: Creating main function\n")
	mainFunc := g.module.NewFunc("main", types.NewFunc(types.I32))
	mainBlock := mainFunc.NewBlock("entry")
	g.context.currentFunction = mainFunc
	g.context.blocks["entry"] = mainBlock

	fmt.Fprintf(os.Stderr, "DEBUG: Processing statements in main function\n")
	// Process statements in main function
	for i, stmt := range program.Statements {
		if varStmt, ok := stmt.(*ast.VarStatement); ok {
			if _, ok := varStmt.Value.(*ast.FunctionLiteral); ok {
				continue
			}
		} else if exprStmt, ok := stmt.(*ast.ExpressionStatement); ok {
			if _, ok := exprStmt.Expression.(*ast.FunctionLiteral); ok {
				continue
			}
		}

		fmt.Fprintf(os.Stderr, "DEBUG: Generating statement %d: %T\n", i, stmt)
		_, err := g.generateStatement(stmt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to generate statement %d: %v\n", i, err)
			return nil, fmt.Errorf("error processing statement %d: %w", i, err)
		}
	}

	// Ensure main function has a return
	fmt.Fprintf(os.Stderr, "DEBUG: Checking main function terminator\n")
	if mainBlock.Term == nil {
		fmt.Fprintf(os.Stderr, "DEBUG: Adding default return to main function\n")
		mainBlock.NewRet(constant.NewInt(types.I32, 0))
	}

	// Ensure all functions have a return
	fmt.Fprintf(os.Stderr, "DEBUG: Checking terminator for all functions\n")
	for _, fn := range g.module.Funcs {
		for _, block := range fn.Blocks {
			if block.Term == nil {
				if fn.Sig.RetType.Equal(types.Void) {
					block.NewRet(nil)
				} else {
					block.NewRet(constant.NewInt(types.I32, 0))
				}
			}
		}
	}

	fmt.Fprintf(os.Stderr, "DEBUG: Total functions in module: %d\n", len(g.module.Funcs))
	for _, fn := range g.module.Funcs {
		fmt.Fprintf(os.Stderr, "DEBUG: Function %s\n", fn.Name())
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
		if g.context != nil && g.context.currentFunction != nil {
			fmt.Fprintf(os.Stderr, "Current function: %s\n", g.context.currentFunction.Name())
		}
	}
}

// generateExpression generates code for an expression
func (g *Generator) generateExpression(expr ast.Expression) (value.Value, error) {
	g.debugExpression(expr)
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
		return constant.NewInt(types.I32, 0), nil
	default:
		return nil, fmt.Errorf("unknown expression type: %T", expr)
	}
}

func (g *Generator) generatePrintStatement(stmt *ast.PrintStatement) (value.Value, error) {
	// Get current function and block
	fn := g.context.currentFunction
	current := fn.Blocks[len(fn.Blocks)-1]

	// 1) Evaluate the expression (load variables, handle call-exprs)
	rawVal, err := g.generateExpression(stmt.Value)
	if err != nil {
		return nil, fmt.Errorf("generatePrintStatement: %w", err)
	}

	var val value.Value
	// 2) If rawVal is a function or function pointer, bitcast to a ptr-to-func and call
	switch rv := rawVal.(type) {
	case *ir.Func:
		// raw function definition
		ptrTy := types.NewPointer(rv.Sig)
		callee := current.NewBitCast(rv, ptrTy)
		val = current.NewCall(callee)
	default:
		// not a raw *ir.Func, check its type
		switch t := rawVal.Type().(type) {
		case *types.PointerType:
			if _, ok := t.ElemType.(*types.FuncType); ok {
				val = current.NewCall(rawVal)
				break
			}
		case *types.FuncType:
			// function value without ptr
			ptrTy := types.NewPointer(t)
			callee := current.NewBitCast(rawVal, ptrTy)
			val = current.NewCall(callee)
		default:
			// normal value (int, ptr to i8, etc.)
			val = rawVal
			// normal value (int, ptr to i8, etc.)
			val = rawVal
		}
	}

	// 3) Format 'val' based on its LLVM type
	var fmtStr string
	var arg value.Value
	switch t := val.Type().(type) {
	case *types.IntType:
		if t.Equal(types.I1) {
			fmtStr, arg = "%d", current.NewZExt(val, types.I32)
		} else {
			fmtStr, arg = "%d", val
		}
	case *types.FloatType:
		// float → extend to double for printf
		fmtStr, arg = "%f", current.NewFPExt(val, types.Double)
	case *types.PointerType:
		if t.ElemType.Equal(types.I8) {
			fmtStr, arg = "%s", val
		} else {
			return nil, fmt.Errorf("generatePrintStatement: unsupported ptr type %s", t)
		}
	default:
		return nil, fmt.Errorf("generatePrintStatement: unsupported type %T", t)
	}

	// 4) Prepare printf callee (bitcast to ptr-to-func) and emit printf call
	printfPtrTy := types.NewPointer(g.printfFunc.Sig)
	printfCallee := constant.NewBitCast(g.printfFunc, printfPtrTy)
	formatConstant := g.getStringConstant(fmtStr)
	return current.NewCall(printfCallee, formatConstant, arg), nil
}

func (g *Generator) generateAssignmentExpression(expr *ast.AssignmentExpression) (value.Value, error) {
	val, err := g.generateExpression(expr.Value)
	if err != nil {
		return nil, err
	}

	varVal, ok := g.context.Lookup(expr.Name.Value)
	if !ok {
		return nil, fmt.Errorf("undefined variable for assignment: %s", expr.Name.Value)
	}

	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]
	currentBlock.NewStore(val, varVal)
	return val, nil
}

func (g *Generator) generateVarStatement(stmt *ast.VarStatement) (value.Value, error) {
	if g.context.currentFunction == nil {
		var initFunc *ir.Func
		var initBlock *ir.Block

		for _, fn := range g.module.Funcs {
			if fn.Name() == "global_init" {
				initFunc = fn
				break
			}
		}

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

		val, err := g.generateExpression(stmt.Value)
		if err != nil {
			return nil, err
		}

		if funcVal, ok := val.(*ir.Func); ok {
			g.context.namedValues[stmt.Name.Value] = funcVal
			if funcVal.Name() != stmt.Name.Value && len(funcVal.Blocks) == 0 {
				funcVal.SetName(stmt.Name.Value)
			}
			return funcVal, nil
		} else {
			global := g.module.NewGlobal(stmt.Name.Value, val.Type())
			global.Init = constant.NewZeroInitializer(val.Type())
			initBlock.NewStore(val, global)
			g.context.namedValues[stmt.Name.Value] = global
			return global, nil
		}
	}

	val, err := g.generateExpression(stmt.Value)
	if err != nil {
		return nil, err
	}

	entryBlock := g.context.currentFunction.Blocks[0]
	allocaInst := entryBlock.NewAlloca(val.Type())
	if len(entryBlock.Insts) > 1 {
		entryBlock.Insts = append([]ir.Instruction{allocaInst}, entryBlock.Insts[:len(entryBlock.Insts)-1]...)
	}

	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]
	currentBlock.NewStore(val, allocaInst)
	g.context.namedValues[stmt.Name.Value] = allocaInst
	return allocaInst, nil
}

func (g *Generator) generateReturnStatement(stmt *ast.ReturnStatement) (value.Value, error) {
	val, err := g.generateExpression(stmt.ReturnValue)
	if err != nil {
		return nil, err
	}

	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]
	currentBlock.NewRet(val)
	return val, nil
}

func (g *Generator) generateBlockStatement(stmt *ast.BlockStatement) (value.Value, error) {
	oldContext := g.context
	newContext := newContext(oldContext)
	newContext.currentFunction = oldContext.currentFunction
	g.context = newContext

	if g.context.currentFunction == nil {
		fmt.Fprintf(os.Stderr, "WARNING: No current function when generating block statement\n")
	}

	var lastVal value.Value
	for i, s := range stmt.Statements {
		// Check if the statement is a return statement
		if retStmt, ok := s.(*ast.ReturnStatement); ok {
			val, err := g.generateReturnStatement(retStmt)
			g.context = oldContext
			return val, err
		}
		val, err := g.generateStatement(s)
		if err != nil {
			g.context = oldContext
			return nil, fmt.Errorf("error in block statement %d: %w", i, err)
		}
		lastVal = val
	}

	g.context = oldContext
	return lastVal, nil
}

func (g *Generator) generateWhileStatement(stmt *ast.WhileStatement) (value.Value, error) {
	fn := g.context.currentFunction
	currentBlock := fn.Blocks[len(fn.Blocks)-1]

	condBlock := fn.NewBlock(fmt.Sprintf("while.cond.%d", g.blockCounter))
	g.blockCounter++
	bodyBlock := fn.NewBlock(fmt.Sprintf("while.body.%d", g.blockCounter))
	g.blockCounter++
	endBlock := fn.NewBlock(fmt.Sprintf("while.end.%d", g.blockCounter))
	g.blockCounter++

	currentBlock.NewBr(condBlock)

	condVal, err := g.generateExpression(stmt.Condition)
	if err != nil {
		return nil, err
	}

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

	condBlock.NewCondBr(condBool, bodyBlock, endBlock)
	g.generateStatement(stmt.Body)

	if len(bodyBlock.Insts) == 0 || bodyBlock.Term == nil {
		bodyBlock.NewBr(condBlock)
	}

	return nil, nil
}

func (g *Generator) generateIfStatement(stmt *ast.IfStatement) (value.Value, error) {
	if g.context.currentFunction == nil {
		fmt.Fprintf(os.Stderr, "ERROR: No current function when generating if statement\n")
		return constant.NewInt(types.I32, 0), nil
	}

	fn := g.context.currentFunction

	condVal, err := g.generateExpression(stmt.Condition)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to generate condition for if statement: %v\n", err)
		return nil, err
	}

	if len(fn.Blocks) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: Function has no blocks when generating if statement\n")
		entryBlock := fn.NewBlock("entry")
		g.context.blocks["entry"] = entryBlock
	}

	currentBlock := fn.Blocks[len(fn.Blocks)-1]

	thenBlock := fn.NewBlock(fmt.Sprintf("if.then.%d", g.blockCounter))
	g.blockCounter++
	mergeBlock := fn.NewBlock(fmt.Sprintf("if.merge.%d", g.blockCounter))
	g.blockCounter++

	var elseBlock *ir.Block
	if stmt.Alternative != nil {
		elseBlock = fn.NewBlock(fmt.Sprintf("if.else.%d", g.blockCounter))
		g.blockCounter++
	} else {
		elseBlock = mergeBlock
	}

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

	g.context.currentFunction = fn
	_, err = g.generateStatement(stmt.Consequence)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to generate then block: %v\n", err)
		return nil, err
	}

	if len(thenBlock.Insts) == 0 || thenBlock.Term == nil {
		thenBlock.NewBr(mergeBlock)
	}

	if stmt.Alternative != nil {
		g.context.currentFunction = fn
		_, err = g.generateStatement(stmt.Alternative)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to generate else block: %v\n", err)
			return nil, err
		}

		if len(elseBlock.Insts) == 0 || elseBlock.Term == nil {
			elseBlock.NewBr(mergeBlock)
		}
	}

	return nil, nil
}

func (g *Generator) generatePrefixExpression(expr *ast.PrefixExpression) (value.Value, error) {
	right, err := g.generateExpression(expr.Right)
	if err != nil {
		return nil, err
	}

	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

	switch expr.Operator {
	case "!":
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
		intType, ok := right.Type().(*types.IntType)
		if !ok {
			return nil, fmt.Errorf("expected int type for - operator, got %T", right.Type())
		}
		return currentBlock.NewSub(constant.NewInt(intType, 0), right), nil
	default:
		return nil, fmt.Errorf("unknown prefix operator: %s", expr.Operator)
	}
}

func (g *Generator) getStringConstant(str string) value.Value {
	if global, ok := g.context.stringConstants[str]; ok {
		zero := constant.NewInt(types.I64, 0)
		strType := types.NewArray(uint64(len(str)+1), types.I8)
		return constant.NewGetElementPtr(strType, global, zero, zero)
	}

	processedStr := strings.ReplaceAll(str, "\\n", "\n")
	strType := types.NewArray(uint64(len(processedStr)+1), types.I8)
	strConst := g.module.NewGlobalDef(fmt.Sprintf(".str.%d", g.stringCounter),
		constant.NewCharArrayFromString(processedStr+"\x00"))
	g.stringCounter++

	zero := constant.NewInt(types.I64, 0)
	strPtr := constant.NewGetElementPtr(strType, strConst, zero, zero)
	g.context.stringConstants[str] = strConst
	return strPtr
}

func (g *Generator) generateIdentifier(expr *ast.Identifier) (value.Value, error) {
	if val, ok := g.context.Lookup(expr.Value); ok {
		if val == nil {
			fmt.Fprintf(os.Stderr, "WARNING: Variable %s found in context but has nil value\n", expr.Value)
			return constant.NewInt(types.I32, 0), nil
		}

		if g.context.currentFunction == nil || len(g.context.currentFunction.Blocks) == 0 {
			fmt.Fprintf(os.Stderr, "WARNING: No current function or blocks when generating identifier\n")
			return constant.NewInt(types.I32, 0), nil
		}
		currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

		if pointerType, ok := val.Type().(*types.PointerType); ok {
			if pointerType.ElemType == nil {
				fmt.Fprintf(os.Stderr, "WARNING: Pointer type has nil element type for variable: %s\n", expr.Value)
				return constant.NewInt(types.I32, 0), nil
			}
			if _, ok := pointerType.ElemType.(*types.FuncType); ok {
				return val, nil
			}
			if pointerType.ElemType.Equal(types.I32) ||
				pointerType.ElemType.Equal(types.I64) ||
				pointerType.ElemType.Equal(types.I8) ||
				(pointerType.ElemType.String() == "*i8") {
				return currentBlock.NewLoad(pointerType.ElemType, val), nil
			}
		}

		if fn, ok := val.(*ir.Func); ok {
			return fn, nil
		}

		return val, nil
	}

	for _, fn := range g.module.Funcs {
		if fn.Name() == expr.Value {
			return fn, nil
		}
	}

	fmt.Fprintf(os.Stderr, "WARNING: Using a default value (0) for undefined variable: %s\n", expr.Value)
	return constant.NewInt(types.I32, 0), nil
}

func (g *Generator) generateIntegerLiteral(expr *ast.IntegerLiteral) (value.Value, error) {
	return constant.NewInt(types.I32, int64(expr.Value)), nil
}

func (g *Generator) generateStringLiteral(expr *ast.StringLiteral) (value.Value, error) {
	return g.getStringConstant(expr.Value), nil
}

func (g *Generator) generateBooleanLiteral(expr *ast.BooleanLiteral) (value.Value, error) {
	if expr.Value {
		return constant.NewInt(types.I1, 1), nil
	}
	return constant.NewInt(types.I1, 0), nil
}

func (g *Generator) generateInfixExpression(expr *ast.InfixExpression) (value.Value, error) {
	left, err := g.generateExpression(expr.Left)
	if err != nil {
		return nil, err
	}

	if left == nil {
		fmt.Fprintf(os.Stderr, "WARNING: Left expression evaluated to nil in infix expression\n")
		return constant.NewInt(types.I32, 0), nil
	}

	right, err := g.generateExpression(expr.Right)
	if err != nil {
		return nil, err
	}

	if right == nil {
		fmt.Fprintf(os.Stderr, "WARNING: Right expression evaluated to nil in infix expression\n")
		return constant.NewInt(types.I32, 0), nil
	}

	if g.context.currentFunction == nil || len(g.context.currentFunction.Blocks) == 0 {
		fmt.Fprintf(os.Stderr, "WARNING: No current function or blocks when generating infix expression\n")
		return constant.NewInt(types.I32, 0), nil
	}

	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

	// Ensure both operands are integers
	leftInt := left
	rightInt := right

	if !left.Type().Equal(types.I32) {
		if left.Type().Equal(types.I1) {
			leftInt = currentBlock.NewZExt(left, types.I32)
		} else {
			return nil, fmt.Errorf("unsupported type for left operand: %s", left.Type())
		}
	}

	if !right.Type().Equal(types.I32) {
		if right.Type().Equal(types.I1) {
			rightInt = currentBlock.NewZExt(right, types.I32)
		} else {
			return nil, fmt.Errorf("unsupported type for right operand: %s", right.Type())
		}
	}

	switch expr.Operator {
	case "+":
		return currentBlock.NewAdd(leftInt, rightInt), nil
	case "-":
		return currentBlock.NewSub(leftInt, rightInt), nil
	case "*":
		return currentBlock.NewMul(leftInt, rightInt), nil
	case "/":
		return currentBlock.NewSDiv(leftInt, rightInt), nil
	case "%":
		return currentBlock.NewSRem(leftInt, rightInt), nil
	case "<":
		return currentBlock.NewICmp(enum.IPredSLT, leftInt, rightInt), nil
	case ">":
		return currentBlock.NewICmp(enum.IPredSGT, leftInt, rightInt), nil
	case "<=":
		return currentBlock.NewICmp(enum.IPredSLE, leftInt, rightInt), nil
	case ">=":
		return currentBlock.NewICmp(enum.IPredSGE, leftInt, rightInt), nil
	case "==":
		return currentBlock.NewICmp(enum.IPredEQ, leftInt, rightInt), nil
	case "!=":
		return currentBlock.NewICmp(enum.IPredNE, leftInt, rightInt), nil
	default:
		return nil, fmt.Errorf("unknown infix operator: %s", expr.Operator)
	}
}

// generateFunctionLiteral creates a new LLVM function with return type inferred from its AST body.
func (g *Generator) generateFunctionLiteral(expr *ast.FunctionLiteral) (value.Value, error) {
	oldContext := g.context
	newContext := newContext(oldContext)
	// Copy named values so nested generation can see globals
	for name, val := range oldContext.namedValues {
		newContext.namedValues[name] = val
	}
	g.context = newContext

	// Determine return type by scanning the body for the first ReturnStatement
	var retType types.Type = types.I32
	for _, stmt := range expr.Body.Statements {
		if retStmt, ok := stmt.(*ast.ReturnStatement); ok {
			switch retStmt.ReturnValue.(type) {
			case *ast.StringLiteral:
				retType = types.NewPointer(types.I8)
			case *ast.BooleanLiteral:
				retType = types.I1
			default:
				retType = types.I32
			}
			break
		}
	}

	// Build parameter types (all i32)
	paramTypes := make([]types.Type, len(expr.Parameters))
	for i := range paramTypes {
		paramTypes[i] = types.I32
	}

	// Create the LLVM function with the inferred return type
	funcType := types.NewFunc(retType, paramTypes...)
	funcName := expr.Name
	if funcName == "" {
		funcName = fmt.Sprintf("anon.%d", g.blockCounter)
		g.blockCounter++
	}
	fn := g.module.NewFunc(funcName, funcType)
	newContext.namedValues[funcName] = fn
	g.context.currentFunction = fn

	// Entry block and parameter allocas
	entry := fn.NewBlock("entry")
	g.context.blocks["entry"] = entry
	for i, p := range fn.Params {
		if i < len(expr.Parameters) {
			p.SetName(expr.Parameters[i].Value)
		}
		// Allocate space and store argument
		alloca := entry.NewAlloca(p.Typ)
		alloca.SetName(p.Name() + ".addr")
		entry.NewStore(p, alloca)
		g.context.namedValues[expr.Parameters[i].Value] = alloca
	}

	// Generate the function body and capture the last returned Value
	bodyVal, err := g.generateStatement(expr.Body)
	if err != nil {
		g.context = oldContext
		return nil, err
	}

	// Ensure a proper terminator with matching return type
	lastBlock := fn.Blocks[len(fn.Blocks)-1]
	if lastBlock.Term == nil {
		switch retType {
		case types.I1:
			// boolean → extend to i1
			boolVal := bodyVal
			if !bodyVal.Type().Equal(types.I1) {
				boolVal = lastBlock.NewICmp(enum.IPredNE, bodyVal, constant.NewInt(types.I32, 0))
			}
			lastBlock.NewRet(boolVal)
		case types.NewPointer(types.I8):
			// string pointer
			ptrVal := bodyVal
			// no coercion needed
			lastBlock.NewRet(ptrVal)
		default:
			// integer (i32)
			intVal := bodyVal
			if !bodyVal.Type().Equal(types.I32) {
				// if bool, extend; else default to 0
				if bodyVal.Type().Equal(types.I1) {
					intVal = lastBlock.NewZExt(bodyVal, types.I32)
				} else {
					intVal = constant.NewInt(types.I32, 0)
				}
			}
			lastBlock.NewRet(intVal)
		}
	}

	g.context = oldContext
	return fn, nil
}

func (g *Generator) generateCallExpression(expr *ast.CallExpression) (value.Value, error) {
	function, err := g.generateExpression(expr.Function)
	if err != nil {
		return nil, err
	}

	if function == nil {
		fmt.Fprintf(os.Stderr, "ERROR: Function expression evaluated to nil in call expression\n")
		return constant.NewInt(types.I32, 0), nil
	}

	if g.context.currentFunction == nil || len(g.context.currentFunction.Blocks) == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: No current function or blocks when generating call expression\n")
		return constant.NewInt(types.I32, 0), nil
	}
	currentBlock := g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]

	var callableFunction value.Value
	if fn, ok := function.(*ir.Func); ok {
		ptrTy := types.NewPointer(fn.Sig)
		callableFunction = constant.NewBitCast(fn, ptrTy)
	} else if funcPtr, ok := function.Type().(*types.PointerType); ok {
		if _, ok := funcPtr.ElemType.(*types.FuncType); ok {
			callableFunction = function
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: Invalid function pointer type %s for %s\n",
				funcPtr.ElemType, expr.Function.String())
			return constant.NewInt(types.I32, 0), nil
		}
	} else {
		if ident, ok := expr.Function.(*ast.Identifier); ok {
			for _, fn := range g.module.Funcs {
				if fn.Name() == ident.Value {
					callableFunction = fn
					break
				}
			}
		}
		if callableFunction == nil {
			fmt.Fprintf(os.Stderr, "ERROR: Cannot call value of type %s (identifier: %s)\n",
				function.Type(), expr.Function.String())
			return constant.NewInt(types.I32, 0), nil
		}
	}

	args := make([]value.Value, len(expr.Arguments))
	for i, arg := range expr.Arguments {
		if arg == nil {
			fmt.Fprintf(os.Stderr, "WARNING: Nil argument at position %d in function call\n", i)
			args[i] = constant.NewInt(types.I32, 0)
			continue
		}

		argVal, err := g.generateExpression(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to generate argument %d: %v\n", i, err)
			return nil, err
		}

		if argVal == nil {
			fmt.Fprintf(os.Stderr, "WARNING: Argument expression %d evaluated to nil\n", i)
			args[i] = constant.NewInt(types.I32, 0)
			continue
		}

		// Convert boolean to integer if needed
		if argVal.Type().Equal(types.I1) {
			args[i] = currentBlock.NewZExt(argVal, types.I32)
		} else if !argVal.Type().Equal(types.I32) {
			// If not a boolean or integer, use default value
			args[i] = constant.NewInt(types.I32, 0)
		} else {
			args[i] = argVal
		}
	}

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

	if callResult.Type().Equal(types.I1) {
		callResult = currentBlock.NewZExt(callResult, types.I32)
	}

	// Ensure the return value is an integer
	// if !callResult.Type().Equal(types.I32) {
	// 	if callResult.Type().Equal(types.I1) {
	// 		callResult = currentBlock.NewZExt(callResult, types.I32)
	// 	} else {
	// 		callResult = constant.NewInt(types.I32, 0)
	// 	}
	// }

	return callResult, nil
}

func (g *Generator) WriteToFile(filename string) error {
	var buf strings.Builder
	_, err := g.module.WriteTo(&buf)
	if err != nil {
		return err
	}

	ir := buf.String()
	ir = fixFunctionDeclarations(ir)
	return os.WriteFile(filename, []byte(ir), 0644)
}

func CompileToLLVM(program *ast.Program) (string, error) {
	generator := New()
	module, err := generator.Generate(program)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	_, err = module.WriteTo(&buf)
	if err != nil {
		return "", err
	}

	ir := buf.String()
	ir = fixFunctionDeclarations(ir)
	return ir, nil
}

func getStandardFunctionDeclarations() string {
	return `
; Standard function declarations
declare i32 @printf(i8*, ...)
declare i8* @malloc(i64)
declare void @free(i8*)
declare i64 @strlen(i8*)
declare i8* @strcpy(i8*, i8*)
declare i32 @abs(i32)
declare double @pow(double, double)
declare void @exit(i32)

; Standard format strings
@.fmt.int = private constant [4 x i8] c"%d\0A\00"
@.fmt.str = private constant [3 x i8] c"%s\00"
`
}

func fixFunctionDeclarations(ir string) string {
	// Dump initial IR for debugging
	fmt.Fprintf(os.Stderr, "DEBUG: Initial IR:\n%s\n", ir)

	// 1) Strip out any old declarations for printf, malloc, free, etc.
	ir = regexp.MustCompile(
		`declare\s+(?:external\s+)?(?:ccc\s+)?[^@]+@(?:printf|malloc|free|strlen|strcpy|abs|pow|exit)\(.*\)`,
	).ReplaceAllString(ir, "")

	// 2) Prepend exactly one correct set of standard declarations
	ir = getStandardFunctionDeclarations() + "\n" + ir
	fmt.Fprintf(os.Stderr, "DEBUG: IR after decl-fix:\n%s\n", ir)

	// 3) Fix all user-function calls: remove the extra `()` signature
	//    e.g. turn “call i32 () @foo()” into “call i32 @foo()”
	ir = regexp.MustCompile(
		`call\s+(\S+)\s*\(\)\s*@(\w+)\(\)`,
	).ReplaceAllString(ir, "call $1 @$2()")

	// 4) Remove any lingering mangled signature before @printf in call sites:
	//    e.g. ‘call i32 (i8*) (…) @printf(...)’ → ‘call i32 @printf’
	ir = regexp.MustCompile(
		`call\s+i32\s*\([^)]*\)\s*\([^)]*\)\s*@printf`,
	).ReplaceAllString(ir, "call i32 @printf")

	// 5) Normalize main function signature to valid IR
	ir = regexp.MustCompile(
		`define\s+[^@]+\s+@main\s*\([^)]*\)\s*(?:#[0-9]+)?\s*{`,
	).ReplaceAllString(ir, "define i32 @main() {")

	// 6) Fix other function definitions that include an empty ‘()’ in the return-type:
	//    e.g. ‘define i32 () @bar()’ → ‘define i32 @bar()’
	ir = regexp.MustCompile(
		`define\s+(\S+)\s*\(\)\s+@(\w+)\(\)`,
	).ReplaceAllString(ir, "define $1 @$2()")

	fmt.Fprintf(os.Stderr, "DEBUG: Final IR after all fixes:\n%s\n", ir)
	return ir
}

// func fixFunctionDeclarations(ir string) string {
// 	fmt.Fprintf(os.Stderr, "DEBUG: Initial IR before adding standard declarations:\n%s\n", ir)
//
// 	// Remove any existing function declarations
// 	ir = regexp.MustCompile(`declare\s+(?:external\s+)?(?:ccc\s+)?[^@]+@(?:printf|malloc|free|strlen|strcpy|abs|pow|exit)\(.*\)`).ReplaceAllString(ir, "")
//
// 	// Add standard function declarations at the top
// 	ir = getStandardFunctionDeclarations() + "\n" + ir
//
// 	fmt.Fprintf(os.Stderr, "DEBUG: IR after adding standard declarations:\n%s\n", ir)
//
// 	// Fix main function signature
// 	mainPattern := regexp.MustCompile(`define\s+[^@]+\s+@main\s*\([^)]*\)\s*(?:#[0-9]+)?\s*{`)
// 	ir = mainPattern.ReplaceAllStringFunc(ir, func(s string) string {
// 		fmt.Fprintf(os.Stderr, "DEBUG: Main function found: %s\n", s)
// 		return "define i32 @main() {"
// 	})
//
// 	ir = regexp.MustCompile(`call\s+i32\s*\([^)]*\)\s*@printf`).ReplaceAllString(ir, "call i32 @printf(i8*, ...)")
//
// 	//TEST:
// 	ir = regexp.MustCompile(`\(([^)]*)\)\s*\(\.\.\.\)`).ReplaceAllString(ir, "($1, ...)")
// 	// now catch printf specifically and insert the full variadic signature
// 	ir = regexp.MustCompile(`call\s+i32\s*\([^)]*\)\s*@printf`).ReplaceAllString(ir, "call i32 (i8*, ...) @printf")
//
// 	// Fix getelementptr instructions for printf calls
// 	ir = regexp.MustCompile(`getelementptr\s+\[(\d+)\s+x\s+i8\],\s*i8\*\s*getelementptr`).ReplaceAllString(ir, "getelementptr [$1 x i8], [$1 x i8]*")
// 	ir = regexp.MustCompile(`i8\*\s*getelementptr\(\[(\d+)\s+x\s+i8\],\s*i8\*\s*getelementptr`).ReplaceAllString(ir, "i8* getelementptr([$1 x i8], [$1 x i8]*")
//
// 	// Convert array types to pointer types in printf calls (catch the common pattern)
// 	// before our existing hack, insert something like:
// 	re := regexp.MustCompile(`getelementptr\s+\[(\d+)\s+x\s+i8\],\s*\[\d+\s+x\s+i8\]\*\s*(@\.str\.\d+),\s*i64\s+0,\s*i64\s+0`)
// 	ir = re.ReplaceAllString(ir,
// 		`getelementptr inbounds ([$1 x i8], [$1 x i8]* $2, i64 0, i64 0)`)
//
// 	badFuncPattern := regexp.MustCompile(`define\s+i32\s+\(i32\)\s+@(\w+)`)
// 	matches := badFuncPattern.FindAllString(ir, -1)
// 	for _, m := range matches {
// 		fmt.Fprintf(os.Stderr, "ERROR: Invalid function definition: %s\n", m)
// 	}
//
// 	// Fix string constant declarations
// 	ir = regexp.MustCompile(`@\.str\.[0-9]+ = .*c"([^"]*)".*`).ReplaceAllStringFunc(ir, func(s string) string {
// 		matches := regexp.MustCompile(`c"([^"]*)"`).FindStringSubmatch(s)
// 		if len(matches) > 1 {
// 			content := strings.ReplaceAll(matches[1], "\\n", "\n")
// 			content = strings.ReplaceAll(content, "\\00", "\x00")
// 			return strings.Replace(s, matches[1], content, 1)
// 		}
// 		return s
// 	})
//
// 	//INFO: defines the main function correctly
// 	ir = regexp.MustCompile(`define\s+i32\s+\(i32\)\s+@(\w+)\s*\(\)\s*\{`).
// 		ReplaceAllString(ir, "define i32 @$1(i32 %n) {")
// 	ir = regexp.MustCompile(`declare\s+i8\*\s+\(i64\)\s+@malloc\(\)`).ReplaceAllString(ir, "declare i8* @malloc(i64)")
// 	ir = regexp.MustCompile(`declare\s+void\s+\(i8\*\)\s+@free\(\)`).ReplaceAllString(ir, "declare void @free(i8*)")
// 	ir = regexp.MustCompile(`declare\s+i64\s+\(i8\*\)\s+@strlen\(\)`).ReplaceAllString(ir, "declare i64 @strlen(i8*)")
// 	ir = regexp.MustCompile(`declare\s+i8\*\s+\(i8\*,\s*i8\*\)\s+@strcpy\(\)`).ReplaceAllString(ir, "declare i8* @strcpy(i8*, i8*)")
// 	ir = regexp.MustCompile(`declare\s+i32\s+\(i32\)\s+@abs\(\)`).ReplaceAllString(ir, "declare i32 @abs(i32)")
// 	ir = regexp.MustCompile(`declare\s+double\s+\(double,\s*double\)\s+@pow\(\)`).ReplaceAllString(ir, "declare double @pow(double, double)")
// 	ir = regexp.MustCompile(`declare\s+void\s+\(i32\)\s+@exit\(\)`).ReplaceAllString(ir, "declare void @exit(i32)")
//
// 	fmt.Fprintf(os.Stderr, "DEBUG: Final IR after all fixes:\n%s\n", ir)
//
// 	// Final verification
// 	if !strings.Contains(ir, "define i32 @main()") {
// 		fmt.Fprintf(os.Stderr, "ERROR: Main function could not be generated\n")
// 	} else {
// 		fmt.Fprintf(os.Stderr, "DEBUG: Main function successfully generated\n")
// 	}
//
// 	return ir
// }
