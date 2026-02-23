package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/extism/go-pdk"
)

type calcInput struct {
	Expression string `json:"expression"`
}

type calcOutput struct {
	Result float64 `json:"result"`
}

type calcError struct {
	Error string `json:"error"`
}

//export handle
func handle() int32 {
	input := pdk.Input()

	var req calcInput
	if err := json.Unmarshal(input, &req); err != nil {
		return outputError("invalid input: " + err.Error())
	}

	if req.Expression == "" {
		return outputError("expression is required")
	}

	result, err := evaluate(req.Expression)
	if err != nil {
		return outputError(err.Error())
	}

	out, _ := json.Marshal(calcOutput{Result: result})
	pdk.Output(out)
	return 0
}

func outputError(msg string) int32 {
	out, _ := json.Marshal(calcError{Error: msg})
	pdk.Output(out)
	return 1
}

// Simple recursive descent parser for arithmetic expressions.
// Supports: +, -, *, /, parentheses, decimal numbers.
type parser struct {
	tokens []token
	pos    int
}

type tokenType int

const (
	tokenNumber tokenType = iota
	tokenPlus
	tokenMinus
	tokenMul
	tokenDiv
	tokenLParen
	tokenRParen
)

type token struct {
	typ tokenType
	val float64
}

func tokenize(expr string) ([]token, error) {
	var tokens []token
	expr = strings.TrimSpace(expr)
	i := 0
	for i < len(expr) {
		ch := rune(expr[i])
		switch {
		case unicode.IsSpace(ch):
			i++
		case ch == '+':
			tokens = append(tokens, token{typ: tokenPlus})
			i++
		case ch == '-':
			// Handle negative numbers: unary minus at start or after operator/left paren
			if len(tokens) == 0 || tokens[len(tokens)-1].typ == tokenLParen ||
				tokens[len(tokens)-1].typ == tokenPlus ||
				tokens[len(tokens)-1].typ == tokenMinus ||
				tokens[len(tokens)-1].typ == tokenMul ||
				tokens[len(tokens)-1].typ == tokenDiv {
				j := i + 1
				for j < len(expr) && (expr[j] == '.' || (expr[j] >= '0' && expr[j] <= '9')) {
					j++
				}
				if j > i+1 {
					val, err := strconv.ParseFloat(expr[i:j], 64)
					if err != nil {
						return nil, fmt.Errorf("invalid number: %s", expr[i:j])
					}
					tokens = append(tokens, token{typ: tokenNumber, val: val})
					i = j
				} else {
					tokens = append(tokens, token{typ: tokenMinus})
					i++
				}
			} else {
				tokens = append(tokens, token{typ: tokenMinus})
				i++
			}
		case ch == '*':
			tokens = append(tokens, token{typ: tokenMul})
			i++
		case ch == '/':
			tokens = append(tokens, token{typ: tokenDiv})
			i++
		case ch == '(':
			tokens = append(tokens, token{typ: tokenLParen})
			i++
		case ch == ')':
			tokens = append(tokens, token{typ: tokenRParen})
			i++
		case ch >= '0' && ch <= '9', ch == '.':
			j := i
			for j < len(expr) && (expr[j] == '.' || (expr[j] >= '0' && expr[j] <= '9')) {
				j++
			}
			val, err := strconv.ParseFloat(expr[i:j], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid number: %s", expr[i:j])
			}
			tokens = append(tokens, token{typ: tokenNumber, val: val})
			i = j
		default:
			return nil, fmt.Errorf("unexpected character: %c", ch)
		}
	}
	return tokens, nil
}

func evaluate(expr string) (float64, error) {
	tokens, err := tokenize(expr)
	if err != nil {
		return 0, err
	}
	if len(tokens) == 0 {
		return 0, fmt.Errorf("empty expression")
	}
	p := &parser{tokens: tokens}
	result, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.pos < len(p.tokens) {
		return 0, fmt.Errorf("unexpected token at position %d", p.pos)
	}
	return result, nil
}

func (p *parser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for p.pos < len(p.tokens) {
		t := p.tokens[p.pos]
		if t.typ != tokenPlus && t.typ != tokenMinus {
			break
		}
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if t.typ == tokenPlus {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

func (p *parser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for p.pos < len(p.tokens) {
		t := p.tokens[p.pos]
		if t.typ != tokenMul && t.typ != tokenDiv {
			break
		}
		p.pos++
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if t.typ == tokenMul {
			left *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		}
	}
	return left, nil
}

func (p *parser) parseFactor() (float64, error) {
	if p.pos >= len(p.tokens) {
		return 0, fmt.Errorf("unexpected end of expression")
	}
	t := p.tokens[p.pos]
	if t.typ == tokenNumber {
		p.pos++
		return t.val, nil
	}
	if t.typ == tokenLParen {
		p.pos++
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		if p.pos >= len(p.tokens) || p.tokens[p.pos].typ != tokenRParen {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		p.pos++
		return val, nil
	}
	return 0, fmt.Errorf("unexpected token")
}

func main() {}
