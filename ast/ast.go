package ast

import (
	"bytes"
	"compiler/lexer"
	"fmt"
	"strings"
)

// Node represents a node in the AST
type Node interface {
	TokenLiteral() string
	String() string
}

// Statement represents a statement in the program
type Statement interface {
	Node
	statementNode()
}

// Expression represents an expression in the program
type Expression interface {
	Node
	expressionNode()
}

// Program represents the entire program
type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var out bytes.Buffer

	for _, s := range p.Statements {
		out.WriteString(s.String())
	}

	return out.String()
}

// VarStatement represents a variable declaration
type VarStatement struct {
	Token lexer.Token // the VAR token
	Name  *Identifier
	Value Expression
}

func (vs *VarStatement) statementNode()       {}
func (vs *VarStatement) TokenLiteral() string { return vs.Token.Literal }
func (vs *VarStatement) String() string {
	var out bytes.Buffer

	out.WriteString(vs.TokenLiteral() + " ")
	out.WriteString(vs.Name.String())
	out.WriteString(" = ")

	if vs.Value != nil {
		out.WriteString(vs.Value.String())
	}

	out.WriteString(";")

	return out.String()
}

// ReturnStatement represents a return statement
type ReturnStatement struct {
	Token       lexer.Token // the RETURN token
	ReturnValue Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return rs.Token.Literal }
func (rs *ReturnStatement) String() string {
	var out bytes.Buffer

	out.WriteString(rs.TokenLiteral() + " ")

	if rs.ReturnValue != nil {
		out.WriteString(rs.ReturnValue.String())
	}

	out.WriteString(";")

	return out.String()
}

// ExpressionStatement represents an expression statement
type ExpressionStatement struct {
	Token      lexer.Token // the first token of the expression
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Token.Literal }
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}
	return ""
}

// BlockStatement represents a block of statements
type BlockStatement struct {
	Token      lexer.Token // the { token
	Statements []Statement
}

func (bs *BlockStatement) statementNode()       {}
func (bs *BlockStatement) TokenLiteral() string { return bs.Token.Literal }
func (bs *BlockStatement) String() string {
	var out bytes.Buffer

	for _, s := range bs.Statements {
		out.WriteString(s.String())
	}

	return out.String()
}

// IfStatement represents an if statement
type IfStatement struct {
	Token       lexer.Token // the IF token
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (is *IfStatement) statementNode()       {}
func (is *IfStatement) TokenLiteral() string { return is.Token.Literal }
func (is *IfStatement) String() string {
	var out bytes.Buffer

	out.WriteString("if")
	out.WriteString("(")
	out.WriteString(is.Condition.String())
	out.WriteString(")")
	out.WriteString(" ")
	out.WriteString(is.Consequence.String())

	if is.Alternative != nil {
		out.WriteString("else ")
		out.WriteString(is.Alternative.String())
	}

	return out.String()
}

// WhileStatement represents a while loop
type WhileStatement struct {
	Token     lexer.Token // the WHILE token
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return ws.Token.Literal }
func (ws *WhileStatement) String() string {
	var out bytes.Buffer

	out.WriteString("while")
	out.WriteString("(")
	out.WriteString(ws.Condition.String())
	out.WriteString(")")
	out.WriteString(" ")
	out.WriteString(ws.Body.String())

	return out.String()
}

// PrintStatement represents a print statement
type PrintStatement struct {
	Token lexer.Token // the PRINT token
	Value Expression
}

func (ps *PrintStatement) statementNode()       {}
func (ps *PrintStatement) TokenLiteral() string { return ps.Token.Literal }
func (ps *PrintStatement) String() string {
	var out bytes.Buffer

	out.WriteString(ps.TokenLiteral() + " ")
	out.WriteString(ps.Value.String())
	out.WriteString(";")

	return out.String()
}

// Identifier represents an identifier
type Identifier struct {
	Token lexer.Token // the IDENT token
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Token.Literal }
func (i *Identifier) String() string       { return i.Value }

// IntegerLiteral represents an integer literal
type IntegerLiteral struct {
	Token lexer.Token // the INT token
	Value int64
}

func (il *IntegerLiteral) expressionNode()      {}
func (il *IntegerLiteral) TokenLiteral() string { return il.Token.Literal }
func (il *IntegerLiteral) String() string       { return il.Token.Literal }

// StringLiteral represents a string literal
type StringLiteral struct {
	Token lexer.Token // the STRING token
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Token.Literal }
func (sl *StringLiteral) String() string       { return "\"" + sl.Value + "\"" }

// BooleanLiteral represents a boolean literal
type BooleanLiteral struct {
	Token lexer.Token // the true/false token
	Value bool
}

func (bl *BooleanLiteral) expressionNode()      {}
func (bl *BooleanLiteral) TokenLiteral() string { return bl.Token.Literal }
func (bl *BooleanLiteral) String() string       { return bl.Token.Literal }

// PrefixExpression represents a prefix operator expression
type PrefixExpression struct {
	Token    lexer.Token // the prefix token, e.g. !
	Operator string
	Right    Expression
}

func (pe *PrefixExpression) expressionNode()      {}
func (pe *PrefixExpression) TokenLiteral() string { return pe.Token.Literal }
func (pe *PrefixExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(pe.Operator)
	out.WriteString(pe.Right.String())
	out.WriteString(")")

	return out.String()
}

// InfixExpression represents an infix operator expression
type InfixExpression struct {
	Token    lexer.Token // the operator token, e.g. +
	Left     Expression
	Operator string
	Right    Expression
}

func (ie *InfixExpression) expressionNode()      {}
func (ie *InfixExpression) TokenLiteral() string { return ie.Token.Literal }
func (ie *InfixExpression) String() string {
	var out bytes.Buffer

	out.WriteString("(")
	out.WriteString(ie.Left.String())
	out.WriteString(" " + ie.Operator + " ")
	out.WriteString(ie.Right.String())
	out.WriteString(")")

	return out.String()
}

// FunctionLiteral represents a function literal
type FunctionLiteral struct {
	Token      lexer.Token // the 'fn' token
	Parameters []*Identifier
	Body       *BlockStatement
	Name       string
	ReturnType string // Add this field
}

func (fl *FunctionLiteral) expressionNode()      {}
func (fl *FunctionLiteral) TokenLiteral() string { return fl.Token.Literal }
func (fl *FunctionLiteral) String() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range fl.Parameters {
		params = append(params, p.String())
	}

	out.WriteString(fl.TokenLiteral())
	if fl.Name != "" {
		out.WriteString(" " + fl.Name)
	}
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(")")
	if fl.ReturnType != "" {
		out.WriteString(": " + fl.ReturnType)
	}
	out.WriteString(" ")
	out.WriteString(fl.Body.String())

	return out.String()
}

// CallExpression represents a function call
type CallExpression struct {
	Token     lexer.Token // the '(' token
	Function  Expression  // Identifier or FunctionLiteral
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Token.Literal }
func (ce *CallExpression) String() string {
	var out bytes.Buffer

	args := []string{}
	for _, a := range ce.Arguments {
		args = append(args, a.String())
	}

	out.WriteString(ce.Function.String())
	out.WriteString("(")
	out.WriteString(strings.Join(args, ", "))
	out.WriteString(")")

	return out.String()
}

// Print the AST for debugging
func PrintAST(node Node, indent string) {
	switch node := node.(type) {
	case *Program:
		fmt.Printf("%sProgram\n", indent)
		for _, stmt := range node.Statements {
			PrintAST(stmt, indent+"  ")
		}
	case *VarStatement:
		fmt.Printf("%sVarStatement: %s = \n", indent, node.Name.Value)
		PrintAST(node.Value, indent+"  ")
	case *ReturnStatement:
		fmt.Printf("%sReturnStatement:\n", indent)
		PrintAST(node.ReturnValue, indent+"  ")
	case *ExpressionStatement:
		fmt.Printf("%sExpressionStatement:\n", indent)
		PrintAST(node.Expression, indent+"  ")
	case *BlockStatement:
		fmt.Printf("%sBlockStatement:\n", indent)
		for _, stmt := range node.Statements {
			PrintAST(stmt, indent+"  ")
		}
	case *IfStatement:
		fmt.Printf("%sIfStatement:\n", indent)
		fmt.Printf("%sCondition:\n", indent)
		PrintAST(node.Condition, indent+"  ")
		fmt.Printf("%sConsequence:\n", indent)
		PrintAST(node.Consequence, indent+"  ")
		if node.Alternative != nil {
			fmt.Printf("%sAlternative:\n", indent)
			PrintAST(node.Alternative, indent+"  ")
		}
	case *WhileStatement:
		fmt.Printf("%sWhileStatement:\n", indent)
		fmt.Printf("%sCondition:\n", indent)
		PrintAST(node.Condition, indent+"  ")
		fmt.Printf("%sBody:\n", indent)
		PrintAST(node.Body, indent+"  ")
	case *PrintStatement:
		fmt.Printf("%sPrintStatement:\n", indent)
		PrintAST(node.Value, indent+"  ")
	case *Identifier:
		fmt.Printf("%sIdentifier: %s\n", indent, node.Value)
	case *IntegerLiteral:
		fmt.Printf("%sIntegerLiteral: %d\n", indent, node.Value)
	case *StringLiteral:
		fmt.Printf("%sStringLiteral: %s\n", indent, node.Value)
	case *BooleanLiteral:
		fmt.Printf("%sBooleanLiteral: %t\n", indent, node.Value)
	case *PrefixExpression:
		fmt.Printf("%sPrefixExpression: %s\n", indent, node.Operator)
		PrintAST(node.Right, indent+"  ")
	case *InfixExpression:
		fmt.Printf("%sInfixExpression: %s\n", indent, node.Operator)
		fmt.Printf("%sLeft:\n", indent)
		PrintAST(node.Left, indent+"  ")
		fmt.Printf("%sRight:\n", indent)
		PrintAST(node.Right, indent+"  ")
	case *FunctionLiteral:
		fmt.Printf("%sFunctionLiteral: %s\n", indent, node.Name)
		fmt.Printf("%sParameters:\n", indent)
		for _, param := range node.Parameters {
			PrintAST(param, indent+"  ")
		}
		fmt.Printf("%sBody:\n", indent)
		PrintAST(node.Body, indent+"  ")
	case *CallExpression:
		fmt.Printf("%sCallExpression:\n", indent)
		fmt.Printf("%sFunction:\n", indent)
		PrintAST(node.Function, indent+"  ")
		fmt.Printf("%sArguments:\n", indent)
		for _, arg := range node.Arguments {
			PrintAST(arg, indent+"  ")
		}
	}
}

// AssignmentExpression represents an assignment
type AssignmentExpression struct {
	Token lexer.Token // the = token
	Name  *Identifier
	Value Expression
}

func (ae *AssignmentExpression) expressionNode()      {}
func (ae *AssignmentExpression) TokenLiteral() string { return ae.Token.Literal }
func (ae *AssignmentExpression) String() string {
	var out bytes.Buffer
	out.WriteString(ae.Name.String())
	out.WriteString(" = ")
	out.WriteString(ae.Value.String())
	return out.String()
}

// EmptyExpression represents an empty expression
type EmptyExpression struct {
	Token lexer.Token
}

func (ee *EmptyExpression) expressionNode()      {}
func (ee *EmptyExpression) TokenLiteral() string { return ee.Token.Literal }
func (ee *EmptyExpression) String() string       { return "" }

// FunctionStatement represents a function declaration
type FunctionStatement struct {
	Token      lexer.Token // the FUNCTION token
	Name       *Identifier
	Parameters []*Identifier
	Body       *BlockStatement
	ReturnType string
}

func (fs *FunctionStatement) statementNode()       {}
func (fs *FunctionStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *FunctionStatement) String() string {
	var out bytes.Buffer

	params := []string{}
	for _, p := range fs.Parameters {
		params = append(params, p.String())
	}

	out.WriteString("function ")
	out.WriteString(fs.Name.String())
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") ")
	if fs.ReturnType != "" {
		out.WriteString(": " + fs.ReturnType + " ")
	}
	out.WriteString(fs.Body.String())

	return out.String()
}

// ForStatement represents a for loop
type ForStatement struct {
	Token     lexer.Token // the FOR token
	Init      Statement
	Condition Expression
	Update    Statement
	Body      *BlockStatement
}

func (fs *ForStatement) statementNode()       {}
func (fs *ForStatement) TokenLiteral() string { return fs.Token.Literal }
func (fs *ForStatement) String() string {
	var out bytes.Buffer

	out.WriteString("for(")
	if fs.Init != nil {
		out.WriteString(fs.Init.String())
	}
	out.WriteString("; ")
	if fs.Condition != nil {
		out.WriteString(fs.Condition.String())
	}
	out.WriteString("; ")
	if fs.Update != nil {
		out.WriteString(fs.Update.String())
	}
	out.WriteString(") ")
	out.WriteString(fs.Body.String())

	return out.String()
}

// ImportStatement represents an import statement
type ImportStatement struct {
	Token lexer.Token // the IMPORT token
	Path  *StringLiteral
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return is.Token.Literal }
func (is *ImportStatement) String() string {
	var out bytes.Buffer

	out.WriteString("import ")
	out.WriteString(is.Path.String())
	out.WriteString(";")

	return out.String()
}

// StructDefinition represents a struct definition
type StructDefinition struct {
	Token  lexer.Token // the STRUCT token
	Name   *Identifier
	Fields []*StructField
}

func (sd *StructDefinition) expressionNode()      {}
func (sd *StructDefinition) TokenLiteral() string { return sd.Token.Literal }
func (sd *StructDefinition) String() string {
	var out bytes.Buffer

	fields := []string{}
	for _, f := range sd.Fields {
		fields = append(fields, f.String())
	}

	out.WriteString("struct ")
	out.WriteString(sd.Name.String())
	out.WriteString(" {\n")
	out.WriteString(strings.Join(fields, "\n"))
	out.WriteString("\n}")

	return out.String()
}

// StructField represents a field in a struct
type StructField struct {
	Name *Identifier
	Type string
}

func (sf *StructField) String() string {
	return sf.Name.String() + ": " + sf.Type
}

// ArrayLiteral represents an array literal
type ArrayLiteral struct {
	Token    lexer.Token // the '[' token
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return al.Token.Literal }
func (al *ArrayLiteral) String() string {
	var out bytes.Buffer

	elements := []string{}
	for _, el := range al.Elements {
		elements = append(elements, el.String())
	}

	out.WriteString("[")
	out.WriteString(strings.Join(elements, ", "))
	out.WriteString("]")

	return out.String()
}
