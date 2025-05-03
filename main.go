package main

import (
	"compiler/ast"
	"compiler/codegen"
	"compiler/lexer"
	"compiler/parser"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	// Parse command line arguments
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: compiler <source_file>")
		os.Exit(1)
	}
	sourceFile := args[0]
	outputFile := strings.TrimSuffix(filepath.Base(sourceFile), filepath.Ext(sourceFile))

	// Read the source file
	source, err := ioutil.ReadFile(sourceFile)
	if err != nil {
		fmt.Printf("Error reading source file: %v\n", err)
		os.Exit(1)
	}

	// Compile the source code
	llvmIR, err := compile(string(source))
	if err != nil {
		fmt.Printf("Compilation error: %v\n", err)
		os.Exit(1)
	}

	// Write the LLVM IR to a file
	irFile := outputFile + ".ll"
	err = ioutil.WriteFile(irFile, []byte(llvmIR), 0644)
	if err != nil {
		fmt.Printf("Error writing LLVM IR file: %v\n", err)
		os.Exit(1)
	}

	// Compile the LLVM IR to an executable
	err = compileIRToExecutable(irFile, outputFile)
	if err != nil {
		fmt.Printf("Error compiling LLVM IR to executable: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully compiled to %s\n", outputFile)
}

// compile compiles source code to LLVM IR
// compile compiles source code to LLVM IR
func compile(source string) (string, error) {
	// Lexical analysis
	tokens, err := lexer.TokenizeFile(source)
	if err != nil {
		return "", fmt.Errorf("lexer error: %v", err)
	}

	fmt.Println("==== Tokens ====")
	for i, token := range tokens {
		fmt.Printf("%d: Type=%s, Literal=%q, Line=%d, Column=%d\n",
			i, token.Type, token.Literal, token.Line, token.Column)
	}
	fmt.Println("================")

	// Syntax analysis
	p := parser.NewWithTokens(tokens)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		return "", fmt.Errorf("parser errors: %v", p.Errors())
	}

	// Optional: Print the AST for debugging
	if os.Getenv("DEBUG") == "1" {
		ast.PrintAST(program, "")
	}

	// Code generation
	llvmIR, err := codegen.CompileToLLVM(program)
	if err != nil {
		return "", fmt.Errorf("code generation error: %v", err)
	}

	return llvmIR, nil
}

// compileIRToExecutable compiles LLVM IR to an executable
func compileIRToExecutable(irFile, outputFile string) error {
	// Use clang to compile the LLVM IR to an executable
	cmd := exec.Command("clang", irFile, "-o", outputFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("clang error: %v\n%s", err, output)
	}

	return nil
}
