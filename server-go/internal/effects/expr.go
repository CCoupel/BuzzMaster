package effects

import (
	"fmt"
	"strconv"
	"strings"
)

// expr.go — expressions of the start and the step of a variable (contract light-effects.md
// §12.2, v11.8.0, #210): integers, N, g*, g<k>, variable names, + - * /, parentheses, a
// leading sign. Integer arithmetic; the division truncates toward zero. The analysis is
// done once, at validation; evaluation is pure and allocation-free.

// MaxExprLen is the longest accepted expression (characters).
const MaxExprLen = 60

// maxExprInt is the largest integer literal.
const maxExprInt = 999

type exprOp byte

const (
	opNum exprOp = 'n' // integer literal
	opN   exprOp = 'N' // number of non-empty groups (also g*)
	opVar exprOp = 'v' // rank of the group a variable designates
	opNeg exprOp = 'u'
	opAdd exprOp = '+'
	opSub exprOp = '-'
	opMul exprOp = '*'
	opDiv exprOp = '/'
)

type exprNode struct {
	op   exprOp
	val  int    // opNum
	name string // opVar
	l, r *exprNode
}

// Expr is a parsed expression. The zero value is nil-like: use only through its pointer.
type Expr struct {
	Src  string // normalised source text (single spaces removed around nothing: as written, trimmed)
	root *exprNode
}

// exprError is a syntax or static error: Message is a sentence, without file or line.
type exprError struct{ Message string }

func (e *exprError) Error() string { return e.Message }

// ParseExpr analyses src. The returned expression keeps src (trimmed) for messages and for
// the document form.
func ParseExpr(src string) (*Expr, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, &exprError{"une expression est attendue (par exemple 1, N/2 ou gA+1)"}
	}
	if len(src) > MaxExprLen {
		return nil, &exprError{fmt.Sprintf("l'expression « %s… » dépasse %d caractères", src[:20], MaxExprLen)}
	}
	p := &exprParser{src: src}
	if err := p.lex(); err != nil {
		return nil, err
	}
	root, err := p.expression()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.toks) {
		if p.toks[p.pos].kind == tokRParen {
			return nil, &exprError{fmt.Sprintf("l'expression « %s » a une parenthèse fermante en trop", src)}
		}
		return nil, p.malformed()
	}
	x := &Expr{Src: src, root: root}
	if err := checkConstDivision(root, src); err != nil {
		return nil, err
	}
	return x, nil
}

type tokKind int

const (
	tokNum tokKind = iota
	tokN
	tokVar
	tokOp
	tokLParen
	tokRParen
)

type token struct {
	kind tokKind
	val  int    // tokNum, and g<k>
	name string // tokVar
	op   byte   // tokOp
}

type exprParser struct {
	src  string
	toks []token
	pos  int
}

func (p *exprParser) malformed() error {
	return &exprError{fmt.Sprintf("l'expression « %s » est mal écrite", p.src)}
}

func (p *exprParser) lex() error {
	s := p.src
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c >= '0' && c <= '9':
			j := i
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			v, err := strconv.Atoi(s[i:j])
			if err != nil || v > maxExprInt {
				return &exprError{fmt.Sprintf("l'entier « %s » est trop grand (%d au plus)", s[i:j], maxExprInt)}
			}
			if j < len(s) && isIdentChar(s[j]) {
				return p.malformed()
			}
			p.toks = append(p.toks, token{kind: tokNum, val: v})
			i = j
		case isLetter(c):
			j := i
			for j < len(s) && isIdentChar(s[j]) {
				j++
			}
			id := s[i:j]
			switch {
			case id == "N":
				p.toks = append(p.toks, token{kind: tokN})
			case id == "g" && j < len(s) && s[j] == '*':
				p.toks = append(p.toks, token{kind: tokN})
				j++
			case reGn.MatchString(id):
				k, _ := strconv.Atoi(id[1:])
				p.toks = append(p.toks, token{kind: tokNum, val: k})
			case reVarName.MatchString(id):
				p.toks = append(p.toks, token{kind: tokVar, name: id})
			default:
				return p.malformed()
			}
			i = j
		case c == '+' || c == '-' || c == '*' || c == '/':
			p.toks = append(p.toks, token{kind: tokOp, op: c})
			i++
		case c == '(':
			p.toks = append(p.toks, token{kind: tokLParen})
			i++
		case c == ')':
			p.toks = append(p.toks, token{kind: tokRParen})
			i++
		default:
			return p.malformed()
		}
	}
	return nil
}

func isLetter(c byte) bool { return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }
func isIdentChar(c byte) bool {
	return isLetter(c) || c >= '0' && c <= '9'
}

func (p *exprParser) peekOp(ops ...byte) (byte, bool) {
	if p.pos >= len(p.toks) || p.toks[p.pos].kind != tokOp {
		return 0, false
	}
	for _, o := range ops {
		if p.toks[p.pos].op == o {
			return o, true
		}
	}
	return 0, false
}

func (p *exprParser) expression() (*exprNode, error) {
	left, err := p.term()
	if err != nil {
		return nil, err
	}
	for {
		op, ok := p.peekOp('+', '-')
		if !ok {
			return left, nil
		}
		p.pos++
		right, err := p.term()
		if err != nil {
			return nil, err
		}
		left = &exprNode{op: exprOp(op), l: left, r: right}
	}
}

func (p *exprParser) term() (*exprNode, error) {
	left, err := p.factor()
	if err != nil {
		return nil, err
	}
	for {
		op, ok := p.peekOp('*', '/')
		if !ok {
			return left, nil
		}
		p.pos++
		right, err := p.factor()
		if err != nil {
			return nil, err
		}
		left = &exprNode{op: exprOp(op), l: left, r: right}
	}
}

func (p *exprParser) factor() (*exprNode, error) {
	if p.pos >= len(p.toks) {
		return nil, &exprError{fmt.Sprintf("l'expression « %s » est incomplète", p.src)}
	}
	t := p.toks[p.pos]
	switch t.kind {
	case tokNum:
		p.pos++
		return &exprNode{op: opNum, val: t.val}, nil
	case tokN:
		p.pos++
		return &exprNode{op: opN}, nil
	case tokVar:
		p.pos++
		return &exprNode{op: opVar, name: t.name}, nil
	case tokLParen:
		p.pos++
		inner, err := p.expression()
		if err != nil {
			return nil, err
		}
		if p.pos >= len(p.toks) || p.toks[p.pos].kind != tokRParen {
			return nil, &exprError{fmt.Sprintf("l'expression « %s » a une parenthèse non fermée", p.src)}
		}
		p.pos++
		return inner, nil
	case tokOp:
		if t.op == '+' || t.op == '-' {
			p.pos++
			inner, err := p.factor()
			if err != nil {
				return nil, err
			}
			if t.op == '+' {
				return inner, nil
			}
			return &exprNode{op: opNeg, l: inner}, nil
		}
	}
	return nil, p.malformed()
}

// ExprLimit bounds every intermediate value of an expression (|v| ≤ 1 000 000): beyond it a
// constant is refused at validation and a run-time value is "without value". The bound keeps
// the arithmetic identical on 32- and 64-bit machines (§12.2).
const ExprLimit = 1000000

// constValue returns the value of a node that depends on neither N nor a variable; over=true
// when an intermediate value exceeds ExprLimit.
func constValue(n *exprNode) (v int64, isConst, over bool) {
	switch n.op {
	case opNum:
		return int64(n.val), true, false
	case opN, opVar:
		return 0, false, false
	case opNeg:
		v, ok, over := constValue(n.l)
		return -v, ok, over
	}
	a, ok, o1 := constValue(n.l)
	b, ok2, o2 := constValue(n.r)
	if o1 || o2 {
		return 0, false, true
	}
	if !ok || !ok2 {
		return 0, false, false
	}
	switch n.op {
	case opAdd:
		v = a + b
	case opSub:
		v = a - b
	case opMul:
		v = a * b
	case opDiv:
		if b == 0 {
			return 0, false, false
		}
		v = a / b
	}
	if v > ExprLimit || v < -ExprLimit {
		return 0, false, true
	}
	return v, true, false
}

// checkConstDivision refuses a division by a constant zero written in the expression (N/0,
// 3/(2-2)) and a constant that grows past ExprLimit; a divisor that depends on N or a variable
// is only checked when evaluated.
func checkConstDivision(n *exprNode, src string) error {
	if _, _, over := constValue(n); over {
		return &exprError{fmt.Sprintf("l'expression « %s » dépasse la valeur permise (%d au plus)", src, ExprLimit)}
	}
	return checkZero(n, src)
}

func checkZero(n *exprNode, src string) error {
	if n == nil {
		return nil
	}
	if n.op == opDiv {
		if b, ok, _ := constValue(n.r); ok && b == 0 {
			return &exprError{fmt.Sprintf("division par zéro dans « %s »", src)}
		}
	}
	if err := checkZero(n.l, src); err != nil {
		return err
	}
	return checkZero(n.r, src)
}

// Refs lists the variable names the expression cites (without duplicates, in order of appearance).
func (x *Expr) Refs() []string {
	var out []string
	seen := map[string]bool{}
	var walk func(n *exprNode)
	walk = func(n *exprNode) {
		if n == nil {
			return
		}
		if n.op == opVar && !seen[n.name] {
			seen[n.name] = true
			out = append(out, n.name)
		}
		walk(n.l)
		walk(n.r)
	}
	walk(x.root)
	return out
}

// eval computes the expression with n groups; ref gives the rank (1…n) of a cited variable.
// ok=false: no value (a division by zero at run time, a cited variable without value, or an
// intermediate value past ExprLimit).
func (x *Expr) eval(n int, ref func(name string) (int, bool)) (int, bool) {
	v, ok := x.root.eval(n, ref)
	return int(v), ok
}

func (nd *exprNode) eval(n int, ref func(string) (int, bool)) (int64, bool) {
	var v int64
	switch nd.op {
	case opNum:
		return int64(nd.val), true
	case opN:
		return int64(n), true
	case opVar:
		if ref == nil {
			return 0, false
		}
		r, ok := ref(nd.name)
		return int64(r), ok
	case opNeg:
		a, ok := nd.l.eval(n, ref)
		return -a, ok
	}
	a, ok := nd.l.eval(n, ref)
	if !ok {
		return 0, false
	}
	b, ok := nd.r.eval(n, ref)
	if !ok {
		return 0, false
	}
	switch nd.op {
	case opAdd:
		v = a + b
	case opSub:
		v = a - b
	case opMul:
		v = a * b
	case opDiv:
		if b == 0 {
			return 0, false
		}
		v = a / b // Go truncates toward zero, as the contract asks
	}
	if v > ExprLimit || v < -ExprLimit {
		return 0, false
	}
	return v, true
}
