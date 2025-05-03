package lexer

import (
	"fmt"
	"regexp"
	"strings"
)

// TokenType represents the type of token
type TokenType int

const (
	ILLEGAL TokenType = iota
	EOF

	// Identifiers + literals
	IDENT  // add, foobar, x, y, ...
	INT    // 1234
	STRING // "hello world"

	// Operators
	ASSIGN   // =
	PLUS     // +
	MINUS    // -
	BANG     // !
	ASTERISK // *
	SLASH    // /
	MODULO   // %

	LT     // <
	GT     // >
	EQ     // ==
	NOT_EQ // !=
	LE     // <=
	GE     // >=

	// Delimiters
	COMMA     // ,
	SEMICOLON // ;
	LPAREN    // (
	RPAREN    // )
	LBRACE    // {
	RBRACE    // }

	// Keywords
	FUNCTION // fn
	VAR      // var
	IF       // if
	ELSE     // else
	RETURN   // return
	WHILE    // while
	PRINT    // print
)

var tokenNames = map[TokenType]string{
	ILLEGAL:   "ILLEGAL",
	EOF:       "EOF",
	IDENT:     "IDENT",
	INT:       "INT",
	STRING:    "STRING",
	ASSIGN:    "=",
	PLUS:      "+",
	MINUS:     "-",
	BANG:      "!",
	ASTERISK:  "*",
	SLASH:     "/",
	MODULO:    "%",
	LT:        "<",
	GT:        ">",
	EQ:        "==",
	NOT_EQ:    "!=",
	LE:        "<=",
	GE:        ">=",
	COMMA:     ",",
	SEMICOLON: ";",
	LPAREN:    "(",
	RPAREN:    ")",
	LBRACE:    "{",
	RBRACE:    "}",
	FUNCTION:  "FUNCTION",
	VAR:       "VAR",
	IF:        "IF",
	ELSE:      "ELSE",
	RETURN:    "RETURN",
	WHILE:     "WHILE",
	PRINT:     "PRINT",
}

func (tt TokenType) String() string {
	return tokenNames[tt]
}

// Token represents a token in the language
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

// Lexer converts source code into tokens
type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           byte // current char under examination
	line         int  // current line number
	column       int  // current column number
}

// New creates a new Lexer
func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1, column: 0}
	l.readChar()
	return l
}

// readChar reads the next character and advances the position
func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
	l.column++

	// Track line numbers and reset column on newline
	if l.ch == '\n' {
		l.line++
		l.column = 0
	}
}

// peekChar returns the next character without advancing the position
func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

// NextToken returns the next token from the input
func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	// Store the start column for this token
	startColumn := l.column

	switch l.ch {
	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: EQ, Literal: string(ch) + string(l.ch), Line: l.line, Column: startColumn}
		} else {
			tok = Token{Type: ASSIGN, Literal: string(l.ch), Line: l.line, Column: startColumn}
		}
	case '+':
		tok = Token{Type: PLUS, Literal: string(l.ch), Line: l.line, Column: startColumn}
	case '-':
		tok = Token{Type: MINUS, Literal: string(l.ch), Line: l.line, Column: startColumn}
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: NOT_EQ, Literal: string(ch) + string(l.ch), Line: l.line, Column: startColumn}
		} else {
			tok = Token{Type: BANG, Literal: string(l.ch), Line: l.line, Column: startColumn}
		}
	case '*':
		tok = Token{Type: ASTERISK, Literal: string(l.ch), Line: l.line, Column: startColumn}
	case '/':
		tok = Token{Type: SLASH, Literal: string(l.ch), Line: l.line, Column: startColumn}
	case '%':
		tok = Token{Type: MODULO, Literal: string(l.ch), Line: l.line, Column: startColumn}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: LE, Literal: string(ch) + string(l.ch), Line: l.line, Column: startColumn}
		} else {
			tok = Token{Type: LT, Literal: string(l.ch), Line: l.line, Column: startColumn}
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: GE, Literal: string(ch) + string(l.ch), Line: l.line, Column: startColumn}
		} else {
			tok = Token{Type: GT, Literal: string(l.ch), Line: l.line, Column: startColumn}
		}
	case ',':
		tok = Token{Type: COMMA, Literal: string(l.ch), Line: l.line, Column: startColumn}
	case ';':
		tok = Token{Type: SEMICOLON, Literal: string(l.ch), Line: l.line, Column: startColumn}
	case '(':
		tok = Token{Type: LPAREN, Literal: string(l.ch), Line: l.line, Column: startColumn}
	case ')':
		tok = Token{Type: RPAREN, Literal: string(l.ch), Line: l.line, Column: startColumn}
	case '{':
		tok = Token{Type: LBRACE, Literal: string(l.ch), Line: l.line, Column: startColumn}
	case '}':
		tok = Token{Type: RBRACE, Literal: string(l.ch), Line: l.line, Column: startColumn}
	case '"':
		tok.Type = STRING
		tok.Literal = l.readString()
		tok.Line = l.line
		tok.Column = startColumn
	case 0:
		tok.Type = EOF
		tok.Literal = ""
		tok.Line = l.line
		tok.Column = startColumn
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = lookupIdent(tok.Literal)
			tok.Line = l.line
			tok.Column = startColumn
			return tok
		} else if isDigit(l.ch) {
			tok.Type = INT
			tok.Literal = l.readNumber()
			tok.Line = l.line
			tok.Column = startColumn
			return tok
		} else {
			tok = Token{Type: ILLEGAL, Literal: string(l.ch), Line: l.line, Column: startColumn}
		}
	}

	l.readChar()
	return tok
}

// skipWhitespace skips whitespace characters
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

// readIdentifier reads an identifier
func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readNumber reads a number
func (l *Lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readString reads a string
func (l *Lexer) readString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '"' || l.ch == 0 {
			break
		}
	}
	return l.input[position:l.position]
}

// isLetter returns true if the character is a letter or underscore
func isLetter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

// isDigit returns true if the character is a digit
func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

// lookupIdent checks if the identifier is a keyword
func lookupIdent(ident string) TokenType {
	keywords := map[string]TokenType{
		"fn":     FUNCTION,
		"var":    VAR,
		"if":     IF,
		"else":   ELSE,
		"return": RETURN,
		"while":  WHILE,
		"print":  PRINT,
	}

	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

// Tokenize converts source code into a list of tokens
func Tokenize(source string) []Token {
	l := New(source)
	var tokens []Token

	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == EOF {
			break
		}
	}

	return tokens
}

// RemoveComments removes comments from source code
func RemoveComments(source string) string {
	// Remove single-line comments
	re := regexp.MustCompile(`//.*\n`)
	source = re.ReplaceAllString(source, "\n")

	// Remove multi-line comments
	re = regexp.MustCompile(`(?s)/\*.*?\*/`)
	source = re.ReplaceAllString(source, "")

	return source
}

// Print tokens for debugging
func PrintTokens(tokens []Token) {
	for i, tok := range tokens {
		fmt.Printf("%d: Type=%s, Literal=%q, Line=%d, Column=%d\n",
			i, tok.Type, tok.Literal, tok.Line, tok.Column)
	}
}

// TokenizeFile tokenizes a source file and returns the tokens
func TokenizeFile(source string) ([]Token, error) {
	// Remove comments
	source = RemoveComments(source)

	// Normalize line endings
	source = strings.ReplaceAll(source, "\r\n", "\n")

	// Tokenize
	tokens := Tokenize(source)

	return tokens, nil
}
