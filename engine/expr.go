package engine

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// SyntaxError — ошибка разбора выражения. Pos — номер символа с нуля.
type SyntaxError struct {
	Pos int
	Msg string
}

func (e *SyntaxError) Error() string { return fmt.Sprintf("символ %d: %s", e.Pos+1, e.Msg) }

// ───────────────────────────── лексер ─────────────────────────────

type tokKind int

const (
	tNum tokKind = iota
	tIdent
	tOp
	tLParen
	tRParen
	tComma
	tEOF
)

type token struct {
	kind       tokKind
	text       string
	num        float64
	start, end int
}

// normRune превращает «типографские» знаки из методичек в обычные.
func normRune(r rune) rune {
	switch r {
	case '−', '–', '—':
		return '-'
	case '×', '·', '⋅':
		return '*'
	case '÷':
		return '/'
	}
	return r
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func lex(src []rune) ([]token, error) {
	var toks []token
	i := 0
	for i < len(src) {
		r := normRune(src[i])
		switch {
		case unicode.IsSpace(r):
			i++

		case isDigit(r) || (r == '.' && i+1 < len(src) && isDigit(src[i+1])):
			start := i
			for i < len(src) && isDigit(src[i]) {
				i++
			}
			if i < len(src) && src[i] == '.' {
				i++
				for i < len(src) && isDigit(src[i]) {
					i++
				}
			}
			text := string(src[start:i])
			v, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return nil, &SyntaxError{start, "некорректное число «" + text + "»"}
			}
			if i < len(src) && (unicode.IsLetter(src[i]) || src[i] == '_') {
				return nil, &SyntaxError{i, "после числа нужен знак операции (например *), а не буква"}
			}
			toks = append(toks, token{kind: tNum, text: text, num: v, start: start, end: i})

		case unicode.IsLetter(r) || r == '_':
			start := i
			for i < len(src) && (unicode.IsLetter(src[i]) || unicode.IsDigit(src[i]) || src[i] == '_') {
				i++
			}
			toks = append(toks, token{kind: tIdent, text: string(src[start:i]), start: start, end: i})

		case r == '(':
			toks = append(toks, token{kind: tLParen, text: "(", start: i, end: i + 1})
			i++
		case r == ')':
			toks = append(toks, token{kind: tRParen, text: ")", start: i, end: i + 1})
			i++
		case r == ',' || r == ';':
			toks = append(toks, token{kind: tComma, text: string(r), start: i, end: i + 1})
			i++

		case strings.ContainsRune("+-*/^", r):
			toks = append(toks, token{kind: tOp, text: string(r), start: i, end: i + 1})
			i++

		case r == '<' || r == '>' || r == '=' || r == '!':
			start := i
			text := string(r)
			i++
			if i < len(src) && src[i] == '=' {
				text += "="
				i++
			}
			switch text {
			case "<", "<=", ">", ">=", "==", "!=":
				toks = append(toks, token{kind: tOp, text: text, start: start, end: i})
			case "=":
				return nil, &SyntaxError{start, "для сравнения используйте «==»; знак «=» в выражении не нужен"}
			default:
				return nil, &SyntaxError{start, "неизвестный оператор «" + text + "»"}
			}

		case r == '%':
			return nil, &SyntaxError{i, "знак «%» не поддерживается: вместо 5% пишите 5/100"}
		case strings.ContainsRune("[]{}", r):
			return nil, &SyntaxError{i, "для группировки используйте только круглые скобки"}
		default:
			return nil, &SyntaxError{i, fmt.Sprintf("недопустимый символ «%c»", src[i])}
		}
	}
	toks = append(toks, token{kind: tEOF, start: len(src), end: len(src)})
	return toks, nil
}

// ───────────────────────────── функции ─────────────────────────────

type fn struct {
	min, max int // max < 0 — любое количество
	f        func(a []float64) (float64, error)
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

var funcs = map[string]fn{
	"abs":   {1, 1, func(a []float64) (float64, error) { return math.Abs(a[0]), nil }},
	"floor": {1, 1, func(a []float64) (float64, error) { return math.Floor(a[0]), nil }},
	"ceil":  {1, 1, func(a []float64) (float64, error) { return math.Ceil(a[0]), nil }},
	"exp":   {1, 1, func(a []float64) (float64, error) { return math.Exp(a[0]), nil }},
	"pow":   {2, 2, func(a []float64) (float64, error) { return math.Pow(a[0], a[1]), nil }},
	"sqrt": {1, 1, func(a []float64) (float64, error) {
		if a[0] < 0 {
			return 0, errors.New("корень из отрицательного числа")
		}
		return math.Sqrt(a[0]), nil
	}},
	"ln": {1, 1, func(a []float64) (float64, error) {
		if a[0] <= 0 {
			return 0, errors.New("логарифм неположительного числа")
		}
		return math.Log(a[0]), nil
	}},
	"min": {2, -1, func(a []float64) (float64, error) {
		m := a[0]
		for _, x := range a[1:] {
			m = math.Min(m, x)
		}
		return m, nil
	}},
	"max": {2, -1, func(a []float64) (float64, error) {
		m := a[0]
		for _, x := range a[1:] {
			m = math.Max(m, x)
		}
		return m, nil
	}},
	"round": {1, 2, func(a []float64) (float64, error) {
		digits := 0.0
		if len(a) == 2 {
			digits = a[1]
		}
		p := math.Pow(10, digits)
		return math.Round(a[0]*p) / p, nil
	}},
	"if": {3, 3, nil}, // вычисляется лениво в callNode
}

// FuncNames — список поддерживаемых функций (для подсказок).
func FuncNames() []string {
	names := make([]string, 0, len(funcs))
	for n := range funcs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func arityText(f fn) string {
	switch {
	case f.max < 0:
		return fmt.Sprintf("не меньше %d аргументов", f.min)
	case f.min == f.max:
		return fmt.Sprintf("ровно %d арг.", f.min)
	default:
		return fmt.Sprintf("от %d до %d аргументов", f.min, f.max)
	}
}

// ───────────────────────────── AST ─────────────────────────────

type node interface {
	eval(vars map[string]float64) (float64, error)
}

type numNode struct{ v float64 }
type identNode struct{ name string }
type negNode struct{ x node }
type binNode struct {
	op   string
	l, r node
}
type callNode struct {
	name string
	args []node
}

func finite(x float64) (float64, error) {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return 0, errors.New("результат не является конечным числом")
	}
	return x, nil
}

func (n *numNode) eval(map[string]float64) (float64, error) { return n.v, nil }

func (n *identNode) eval(vars map[string]float64) (float64, error) {
	v, ok := vars[n.name]
	if !ok {
		return 0, fmt.Errorf("нет значения для «%s»", n.name)
	}
	return v, nil
}

func (n *negNode) eval(vars map[string]float64) (float64, error) {
	v, err := n.x.eval(vars)
	return -v, err
}

func (n *binNode) eval(vars map[string]float64) (float64, error) {
	a, err := n.l.eval(vars)
	if err != nil {
		return 0, err
	}
	b, err := n.r.eval(vars)
	if err != nil {
		return 0, err
	}
	var res float64
	switch n.op {
	case "+":
		res = a + b
	case "-":
		res = a - b
	case "*":
		res = a * b
	case "/":
		if b == 0 {
			return 0, errors.New("деление на ноль")
		}
		res = a / b
	case "^":
		res = math.Pow(a, b)
	case "<":
		res = b2f(a < b)
	case "<=":
		res = b2f(a <= b)
	case ">":
		res = b2f(a > b)
	case ">=":
		res = b2f(a >= b)
	case "==":
		res = b2f(a == b)
	case "!=":
		res = b2f(a != b)
	}
	return finite(res)
}

func (n *callNode) eval(vars map[string]float64) (float64, error) {
	if n.name == "if" {
		c, err := n.args[0].eval(vars)
		if err != nil {
			return 0, err
		}
		if c != 0 {
			return n.args[1].eval(vars)
		}
		return n.args[2].eval(vars)
	}
	args := make([]float64, len(n.args))
	for i, a := range n.args {
		v, err := a.eval(vars)
		if err != nil {
			return 0, err
		}
		args[i] = v
	}
	v, err := funcs[n.name].f(args)
	if err != nil {
		return 0, err
	}
	return finite(v)
}

// ───────────────────────────── парсер ─────────────────────────────

type identRef struct {
	name       string
	start, end int
}

type parser struct {
	toks   []token
	pos    int
	idents []identRef
}

func (p *parser) peek() token { return p.toks[p.pos] }
func (p *parser) next() token {
	t := p.toks[p.pos]
	if t.kind != tEOF {
		p.pos++
	}
	return t
}

func isCmp(s string) bool {
	switch s {
	case "<", "<=", ">", ">=", "==", "!=":
		return true
	}
	return false
}

func (p *parser) parseCmp() (node, error) {
	l, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind == tOp && isCmp(t.text) {
		p.next()
		r, err := p.parseAdd()
		if err != nil {
			return nil, err
		}
		if n := p.peek(); n.kind == tOp && isCmp(n.text) {
			return nil, &SyntaxError{n.start, "цепочки сравнений не поддерживаются"}
		}
		return &binNode{op: t.text, l: l, r: r}, nil
	}
	return l, nil
}

func (p *parser) parseAdd() (node, error) {
	l, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t.kind != tOp || (t.text != "+" && t.text != "-") {
			return l, nil
		}
		p.next()
		r, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		l = &binNode{op: t.text, l: l, r: r}
	}
}

func (p *parser) parseMul() (node, error) {
	l, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.peek()
		if t.kind != tOp || (t.text != "*" && t.text != "/") {
			return l, nil
		}
		p.next()
		r, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		l = &binNode{op: t.text, l: l, r: r}
	}
}

func (p *parser) parseUnary() (node, error) {
	if t := p.peek(); t.kind == tOp && (t.text == "-" || t.text == "+") {
		p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		if t.text == "-" {
			return &negNode{x}, nil
		}
		return x, nil
	}
	return p.parsePow()
}

func (p *parser) parsePow() (node, error) {
	base, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind == tOp && t.text == "^" {
		p.next()
		exp, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &binNode{op: "^", l: base, r: exp}, nil
	}
	return base, nil
}

func (p *parser) parsePrimary() (node, error) {
	t := p.next()
	switch t.kind {
	case tNum:
		return &numNode{t.num}, nil
	case tIdent:
		if p.peek().kind == tLParen {
			return p.parseCall(t)
		}
		p.idents = append(p.idents, identRef{t.text, t.start, t.end})
		return &identNode{t.text}, nil
	case tLParen:
		e, err := p.parseCmp()
		if err != nil {
			return nil, err
		}
		if c := p.peek(); c.kind != tRParen {
			return nil, &SyntaxError{c.start, "не закрыта скобка"}
		}
		p.next()
		return e, nil
	case tEOF:
		return nil, &SyntaxError{t.start, "выражение обрывается: ожидается число, имя или скобка"}
	default:
		return nil, &SyntaxError{t.start, fmt.Sprintf("здесь ожидается число, имя или скобка, а найдено «%s»", t.text)}
	}
}

func (p *parser) parseCall(name token) (node, error) {
	f, ok := funcs[name.text]
	if !ok {
		return nil, &SyntaxError{name.start, fmt.Sprintf("неизвестная функция «%s». Доступны: %s",
			name.text, strings.Join(FuncNames(), ", "))}
	}
	p.next() // «(»
	var args []node
	if p.peek().kind == tRParen {
		p.next()
	} else {
		for {
			a, err := p.parseCmp()
			if err != nil {
				return nil, err
			}
			args = append(args, a)
			t := p.next()
			if t.kind == tComma {
				continue
			}
			if t.kind == tRParen {
				break
			}
			if t.kind == tEOF {
				return nil, &SyntaxError{t.start, "не закрыта скобка функции «" + name.text + "»"}
			}
			return nil, &SyntaxError{t.start, "в аргументах ожидается запятая или закрывающая скобка"}
		}
	}
	if len(args) < f.min || (f.max >= 0 && len(args) > f.max) {
		return nil, &SyntaxError{name.start, fmt.Sprintf(
			"функция «%s» принимает %s, а передано %d. Десятичный разделитель — точка (0.95), запятая разделяет аргументы",
			name.text, arityText(f), len(args))}
	}
	return &callNode{name: name.text, args: args}, nil
}

// ───────────────────────────── Expr ─────────────────────────────

// Expr — разобранное выражение.
type Expr struct {
	src    []rune
	root   node
	idents []identRef
}

// ParseExpr разбирает выражение и не выполняет его.
func ParseExpr(src string) (*Expr, error) {
	if strings.TrimSpace(src) == "" {
		return nil, &SyntaxError{0, "выражение пустое"}
	}
	rs := []rune(src)
	toks, err := lex(rs)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	root, err := p.parseCmp()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind != tEOF {
		switch t.kind {
		case tComma:
			return nil, &SyntaxError{t.start, "запятая допустима только между аргументами функции; десятичный разделитель — точка (0.95)"}
		case tRParen:
			return nil, &SyntaxError{t.start, "лишняя закрывающая скобка"}
		default:
			return nil, &SyntaxError{t.start, fmt.Sprintf("ожидался знак операции (+ - * / ^), а найдено «%s»", t.text)}
		}
	}
	return &Expr{src: rs, root: root, idents: p.idents}, nil
}

// Idents возвращает уникальные имена переменных в порядке появления.
func (e *Expr) Idents() []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range e.idents {
		if !seen[id.name] {
			seen[id.name] = true
			out = append(out, id.name)
		}
	}
	return out
}

// Eval вычисляет выражение. Деление на ноль, NaN и Inf — ошибки.
func (e *Expr) Eval(vars map[string]float64) (float64, error) { return e.root.eval(vars) }

// FormatNumber — компактная запись числа (до 4 знаков после точки).
func FormatNumber(v float64) string {
	if v != 0 && math.Abs(v) < 0.0001 {
		return strconv.FormatFloat(v, 'g', 3, 64)
	}
	s := strconv.FormatFloat(v, 'f', 4, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

// Substitute подставляет значения вместо имён, сохраняя запись пользователя.
func (e *Expr) Substitute(vars map[string]float64) string {
	var b strings.Builder
	last := 0
	for _, id := range e.idents {
		v, ok := vars[id.name]
		if !ok {
			continue
		}
		b.WriteString(string(e.src[last:id.start]))
		s := FormatNumber(v)
		if v < 0 {
			s = "(" + s + ")"
		}
		b.WriteString(s)
		last = id.end
	}
	b.WriteString(string(e.src[last:]))
	return b.String()
}
