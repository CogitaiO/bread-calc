// Package engine — расчётное ядро BreadCalc: параметры, формулы, проверка и вычисление.
// Не зависит от интерфейса и использует только стандартную библиотеку.
package engine

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

// Param — настраиваемый числовой параметр (норма, влажность и т. п.).
type Param struct {
	ID    string
	Title string
	Unit  string
	Note  string
	Value float64 // значение по умолчанию
}

// Formula — формула расчёта.
type Formula struct {
	ID    string
	Title string
	Unit  string
	Expr  string
	Ref   string // обозначение в методичке, например «(5)»
	Group string // если задана, формула считается только при включённой группе
	Final bool   // итоговый результат: выделяется в отчёте
}

// Model — набор параметров и формул.
type Model struct {
	Params   []Param
	Formulas []Formula
}

// Var — системная переменная, которую задаёт рецептура.
type Var struct{ ID, Title string }

// SystemVars — переменные, доступные во всех формулах.
var SystemVars = []Var{
	{"Mc", "суммарная масса сырья, кг"},
	{"SumMW", "сумма «масса × влажность» по сырью"},
	{"Cnt", "количество видов сырья"},
}

// Problem — найденная при проверке ошибка.
type Problem struct {
	Kind string // "param" или "formula"
	ID   string
	Msg  string
}

func (p Problem) String() string {
	if p.ID == "" {
		return p.Msg
	}
	return fmt.Sprintf("«%s»: %s", p.ID, p.Msg)
}

// ───────────────────────────── вспомогательное ─────────────────────────────

func (m *Model) Clone() *Model {
	return &Model{
		Params:   append([]Param(nil), m.Params...),
		Formulas: append([]Formula(nil), m.Formulas...),
	}
}

func (m *Model) FindParam(id string) int {
	for i, p := range m.Params {
		if p.ID == id {
			return i
		}
	}
	return -1
}

func (m *Model) FindFormula(id string) int {
	for i, f := range m.Formulas {
		if f.ID == id {
			return i
		}
	}
	return -1
}

// WithFormula возвращает копию модели с добавленной или заменённой формулой.
// oldID — прежнее имя заменяемой формулы ("" — добавить новую).
func (m *Model) WithFormula(f Formula, oldID string) *Model {
	c := m.Clone()
	if i := c.FindFormula(oldID); oldID != "" && i >= 0 {
		c.Formulas[i] = f
	} else {
		c.Formulas = append(c.Formulas, f)
	}
	return c
}

// WithParam — то же для параметров.
func (m *Model) WithParam(p Param, oldID string) *Model {
	c := m.Clone()
	if i := c.FindParam(oldID); oldID != "" && i >= 0 {
		c.Params[i] = p
	} else {
		c.Params = append(c.Params, p)
	}
	return c
}

func (m *Model) WithoutFormula(id string) *Model {
	c := m.Clone()
	if i := c.FindFormula(id); i >= 0 {
		c.Formulas = append(c.Formulas[:i], c.Formulas[i+1:]...)
	}
	return c
}

func (m *Model) WithoutParam(id string) *Model {
	c := m.Clone()
	if i := c.FindParam(id); i >= 0 {
		c.Params = append(c.Params[:i], c.Params[i+1:]...)
	}
	return c
}

// UsedBy возвращает имена формул, в выражениях которых встречается id.
func (m *Model) UsedBy(id string) []string {
	var out []string
	for _, f := range m.Formulas {
		if f.ID == id {
			continue
		}
		e, err := ParseExpr(f.Expr)
		if err != nil {
			continue
		}
		for _, name := range e.Idents() {
			if name == id {
				out = append(out, f.ID)
				break
			}
		}
	}
	return out
}

// Groups — названия групп в порядке первого появления.
func (m *Model) Groups() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range m.Formulas {
		if f.Group != "" && !seen[f.Group] {
			seen[f.Group] = true
			out = append(out, f.Group)
		}
	}
	return out
}

// Names — все имена, доступные в выражениях (для подсказок в интерфейсе).
func (m *Model) Names() []Var {
	var out []Var
	for _, v := range SystemVars {
		out = append(out, v)
	}
	for _, p := range m.Params {
		out = append(out, Var{p.ID, p.Title})
	}
	for _, f := range m.Formulas {
		out = append(out, Var{f.ID, f.Title})
	}
	return out
}

// ValidID проверяет, что имя можно использовать в выражениях.
func ValidID(id string) error {
	if id == "" {
		return fmt.Errorf("имя не может быть пустым")
	}
	rs := []rune(id)
	if !unicode.IsLetter(rs[0]) && rs[0] != '_' {
		return fmt.Errorf("имя должно начинаться с буквы или «_»")
	}
	for _, r := range rs {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return fmt.Errorf("недопустимый символ «%c» в имени (разрешены буквы, цифры и «_», без пробелов)", r)
		}
	}
	for _, v := range SystemVars {
		if v.ID == id {
			return fmt.Errorf("имя зарезервировано программой")
		}
	}
	if _, ok := funcs[id]; ok {
		return fmt.Errorf("имя совпадает с функцией")
	}
	return nil
}

func levenshtein(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func suggest(name string, cands []string) string {
	best, bd := "", 1<<30
	ln := []rune(strings.ToLower(name))
	for _, c := range cands {
		if d := levenshtein(ln, []rune(strings.ToLower(c))); d < bd {
			best, bd = c, d
		}
	}
	if best != "" && bd <= 2 && bd < len(ln) {
		return best
	}
	return ""
}

// ───────────────────────────── проверка ─────────────────────────────

type analysis struct {
	exprs    []*Expr // по индексам формул; nil — не разобрана
	problems []Problem
	order    []int // порядок вычисления (индексы формул)
}

func (m *Model) analyze() analysis {
	var a analysis
	add := func(kind, id, msg string) {
		a.problems = append(a.problems, Problem{Kind: kind, ID: id, Msg: msg})
	}

	kind := map[string]string{}
	for _, v := range SystemVars {
		kind[v.ID] = "system"
	}
	for _, p := range m.Params {
		if err := ValidID(p.ID); err != nil {
			add("param", p.ID, err.Error())
			continue
		}
		if _, dup := kind[p.ID]; dup {
			add("param", p.ID, "это имя уже используется")
			continue
		}
		kind[p.ID] = "param"
		if math.IsNaN(p.Value) || math.IsInf(p.Value, 0) {
			add("param", p.ID, "значение должно быть конечным числом")
		}
	}

	valid := make([]bool, len(m.Formulas))
	for i, f := range m.Formulas {
		if err := ValidID(f.ID); err != nil {
			add("formula", f.ID, err.Error())
			continue
		}
		if _, dup := kind[f.ID]; dup {
			add("formula", f.ID, "это имя уже используется")
			continue
		}
		kind[f.ID] = "formula"
		valid[i] = true
	}

	var cands []string
	for id := range kind {
		cands = append(cands, id)
	}
	sort.Strings(cands)

	a.exprs = make([]*Expr, len(m.Formulas))
	deps := make([][]int, len(m.Formulas)) // от каких формул зависит формула i
	for i, f := range m.Formulas {
		if !valid[i] {
			continue
		}
		e, err := ParseExpr(f.Expr)
		if err != nil {
			add("formula", f.ID, err.Error())
			continue
		}
		a.exprs[i] = e
		seen := map[int]bool{}
		for _, name := range e.Idents() {
			k, ok := kind[name]
			if !ok {
				msg := fmt.Sprintf("неизвестное имя «%s»", name)
				if s := suggest(name, cands); s != "" {
					msg += fmt.Sprintf(". Возможно, имелось в виду «%s»", s)
				}
				add("formula", f.ID, msg)
				continue
			}
			if k != "formula" {
				continue
			}
			j := m.FindFormula(name)
			if j == i {
				add("formula", f.ID, "формула ссылается сама на себя")
				continue
			}
			if tg := m.Formulas[j].Group; tg != "" && tg != f.Group {
				add("formula", f.ID, fmt.Sprintf("«%s» входит в группу «%s» и недоступна вне этой группы", name, tg))
				continue
			}
			if !seen[j] {
				seen[j] = true
				deps[i] = append(deps[i], j)
			}
		}
	}

	// порядок вычисления: устойчивая топологическая сортировка
	done := make([]bool, len(m.Formulas))
	for i := range m.Formulas {
		if a.exprs[i] == nil {
			done[i] = true // испорченные формулы в порядок не входят
		}
	}
	for {
		progress := false
		for i := range m.Formulas {
			if done[i] {
				continue
			}
			ready := true
			for _, j := range deps[i] {
				if !done[j] {
					ready = false
					break
				}
			}
			if ready {
				done[i] = true
				a.order = append(a.order, i)
				progress = true
				break // после каждого шага начинаем с наименьшего индекса
			}
		}
		if !progress {
			break
		}
	}

	// остались формулы, которые не удалось упорядочить: ищем цикл
	inOrder := map[int]bool{}
	for _, i := range a.order {
		inOrder[i] = true
	}
	var rest []int
	for i := range m.Formulas {
		if a.exprs[i] != nil && !inOrder[i] {
			rest = append(rest, i)
		}
	}
	if len(rest) > 0 {
		inRest := map[int]bool{}
		for _, i := range rest {
			inRest[i] = true
		}
		cur := rest[0]
		path := []int{}
		pos := map[int]int{}
		for {
			if p, ok := pos[cur]; ok {
				path = path[p:]
				break
			}
			pos[cur] = len(path)
			path = append(path, cur)
			next := -1
			for _, j := range deps[cur] {
				if inRest[j] {
					next = j
					break
				}
			}
			if next < 0 {
				break
			}
			cur = next
		}
		if len(path) == 0 {
			path = rest[:1]
		}
		var names []string
		for _, i := range path {
			names = append(names, m.Formulas[i].ID)
		}
		names = append(names, names[0])
		add("formula", m.Formulas[path[0]].ID, "циклическая зависимость: "+strings.Join(names, " → "))
		a.order = nil // с циклом считать нельзя
	}
	return a
}

// Validate проверяет модель целиком. Пустой результат — модель корректна.
func (m *Model) Validate() []Problem { return m.analyze().problems }

// ───────────────────────────── расчёт ─────────────────────────────

// Inputs — исходные данные расчёта.
type Inputs struct {
	Mc, SumMW float64
	Cnt       int
	Params    map[string]float64 // значения параметров; отсутствующие берутся по умолчанию
	Groups    map[string]bool    // включённые группы формул
}

// Step — один вычисленный шаг.
type Step struct {
	Formula  Formula
	Expanded string // выражение с подставленными числами
	Value    float64
}

// Result — результат расчёта.
type Result struct {
	Steps []Step
	Vars  map[string]float64
}

// Value возвращает значение вычисленной формулы.
func (r *Result) Value(id string) (float64, bool) {
	for _, s := range r.Steps {
		if s.Formula.ID == id {
			return s.Value, true
		}
	}
	return 0, false
}

// Calculate проверяет модель и считает формулы в порядке зависимостей.
func (m *Model) Calculate(in Inputs) (*Result, error) {
	a := m.analyze()
	if len(a.problems) > 0 {
		msg := a.problems[0].String()
		if n := len(a.problems) - 1; n > 0 {
			msg += fmt.Sprintf(" (и ещё ошибок: %d)", n)
		}
		return nil, fmt.Errorf("в модели есть ошибки: %s", msg)
	}

	vars := map[string]float64{"Mc": in.Mc, "SumMW": in.SumMW, "Cnt": float64(in.Cnt)}
	for _, p := range m.Params {
		if v, ok := in.Params[p.ID]; ok {
			vars[p.ID] = v
		} else {
			vars[p.ID] = p.Value
		}
	}

	res := &Result{Vars: vars}
	for _, i := range a.order {
		f := m.Formulas[i]
		if f.Group != "" && !in.Groups[f.Group] {
			continue
		}
		v, err := a.exprs[i].Eval(vars)
		if err != nil {
			return nil, fmt.Errorf("формула «%s»: %v (подстановка: %s)", f.ID, err, a.exprs[i].Substitute(vars))
		}
		vars[f.ID] = v
		res.Steps = append(res.Steps, Step{Formula: f, Expanded: a.exprs[i].Substitute(vars), Value: v})
	}
	return res, nil
}
