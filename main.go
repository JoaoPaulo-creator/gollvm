package main

import (
	"compiler/ast"
	"compiler/codegen"
	"compiler/lexer"
	"compiler/parser"
	"flag"
	"fmt"
	// "io/ioutil"
	"os"
	"os/exec"
	// "path/filepath"
	// "strings"
)

func main() {
	// Parse command line arguments
	inputFile := flag.String("input", "", "Input file")
	outputFile := flag.String("output", "output.ll", "Output file")
	runOutput := flag.Bool("run", false, "Run the generated LLVM IR with lli")
	flag.Parse()

	// Check if input file is provided
	if *inputFile == "" {
		fmt.Fprintf(os.Stderr, "Error: No input file specified\n")
		flag.Usage()
		os.Exit(1)
	}

	// Read input file
	input, err := os.ReadFile(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
		os.Exit(1)
	}

	// Parse input
	l := lexer.New(string(input))
	p := parser.New(l)
	program := p.ParseProgram()

	// Check for parser errors
	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "Parser error: %s\n", err)
		}
		os.Exit(1)
	}

	// Generate LLVM IR
	ir, err := codegen.CompileToLLVM(program)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Code generation error: %v\n", err)
		os.Exit(1)
	}

	// Write output to file
	err = os.WriteFile(*outputFile, []byte(ir), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully compiled %s to %s\n", *inputFile, *outputFile)

	// Optionally run the generated LLVM IR
	if *runOutput {
		fmt.Println("Running the compiled program:")
		cmd := exec.Command("lli", *outputFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error running compiled program: %v\n", err)
			os.Exit(1)
		}
	}
}

// compile compiles source code to LLVM IR
// compile compiles source code to LLVM IR
func compile(source string) (string, error) {
	// Lexical analysis
	tokens, err := lexer.TokenizeFile(source)
	if err != nil {
		return "", fmt.Errorf("lexer error: %v", err)
	}

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
