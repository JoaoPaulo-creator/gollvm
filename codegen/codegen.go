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
	"os/exec"
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

func fixPrintfCalls(ir string) string {
	lines := strings.Split(ir, "\n")
	result := make([]string, 0, len(lines))

	re := regexp.MustCompile(`(call\s+i32\s+)\([^\)]*\)\s*\(\.\.\.\)\s*(@printf\()([^\)]*\))`)

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "%") && strings.Contains(trimmedLine, "= call") && strings.Contains(trimmedLine, "@printf") {
			fixedLine := re.ReplaceAllString(line, `$1(i8*, ...) $2$3`)
			if fixedLine != line {
				fmt.Fprintf(os.Stderr, "Fixed printf call: %s\n", fixedLine)
			}
			result = append(result, fixedLine)
			continue
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

func declarePrintf(module *ir.Module) *ir.Func {
	paramTypes := []types.Type{types.NewPointer(types.I8)}
	printfType := types.NewFunc(types.I32, paramTypes...)
	fn := module.NewFunc("printf", printfType)
	fn.Sig.Variadic = true
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
			fn.Params[i].SetName(fmt.Sprintf("param%d", i))
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
	// Track defined functions to prevent duplicates
	definedFunctions := make(map[string]*ir.Func)

	// Create main function first
	mainFunc := g.module.NewFunc("main", types.I32)
	mainBlock := mainFunc.NewBlock("entry")
	g.context.currentFunction = mainFunc
	g.context.blocks["entry"] = mainBlock
	fmt.Fprintf(os.Stderr, "Generate: Created main function, currentFunction: %v\n", g.context.currentFunction.Name())

	// Process function declarations first
	for i, stmt := range program.Statements {
		if varStmt, ok := stmt.(*ast.VarStatement); ok {
			if funcLit, ok := varStmt.Value.(*ast.FunctionLiteral); ok {
				// Check if function is already defined
				if _, exists := definedFunctions[funcLit.Name]; exists {
					fmt.Fprintf(os.Stderr, "Generate: Skipping duplicate function %s\n", funcLit.Name)
					continue // Skip duplicate function definition
				}
				// Temporarily clear currentFunction for function declarations
				oldFunc := g.context.currentFunction
				g.context.currentFunction = nil
				fmt.Fprintf(os.Stderr, "Generate: Processing function declaration %s, currentFunction: nil\n", funcLit.Name)
				val, err := g.generateStatement(stmt)
				if err != nil {
					return nil, fmt.Errorf("error processing function declaration %d: %w", i, err)
				}
				g.context.currentFunction = oldFunc
				fmt.Fprintf(os.Stderr, "Generate: Restored currentFunction: %v\n", g.context.currentFunction.Name())
				if fn, ok := val.(*ir.Func); ok {
					definedFunctions[funcLit.Name] = fn
				}
			}
		} else if funcStmt, ok := stmt.(*ast.FunctionStatement); ok {
			// Check if function is already defined
			if _, exists := definedFunctions[funcStmt.Name.Value]; exists {
				fmt.Fprintf(os.Stderr, "Generate: Skipping duplicate function %s\n", funcStmt.Name.Value)
				continue // Skip duplicate function definition
			}
			// Temporarily clear currentFunction for function declarations
			oldFunc := g.context.currentFunction
			g.context.currentFunction = nil
			fmt.Fprintf(os.Stderr, "Generate: Processing function declaration %s, currentFunction: nil\n", funcStmt.Name.Value)
			val, err := g.generateStatement(stmt)
			if err != nil {
				return nil, fmt.Errorf("error processing function declaration %d: %w", i, err)
			}
			g.context.currentFunction = oldFunc
			fmt.Fprintf(os.Stderr, "Generate: Restored currentFunction: %v\n", g.context.currentFunction.Name())
			if fn, ok := val.(*ir.Func); ok {
				definedFunctions[funcStmt.Name.Value] = fn
			}
		}
	}

	// Process non-function statements in main
	for i, stmt := range program.Statements {
		// Skip function declarations
		if varStmt, ok := stmt.(*ast.VarStatement); ok {
			if _, ok := varStmt.Value.(*ast.FunctionLiteral); ok {
				fmt.Fprintf(os.Stderr, "Generate: Skipping function variable %s in main processing\n", varStmt.Name.Value)
				continue
			}
		}
		if _, ok := stmt.(*ast.FunctionStatement); ok {
			fmt.Fprintf(os.Stderr, "Generate: Skipping function statement in main processing\n")
			continue
		}

		fmt.Fprintf(os.Stderr, "Generate: Processing statement %d, currentFunction: %v\n", i, g.context.currentFunction.Name())
		_, err := g.generateStatement(stmt)
		if err != nil {
			return nil, fmt.Errorf("error processing statement %d: %w", i, err)
		}
	}

	// Add a terminator to the main block if needed
	if mainBlock.Term == nil {
		mainBlock.NewRet(constant.NewInt(types.I32, 0))
	}

	// Ensure all blocks have terminators
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

	return g.module, nil
}

// generateStatement generates code for a statement
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
	// Get current function
	fn := g.context.currentFunction
	if fn == nil {
		return nil, fmt.Errorf("no current function for for statement")
	}

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

	// Add branch to update block if no terminator
	if bodyBlock.Term == nil {
		bodyBlock.NewBr(updateBlock)
	}

	// Generate update
	if stmt.Update != nil {
		_, err := g.generateStatement(stmt.Update)
		if err != nil {
			return nil, err
		}
	}

	// Add branch to condition block if no terminator
	if updateBlock.Term == nil {
		updateBlock.NewBr(condBlock)
	}

	return nil, nil
}

func (g *Generator) generateImportStatement(stmt *ast.ImportStatement) (value.Value, error) {
	// Handle import statements
	fmt.Printf("Importing module: %s\n", stmt.Path.Value)
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
		return currentBlock.NewTrunc(val, expectedType)
	case types.IsPointer(val.Type()) && types.IsInt(expectedType):
		return currentBlock.NewPtrToInt(val, expectedType)
	case types.IsInt(val.Type()) && types.IsPointer(expectedType):
		return currentBlock.NewIntToPtr(val, expectedType)
	default:
		fmt.Fprintf(os.Stderr, "WARNING: Unsupported type conversion from %v to %v, returning original value\n",
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

// CompileToLLVM generates LLVM IR from an AST program
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

	// Apply fixes
	ir = fixPrintfDeclaration(ir)
	ir = fixExternalFunctionDeclarations(ir)
	ir = fixFunctionDeclarations(ir)
	ir = fixFunctionPointerUsage(ir)
	ir = fixFunctionTypeUsage(ir)
	ir = removeDuplicateFunctionDefinitions(ir)

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

// CompileToExecutable compiles the AST to a native executable
func (g *Generator) CompileToExecutable(program *ast.Program, outputFile string) error {
	module, err := g.Generate(program)
	if err != nil {
		return fmt.Errorf("failed to generate LLVM IR: %w", err)
	}

	irFile := "temp.ll"
	var buf strings.Builder
	_, err = module.WriteTo(&buf)
	if err != nil {
		return fmt.Errorf("failed to write LLVM IR: %w", err)
	}

	ir := buf.String()
	ir = fixPrintfDeclaration(ir)
	ir = fixExternalFunctionDeclarations(ir)
	ir = fixFunctionDeclarations(ir)
	ir = fixFunctionPointerUsage(ir)
	ir = fixFunctionTypeUsage(ir)
	ir = removeDuplicateFunctionDefinitions(ir)
	ir = fixPrintfCalls(ir) // Add the new fix here

	// Add standard format strings
	formatStrings := `
; Standard format strings
@.fmt.int = private constant [4 x i8] c"%d\0A\00"
@.fmt.str = private constant [4 x i8] c"%s\0A\00"
@.fmt.float = private constant [4 x i8] c"%f\0A\00"
@.fmt.bool = private constant [4 x i8] c"%d\0A\00"
`
	ir = ir + "\n" + formatStrings

	err = os.WriteFile(irFile, []byte(ir), 0644)
	if err != nil {
		return fmt.Errorf("failed to write IR to %s: %w", irFile, err)
	}
	fmt.Fprintf(os.Stderr, "Generated LLVM IR: %s\n", irFile)

	ir = ir + "\n" + formatStrings

	err = os.WriteFile(irFile, []byte(ir), 0644)
	if err != nil {
		return fmt.Errorf("failed to write IR to %s: %w", irFile, err)
	}
	fmt.Fprintf(os.Stderr, "Generated LLVM IR: %s\n", irFile)

	// Step 2: Compile IR to object file
	objFile := "temp.o"
	cmd := exec.Command("llc", "-filetype=obj", irFile, "-o", objFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to compile IR to object file: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Generated object file: %s\n", objFile)

	// Step 3: Link object file to executable
	cmd = exec.Command("clang", objFile, "-o", outputFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to link object file to executable: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Generated executable: %s\n", outputFile)

	// Clean up temporary files
	os.Remove(irFile)
	os.Remove(objFile)

	return nil
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

	// Use the main function's entry block if outside a function
	currentBlock := g.getCurrentBlock()
	currentBlock.NewStore(val, varVal)

	return val, nil
}

// getCurrentBlock retrieves the current block, defaulting to main's entry block if necessary
func (g *Generator) getCurrentBlock() *ir.Block {
	if g.context.currentFunction != nil && len(g.context.currentFunction.Blocks) > 0 {
		return g.context.currentFunction.Blocks[len(g.context.currentFunction.Blocks)-1]
	}
	// Fallback to main function's entry block
	for _, fn := range g.module.Funcs {
		if fn.Name() == "main" && len(fn.Blocks) > 0 {
			fmt.Fprintf(os.Stderr, "getCurrentBlock: Falling back to main's entry block\n")
			return fn.Blocks[0]
		}
	}
	// Create a new block in main if none exists
	mainFunc := g.module.NewFunc("main", types.I32)
	block := mainFunc.NewBlock("entry")
	g.context.currentFunction = mainFunc
	g.context.blocks["entry"] = block
	fmt.Fprintf(os.Stderr, "getCurrentBlock: Created new main function and entry block\n")
	return block
}

// generateVarStatement generates code for a var statement
func (g *Generator) generateVarStatement(stmt *ast.VarStatement) (value.Value, error) {
	fmt.Fprintf(os.Stderr, "Generating var statement: %s, currentFunction: %v\n", stmt.Name.Value, g.context.currentFunction)

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
		// Use a pointer to the function type
		allocType = types.NewPointer(funcVal.Sig)
		fmt.Fprintf(os.Stderr, "Function detected, using function pointer type: %v\n", allocType)
	} else if ptrType, ok := val.Type().(*types.PointerType); ok {
		if funcType, isFunc := ptrType.ElemType.(*types.FuncType); isFunc {
			// For function pointers, use a pointer to the function type
			allocType = types.NewPointer(funcType)
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
		fmt.Fprintf(os.Stderr, "Handling global variable: %s\n", stmt.Name.Value)
		// Create global variable with proper initialization
		global := g.module.NewGlobal(stmt.Name.Value, allocType)

		// Try to use constant initialization if possible
		if funcVal, ok := val.(*ir.Func); ok {
			// For functions, initialize with the function pointer
			global.Init = funcVal
			fmt.Fprintf(os.Stderr, "Global %s initialized with function: %v\n", stmt.Name.Value, funcVal.Name())
		} else if constVal, ok := val.(constant.Constant); ok {
			global.Init = constVal
			fmt.Fprintf(os.Stderr, "Global %s initialized with constant: %v\n", stmt.Name.Value, constVal)
		} else {
			global.Init = constant.NewZeroInitializer(allocType)
			fmt.Fprintf(os.Stderr, "Global %s initialized with zero initializer\n", stmt.Name.Value)

			// Create or use global initializer function
			var initFunc *ir.Func
			for _, fn := range g.module.Funcs {
				if fn.Name() == "global_init" {
					initFunc = fn
					break
				}
			}
			if initFunc == nil {
				initFunc = g.module.NewFunc("global_init", types.Void)
				initBlock := initFunc.NewBlock("entry")
				initBlock.NewRet(nil)
				// Call global_init from main
				for _, fn := range g.module.Funcs {
					if fn.Name() == "main" {
						mainEntry := fn.Blocks[0]
						mainEntry.Insts = append([]ir.Instruction{mainEntry.NewCall(initFunc)}, mainEntry.Insts...)
						fmt.Fprintf(os.Stderr, "Added global_init call to main\n")
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
			fmt.Fprintf(os.Stderr, "Stored global %s in global_init\n", stmt.Name.Value)
		}

		g.context.namedValues[stmt.Name.Value] = global
		return global, nil
	}

	// Local variable handling
	fmt.Fprintf(os.Stderr, "Handling local variable: %s\n", stmt.Name.Value)
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
	currentBlock := g.getCurrentBlock()

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
	currentBlock := g.getCurrentBlock()

	// Ensure the return value is not a function type
	if _, isFunc := val.Type().(*types.FuncType); isFunc {
		fmt.Fprintf(os.Stderr, "WARNING: Return value is a function type, defaulting to i32 0\n")
		val = constant.NewInt(types.I32, 0)
	}

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
	if fn == nil {
		return nil, fmt.Errorf("no current function for while statement")
	}

	// Get the current block BEFORE creating new blocks
	currentBlock := g.getCurrentBlock()

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
	_, err = g.generateStatement(stmt.Body)
	if err != nil {
		return nil, err
	}

	// Add back edge to condition block if no terminator exists
	if bodyBlock.Term == nil {
		bodyBlock.NewBr(condBlock)
	}

	return nil, nil
}

// generateIfStatement generates code for an if statement
func (g *Generator) generateIfStatement(stmt *ast.IfStatement) (value.Value, error) {
	// Verify we have a current function
	if g.context.currentFunction == nil {
		fmt.Fprintf(os.Stderr, "WARNING: No current function when generating if statement, using main\n")
		g.context.currentFunction = g.module.NewFunc("main", types.I32)
		g.context.currentFunction.NewBlock("entry")
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
		return nil, fmt.Errorf("function has no blocks for if statement")
	}

	// Get current block safely
	currentBlock := g.getCurrentBlock()

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
	if thenBlock.Term == nil {
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
		if elseBlock.Term == nil {
			elseBlock.NewBr(mergeBlock)
		}
	}

	return nil, nil
}

// generatePrintStatement generates code for a print statement
func (g *Generator) generatePrintStatement(stmt *ast.PrintStatement) (value.Value, error) {
	// Get current block, falling back to main if necessary
	currentBlock := g.getCurrentBlock()
	fmt.Fprintf(os.Stderr, "Generating print statement, currentFunction: %v, block: %s\n",
		g.context.currentFunction.Name(), currentBlock.Name())

	// Generate the value to print
	val, err := g.generateExpression(stmt.Value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to generate value for print statement: %v\n", err)
		return nil, err
	}

	// Handle function call expressions explicitly
	var printVal value.Value
	if callExpr, ok := stmt.Value.(*ast.CallExpression); ok {
		// Generate the call expression to get the return value
		callResult, err := g.generateCallExpression(callExpr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: Failed to generate call expression for print: %v\n", err)
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "Call expression result type: %v\n", callResult.Type())
		if callResult.Type() == nil || callResult.Type().Equal(types.Void) {
			fmt.Fprintf(os.Stderr, "WARNING: Call expression returned void or nil type, defaulting to i32 0\n")
			printVal = constant.NewInt(types.I32, 0)
		} else if _, isFunc := callResult.Type().(*types.FuncType); isFunc {
			fmt.Fprintf(os.Stderr, "WARNING: Call expression returned function type, defaulting to i32 0\n")
			printVal = constant.NewInt(types.I32, 0)
		} else if ptrType, ok := callResult.Type().(*types.PointerType); ok && types.IsFunc(ptrType.ElemType) {
			fmt.Fprintf(os.Stderr, "WARNING: Call result is a function pointer, defaulting to i32 0\n")
			printVal = constant.NewInt(types.I32, 0)
		} else {
			printVal = callResult
		}
	} else {
		fmt.Fprintf(os.Stderr, "Non-call expression type: %v\n", val.Type())
		printVal = val
	}

	// Depending on the type, select the appropriate format string
	var formatStr string

	fmt.Fprintf(os.Stderr, "Print value type: %v\n", printVal.Type())

	switch valType := printVal.Type().(type) {
	case *types.IntType:
		formatStr = "%d\n"
	case *types.FloatType:
		formatStr = "%f\n"
	case *types.PointerType:
		if valType.ElemType != nil && valType.ElemType.Equal(types.I8) {
			formatStr = "%s\n"
		} else if _, isFunc := valType.ElemType.(*types.FuncType); isFunc {
			fmt.Fprintf(os.Stderr, "WARNING: Attempting to print function pointer type %v, converting to i32\n", valType)
			formatStr = "%d\n"
			printVal = currentBlock.NewPtrToInt(printVal, types.I32)
		} else {
			fmt.Fprintf(os.Stderr, "WARNING: Attempting to print pointer type %v, converting to i32\n", valType)
			formatStr = "%d\n"
			if !types.IsInt(printVal.Type()) {
				printVal = currentBlock.NewPtrToInt(printVal, types.I32)
			}
		}
	case *types.FuncType:
		fmt.Fprintf(os.Stderr, "WARNING: Attempting to print function type %v, defaulting to i32 0\n", valType)
		formatStr = "%d\n"
		printVal = constant.NewInt(types.I32, 0)
	default:
		fmt.Fprintf(os.Stderr, "WARNING: Unsupported type %v for print, defaulting to integer\n", printVal.Type())
		formatStr = "%d\n"
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
		return nil, fmt.Errorf("printf function not found")
	}

	// Ensure printf function has correct signature
	if !printfFn.Sig.RetType.Equal(types.I32) || len(printfFn.Sig.Params) < 1 || !printfFn.Sig.Params[0].Equal(types.NewPointer(types.I8)) {
		fmt.Fprintf(os.Stderr, "WARNING: Incorrect printf signature detected, ensuring correct declaration\n")
		// Remove existing printf declaration to avoid duplicates
		g.module.Funcs = removeFunction(g.module.Funcs, "printf")
		printfFn = declarePrintf(g.module)
		g.printfFunc = printfFn
	}

	// Cast format string constant to the expected pointer type
	formatStrPtr := currentBlock.NewBitCast(formatStrConst, types.NewPointer(types.I8))

	// Ensure printVal is compatible with printf
	var printArg value.Value
	if types.IsPointer(printVal.Type()) && printVal.Type().(*types.PointerType).ElemType.Equal(types.I8) {
		// String pointers are passed directly
		printArg = printVal
	} else if types.IsInt(printVal.Type()) || types.IsFloat(printVal.Type()) {
		// Integers and floats are passed directly
		printArg = printVal
	} else {
		// Allocate and store the value to create a pointer
		alloca := currentBlock.NewAlloca(printVal.Type())
		currentBlock.NewStore(printVal, alloca)
		printArg = alloca
	}

	// Create the printf call
	var result value.Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "PANIC in printf call: %v\n", r)
				result = constant.NewInt(types.I32, 0)
			}
		}()
		result = currentBlock.NewCall(printfFn, formatStrPtr, printArg)
	}()

	if result == nil {
		fmt.Fprintf(os.Stderr, "WARNING: Print call failed, using default value\n")
		return constant.NewInt(types.I32, 0), nil
	}

	return result, nil
}

// removeFunction removes a function with the given name from the function list
func removeFunction(funcs []*ir.Func, name string) []*ir.Func {
	var result []*ir.Func
	for _, fn := range funcs {
		if fn.Name() != name {
			result = append(result, fn)
		}
	}
	return result
}

// getStringConstant gets or creates a string constant
func (g *Generator) getStringConstant(str string) value.Value {
	if global, ok := g.context.stringConstants[str]; ok {
		return global
	}

	processedStr := str
	if !strings.HasSuffix(processedStr, "\x00") {
		processedStr = processedStr + "\x00"
	}

	arrayLength := uint64(len(processedStr))
	strType := types.NewArray(arrayLength, types.I8)
	strConst := g.module.NewGlobalDef(fmt.Sprintf(".str.%d", g.stringCounter),
		constant.NewCharArrayFromString(processedStr))
	g.stringCounter++

	zero := constant.NewInt(types.I64, 0)
	strPtr := constant.NewGetElementPtr(strType, strConst, zero, zero)

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

		// For global variables, return the pointer directly during initialization
		if g.context.currentFunction == nil {
			fmt.Fprintf(os.Stderr, "Returning global variable pointer: %s\n", expr.Value)
			return val, nil
		}

		currentBlock := g.getCurrentBlock()

		// Handle function pointer types
		if _, ok := ptr.ElemType.(*types.FuncType); ok {
			fmt.Fprintf(os.Stderr, "Function pointer detected\n")
			return currentBlock.NewLoad(ptr, val), nil
		}

		// Load regular value
		return currentBlock.NewLoad(ptr.ElemType, val), nil
	}

	return val, nil
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

	var callableFunction value.Value
	var returnType types.Type = types.I32

	fmt.Fprintf(os.Stderr, "Function type: %v\n", function.Type())

	switch fn := function.(type) {
	case *ir.Func:
		fmt.Fprintf(os.Stderr, "Direct function call: %s, signature: %v, return type: %v\n", fn.Name(), fn.Sig, fn.Sig.RetType)
		returnType = fn.Sig.RetType
		callableFunction = fn
	case value.Value:
		if ptrType, ok := fn.Type().(*types.PointerType); ok {
			if funcType, ok := ptrType.ElemType.(*types.FuncType); ok {
				fmt.Fprintf(os.Stderr, "Function pointer call, return type: %v\n", funcType.RetType)
				returnType = funcType.RetType
				callableFunction = fn
			} else {
				fmt.Fprintf(os.Stderr, "ERROR: Attempted to call a non-function pointer: %v\n", ptrType.ElemType)
				return constant.NewInt(types.I32, 0), nil
			}
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: Cannot call value of type %v\n", fn.Type())
			return constant.NewInt(types.I32, 0), nil
		}
	default:
		fmt.Fprintf(os.Stderr, "ERROR: Cannot call value of type %T\n", fn)
		return constant.NewInt(types.I32, 0), nil
	}

	// Validate return type to prevent function type issues
	if _, isFunc := returnType.(*types.FuncType); isFunc {
		fmt.Fprintf(os.Stderr, "WARNING: Return type is a function type, defaulting to i32\n")
		returnType = types.I32
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

		// Ensure argument type matches expected parameter type
		if i < len(callableFunction.(*ir.Func).Sig.Params) {
			expectedType := callableFunction.(*ir.Func).Sig.Params[i]
			if !argVal.Type().Equal(expectedType) {
				fmt.Fprintf(os.Stderr, "WARNING: Argument %d type %v does not match expected %v, attempting conversion\n", i, argVal.Type(), expectedType)
				argVal = g.ensureType(argVal, expectedType)
			}
		}

		args[i] = argVal
	}

	currentBlock := g.getCurrentBlock()

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

	fmt.Fprintf(os.Stderr, "Call result type: %v\n", callResult.Type())

	if returnType != nil && !returnType.Equal(types.Void) {
		if !callResult.Type().Equal(returnType) {
			fmt.Fprintf(os.Stderr, "WARNING: Call result type %v does not match expected return type %v, attempting conversion\n", callResult.Type(), returnType)
			callResult = g.ensureType(callResult, returnType)
		}
		return callResult, nil
	}

	return constant.NewInt(types.I32, 0), nil
}

// generateIntegerLiteral generates code for an integer literal
func (g *Generator) generateIntegerLiteral(expr *ast.IntegerLiteral) (value.Value, error) {
	fmt.Fprintf(os.Stderr, "Integer Literal: %d\n", expr.Value)
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
	right, err := g.generateExpression(expr.Right)
	if err != nil {
		return nil, err
	}

	currentBlock := g.getCurrentBlock()

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

// generateInfixExpression generates code for an infix expression
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

	currentBlock := g.getCurrentBlock()

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
			fmt.Fprintf(os.Stderr, "Reusing existing function: %s\n", expr.Name)
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
	var retType types.Type
	switch expr.ReturnType {
	case "void":
		retType = types.Void
	case "bool":
		retType = types.I1
	case "int", "":
		retType = types.I32
	default:
		fmt.Fprintf(os.Stderr, "WARNING: Unknown return type %s, defaulting to i32\n", expr.ReturnType)
		retType = types.I32
	}

	// Create parameter types
	paramTypes := make([]types.Type, len(expr.Parameters))
	for i := range paramTypes {
		paramTypes[i] = types.I32 // Default to i32 for parameters
	}

	// Create function type
	funcType := types.NewFunc(retType, paramTypes...)
	fmt.Fprintf(os.Stderr, "Function type created: %v\n", funcType)

	// Create function
	fn := g.module.NewFunc(funcName, funcType)
	fmt.Fprintf(os.Stderr, "Created function %s with signature: %v\n", funcName, fn.Sig)

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
				// Ensure bodyVal is not a function type
				if _, isFunc := bodyVal.Type().(*types.FuncType); isFunc {
					fmt.Fprintf(os.Stderr, "WARNING: Function body returned function type, defaulting to i32 0\n")
					bodyVal = constant.NewInt(types.I32, 0)
				}
				// Ensure the return value matches the expected return type
				bodyVal = g.ensureType(bodyVal, retType)
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
	var buf strings.Builder
	_, err := g.module.WriteTo(&buf)
	if err != nil {
		return err
	}

	ir := buf.String()
	fmt.Fprintf(os.Stderr, "IR before fixes:\n%s\n", ir)

	// Apply existing fixes
	ir = fixPrintfDeclaration(ir)
	ir = fixExternalFunctionDeclarations(ir)
	ir = fixFunctionDeclarations(ir)
	ir = fixFunctionPointerUsage(ir)
	ir = fixFunctionTypeUsage(ir)
	ir = removeDuplicateFunctionDefinitions(ir)
	ir = fixPrintfCalls(ir) // Add the new fix here

	fmt.Fprintf(os.Stderr, "IR after fixes:\n%s\n", ir)
	return os.WriteFile(filename, []byte(ir), 0644)
}

// fixPrintfDeclaration ensures the printf declaration is correct
func fixPrintfDeclaration(ir string) string {
	lines := strings.Split(ir, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "declare ") && strings.Contains(trimmedLine, "@printf") {
			// Replace any incorrect printf declaration with the correct one
			result = append(result, "declare i32 @printf(i8*, ...)")
			continue
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// fixExternalFunctionDeclarations fixes malformed external function declarations
func fixExternalFunctionDeclarations(ir string) string {
	lines := strings.Split(ir, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			result = append(result, line)
			continue
		}

		// Fix malformed external function declarations
		if strings.HasPrefix(trimmedLine, "declare ") {
			switch {
			case strings.Contains(trimmedLine, "@malloc"):
				result = append(result, "declare i8* @malloc(i64)")
				continue
			case strings.Contains(trimmedLine, "@free"):
				result = append(result, "declare void @free(i8*)")
				continue
			case strings.Contains(trimmedLine, "@strlen"):
				result = append(result, "declare i64 @strlen(i8*)")
				continue
			case strings.Contains(trimmedLine, "@strcpy"):
				result = append(result, "declare i8* @strcpy(i8*, i8*)")
				continue
			case strings.Contains(trimmedLine, "@abs"):
				result = append(result, "declare i32 @abs(i32)")
				continue
			case strings.Contains(trimmedLine, "@pow"):
				result = append(result, "declare double @pow(double, double)")
				continue
			case strings.Contains(trimmedLine, "@exit"):
				result = append(result, "declare void @exit(i32)")
				continue
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// fixFunctionDeclarations fixes malformed function declarations
func fixFunctionDeclarations(ir string) string {
	lines := strings.Split(ir, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			result = append(result, line)
			continue
		}

		// Fix global variable declarations with function pointers
		fixedLine := line
		if strings.HasPrefix(trimmedLine, "@") && strings.Contains(trimmedLine, "= global") {
			fixedLine = regexp.MustCompile(`i32 \(i32\) \((?:i32)?\)\*`).ReplaceAllString(fixedLine, "i32 (i32)*")
			fixedLine = strings.ReplaceAll(fixedLine, "i32 (i32) ", "i32 (i32)* ")
		}

		// Fix function definitions
		if strings.HasPrefix(trimmedLine, "define ") {
			fixedLine = regexp.MustCompile(`i32 \(i32\) \((?:i32)?\)`).ReplaceAllString(fixedLine, "i32 (i32)")
			fixedLine = strings.ReplaceAll(fixedLine, "i32 () ", "i32 ")
			fixedLine = strings.ReplaceAll(fixedLine, "i32 (i32) ", "i32 ")
			fixedLine = strings.ReplaceAll(fixedLine, "i32 (i32) ()*", "i32 (i32)")
		}

		result = append(result, fixedLine)
	}

	return strings.Join(result, "\n")
}

// fixFunctionPointerUsage fixes incorrect function pointer usage
func fixFunctionPointerUsage(ir string) string {
	lines := strings.Split(ir, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			result = append(result, line)
			continue
		}

		// Fix load instructions for function pointers
		fixedLine := line
		if strings.Contains(trimmedLine, "= load") && strings.Contains(trimmedLine, "i32 (i32)") {
			fixedLine = regexp.MustCompile(`i32 \(i32\), i32 \(i32\)\*`).ReplaceAllString(fixedLine, "i32 (i32)*, i32 (i32)**")
			fixedLine = regexp.MustCompile(`i32 \(i32\) \((?:i32)?\)\*, i32 \(i32\) \((?:i32)?\)\*\*`).ReplaceAllString(fixedLine, "i32 (i32)*, i32 (i32)**")
			fixedLine = strings.ReplaceAll(fixedLine, "i32 (i32) ()*", "i32 (i32)")
		}

		// Fix call instructions
		if strings.Contains(trimmedLine, "= call") && strings.Contains(trimmedLine, "i32 (i32)") {
			fixedLine = regexp.MustCompile(`call i32 \(i32\)`).ReplaceAllString(fixedLine, "call i32")
			fixedLine = strings.ReplaceAll(fixedLine, "i32 (i32) ()*", "i32")
		}

		result = append(result, fixedLine)
	}

	return strings.Join(result, "\n")
}

// fixFunctionTypeUsage fixes incorrect function type usage
func fixFunctionTypeUsage(ir string) string {
	lines := strings.Split(ir, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			result = append(result, line)
			continue
		}

		// Fix function types carefully to preserve valid function signatures
		fixedLine := line
		// Only replace erroneous function types in specific contexts
		if strings.Contains(trimmedLine, "= load") {
			fixedLine = regexp.MustCompile(`i32 \(i32\), i32 \(i32\)\*`).ReplaceAllString(fixedLine, "i32, i32*")
			fixedLine = strings.ReplaceAll(fixedLine, "= load i32 (i32)", "= load i32")
		}
		if strings.Contains(trimmedLine, "= alloca") {
			fixedLine = strings.ReplaceAll(fixedLine, "= alloca i32 (i32)", "= alloca i32")
		}
		if strings.Contains(trimmedLine, "= store") {
			fixedLine = regexp.MustCompile(`store i32 \(i32\)`).ReplaceAllString(fixedLine, "store i32")
		}
		if strings.Contains(trimmedLine, "= ret") {
			fixedLine = regexp.MustCompile(`ret i32 \(i32\)`).ReplaceAllString(fixedLine, "ret i32")
		}

		result = append(result, fixedLine)
	}

	return strings.Join(result, "\n")
}

// removeDuplicateFunctionDefinitions removes duplicate function definitions
func removeDuplicateFunctionDefinitions(ir string) string {
	lines := strings.Split(ir, "\n")
	seenFunctions := make(map[string]bool)
	result := []string{}
	currentFunction := ""
	functionLines := []string{}
	inFunction := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "define ") && strings.Contains(trimmedLine, "@") {
			if inFunction {
				if !seenFunctions[currentFunction] {
					result = append(result, functionLines...)
					seenFunctions[currentFunction] = true
				}
				functionLines = []string{}
			}

			matches := regexp.MustCompile(`@([a-zA-Z0-9_]+)`).FindStringSubmatch(line)
			if len(matches) > 1 {
				currentFunction = matches[1]
				inFunction = true
			}
			functionLines = append(functionLines, line)
			continue
		}

		if inFunction && strings.TrimSpace(line) == "}" {
			functionLines = append(functionLines, line)
			if !seenFunctions[currentFunction] {
				result = append(result, functionLines...)
				seenFunctions[currentFunction] = true
			}
			inFunction = false
			functionLines = []string{}
			currentFunction = ""
			continue
		}

		if inFunction {
			functionLines = append(functionLines, line)
		} else {
			result = append(result, line)
		}
	}

	if inFunction && !seenFunctions[currentFunction] {
		result = append(result, functionLines...)
	}

	return strings.Join(result, "\n")
}
