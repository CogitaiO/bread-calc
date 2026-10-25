package engine

import (
	"math"
	"os"
	"strings"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// ── выражения ──

func TestExprValues(t *testing.T) {
	vars := map[string]float64{"x": 4, "Влажность": 12.5}
	cases := []struct {
		src  string
		want float64
	}{
		{"1 + 2 * 3", 7},
		{"(1 + 2) * 3", 9},
		{"10 / 4", 2.5},
		{"2 ^ 3 ^ 2", 512},
		{"-2 ^ 2", -4},
		{"2 ^ -1", 0.5},
		{"x * 2 - 1", 7},
		{"Влажность * 2", 25},
		{"max(1, 7; 3)", 7},
		{"min(x, 2)", 2},
		{"sqrt(x)", 2},
		{"abs(-3)", 3},
		{"round(2.5)", 3},
		{"if(x > 3, 10, 20)", 10},
		{"if(x > 5, 1 / 0, 20)", 20}, // ленивая ветка: деления на ноль нет
		{"3 × 2 − 1", 5},             // знаки из методички
		{"x == 4", 1},
		{"x != 4", 0},
	}
	for _, c := range cases {
		e, err := ParseExpr(c.src)
		if err != nil {
			t.Errorf("%q: разбор: %v", c.src, err)
			continue
		}
		got, err := e.Eval(vars)
		if err != nil || !near(got, c.want) {
			t.Errorf("%q = %v (err %v), ожидалось %v", c.src, got, err, c.want)
		}
	}
}

func TestExprSyntaxErrors(t *testing.T) {
	cases := map[string]string{
		"":          "пустое",
		"2 +":       "обрывается",
		"(1 + 2":    "не закрыта скобка",
		"1 + 2)":    "лишняя",
		"foo(1)":    "неизвестная функция",
		"max(1)":    "принимает",
		"1 % 2":     "%",
		"0,95 * 2":  "десятичный разделитель",
		"2x":        "знак операции",
		"a = 1":     "==",
		"1 < 2 < 3": "цепочки",
		"1 2":       "знак операции",
		"[1]":       "круглые скобки",
		"if(1, 2)":  "принимает",
		"max(1, 2":  "не закрыта скобка функции",
		"1 $ 2":     "недопустимый символ",
	}
	for src, frag := range cases {
		_, err := ParseExpr(src)
		if err == nil {
			t.Errorf("%q: ожидалась ошибка", src)
			continue
		}
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("%q: ошибка %q не содержит %q", src, err, frag)
		}
	}
}

func TestExprRuntimeErrors(t *testing.T) {
	for _, src := range []string{"1 / 0", "sqrt(0 - 1)", "ln(0)", "10 ^ 1000", "x"} {
		e, err := ParseExpr(src)
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if _, err := e.Eval(map[string]float64{}); err == nil {
			t.Errorf("%q: ожидалась ошибка вычисления", src)
		}
	}
}

func TestSubstitute(t *testing.T) {
	e, _ := ParseExpr("Mc * (100 - Wcp) / x")
	got := e.Substitute(map[string]float64{"Mc": 119, "Wcp": 12.0504201680, "x": -2})
	if got != "119 * (100 - 12.0504) / (-2)" {
		t.Errorf("подстановка: %q", got)
	}
}

// ── модель ──

func TestDefaultModelValid(t *testing.T) {
	if p := Default().Validate(); len(p) != 0 {
		t.Fatalf("встроенная модель некорректна: %v", p)
	}
}

type golden struct {
	name       string
	mc, summw  float64
	cnt        int
	wt, wm, rz float64
	wcp, bt    float64
	losses     float64
	bres, suh  float64
}

// Эталон: значения посчитаны исходным скриптом TEST.py.
var goldens = []golden{
	{"Злаковик", 119.0, 1434.0, 9, 47.5, 14.5, 0.0, 12.050420168067227, 199.35238095238094, 31.214692352765688, 168.13768859961525, 117.69638201973066},
	{"Томатная паста", 134.0, 2581.8, 10, 47.5, 14.5, 0.0, 19.26716417910448, 206.06095238095236, 32.26380652031423, 173.79714586063812, 121.65800210244667},
	{"Лепёшка майская", 125.17, 1826.41, 5, 36.5, 13.0, 0.6, 14.591435647519374, 168.35574803149606, 26.55222105095487, 141.8035269805412, 99.26246888637883},
	{"Лепёшка зерновая", 125.17, 1766.41, 6, 37.5, 13.0, 0.6, 14.112087560917153, 172.00943999999998, 27.13514718284433, 144.87429281715566, 101.41200497200896},
	{"Лепёшка облепиховая", 128.17, 1838.41, 6, 36.5, 13.0, 0.6, 14.343528126706719, 172.89118110236217, 27.260503752444972, 145.6306773499172, 101.94147414494203},
}

func TestGoldenAgainstTestPy(t *testing.T) {
	m := Default()
	for _, g := range goldens {
		in := Inputs{
			Mc: g.mc, SumMW: g.summw, Cnt: g.cnt,
			Params: map[string]float64{"Wt": g.wt, "Wm": g.wm, "GRazd": g.rz},
			Groups: map[string]bool{"Сухарики": true},
		}
		res, err := m.Calculate(in)
		if err != nil {
			t.Fatalf("%s: %v", g.name, err)
		}
		for id, want := range map[string]float64{"Wcp": g.wcp, "Bt": g.bt, "Losses": g.losses, "Bres": g.bres, "SuhYield": g.suh} {
			got, ok := res.Value(id)
			if !ok || math.Abs(got-want) > 1e-9 {
				t.Errorf("%s: %s = %v, эталон %v", g.name, id, got, want)
			}
		}
		// без группы «Сухарики» этих шагов быть не должно
		in.Groups = nil
		res, _ = m.Calculate(in)
		if _, ok := res.Value("SuhYield"); ok {
			t.Errorf("%s: сухарики посчитаны при выключенной группе", g.name)
		}
	}
}

func hasProblem(ps []Problem, id, frag string) bool {
	for _, p := range ps {
		if p.ID == id && strings.Contains(p.Msg, frag) {
			return true
		}
	}
	return false
}

func TestValidateTypoWithSuggestion(t *testing.T) {
	m := Default().WithFormula(Formula{ID: "X", Expr: "Wtt * 2", Unit: "%"}, "")
	ps := m.Validate()
	if !hasProblem(ps, "X", "неизвестное имя «Wtt»") || !hasProblem(ps, "X", "«Wt»") {
		t.Fatalf("нет подсказки про опечатку: %v", ps)
	}
	if _, err := m.Calculate(Inputs{Mc: 100, SumMW: 1200}); err == nil {
		t.Fatal("расчёт не должен идти с ошибочной формулой")
	}
}

func TestValidateCaseTypo(t *testing.T) {
	m := Default().WithFormula(Formula{ID: "X", Expr: "gup + 1"}, "")
	if ps := m.Validate(); !hasProblem(ps, "X", "«GUp»") {
		t.Fatalf("нет подсказки про регистр: %v", ps)
	}
}

func TestValidateCycle(t *testing.T) {
	m := &Model{Formulas: []Formula{
		{ID: "A", Expr: "B + 1"},
		{ID: "B", Expr: "C + 1"},
		{ID: "C", Expr: "A + 1"},
	}}
	ps := m.Validate()
	if !hasProblem(ps, "A", "циклическая") {
		t.Fatalf("цикл не найден: %v", ps)
	}
	self := &Model{Formulas: []Formula{{ID: "A", Expr: "A + 1"}}}
	if !hasProblem(self.Validate(), "A", "сама на себя") {
		t.Fatal("самоссылка не найдена")
	}
}

func TestValidateNamesAndDuplicates(t *testing.T) {
	m := &Model{
		Params:   []Param{{ID: "p", Value: 1}, {ID: "p", Value: 2}, {ID: "Mc", Value: 1}, {ID: "bad name", Value: 1}, {ID: "9x", Value: 1}},
		Formulas: []Formula{{ID: "p", Expr: "1"}, {ID: "max", Expr: "1"}},
	}
	ps := m.Validate()
	for _, want := range [][2]string{{"p", "уже используется"}, {"Mc", "зарезервировано"}, {"bad name", "недопустимый символ"}, {"9x", "начинаться"}, {"max", "функци"}} {
		if !hasProblem(ps, want[0], want[1]) {
			t.Errorf("нет проблемы %v в %v", want, ps)
		}
	}
}

func TestValidateGroupRule(t *testing.T) {
	m := &Model{Formulas: []Formula{
		{ID: "G", Expr: "1", Group: "Сухарики"},
		{ID: "Out", Expr: "G + 1"}, // без группы ссылается на формулу из группы
		{ID: "In", Expr: "G + 1", Group: "Сухарики"},
	}}
	ps := m.Validate()
	if !hasProblem(ps, "Out", "входит в группу") {
		t.Fatalf("ссылка из-вне группы не поймана: %v", ps)
	}
	if hasProblem(ps, "In", "группу") {
		t.Fatalf("ссылка внутри группы ошибочна: %v", ps)
	}
}

func TestOrderIndependent(t *testing.T) {
	// формулы в «обратном» порядке считаются так же
	m := &Model{
		Params: []Param{{ID: "k", Value: 3}},
		Formulas: []Formula{
			{ID: "C", Expr: "B * 2"},
			{ID: "B", Expr: "A + k"},
			{ID: "A", Expr: "Mc"},
		},
	}
	res, err := m.Calculate(Inputs{Mc: 5})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := res.Value("C"); !near(v, 16) {
		t.Fatalf("C = %v, ожидалось 16", v)
	}
	if res.Steps[0].Formula.ID != "A" {
		t.Fatalf("порядок вычисления неверный: %v", res.Steps[0].Formula.ID)
	}
}

func TestRuntimeErrorMentionsFormula(t *testing.T) {
	m := &Model{Formulas: []Formula{{ID: "D", Expr: "10 / (Mc - 5)"}}}
	_, err := m.Calculate(Inputs{Mc: 5})
	if err == nil || !strings.Contains(err.Error(), "«D»") || !strings.Contains(err.Error(), "деление на ноль") {
		t.Fatalf("неинформативная ошибка: %v", err)
	}
}

func TestCustomParamAndFormula(t *testing.T) {
	m := Default().
		WithParam(Param{ID: "Kz", Title: "Коэффициент запаса", Value: 1.05}, "").
		WithFormula(Formula{ID: "Plan", Expr: "Bres * Kz", Unit: "кг", Final: true}, "")
	in := goldens[0]
	res, err := m.Calculate(Inputs{Mc: in.mc, SumMW: in.summw, Params: map[string]float64{"Kz": 2}})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := res.Value("Plan"); !near(v, in.bres*2) {
		t.Fatalf("Plan = %v", v)
	}
}

func TestUsedByAndRemoval(t *testing.T) {
	m := Default()
	if u := m.UsedBy("GUp"); len(u) != 1 || u[0] != "Zup" {
		t.Fatalf("UsedBy(GUp) = %v", u)
	}
	// удаление используемого параметра делает модель некорректной
	if ps := m.WithoutParam("GUp").Validate(); !hasProblem(ps, "Zup", "неизвестное имя") {
		t.Fatalf("удаление параметра не поймано: %v", ps)
	}
}

// ── файл ──

func TestFormatParseRoundTrip(t *testing.T) {
	m := Default().
		WithParam(Param{ID: "Kz", Title: "Запас", Unit: "", Note: "тест", Value: 1.05}, "").
		WithFormula(Formula{ID: "Plan", Title: "План", Expr: "Bres*Kz", Unit: "кг", Ref: "(99)", Group: "Сухарики", Final: true}, "")
	text := Format(m)
	back, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	if Format(back) != text {
		t.Fatal("после чтения и записи файл изменился")
	}
	if len(back.Params) != len(m.Params) || len(back.Formulas) != len(m.Formulas) {
		t.Fatal("потеряны строки")
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"param Wt 47.5":             "строка 1",
		"param Wt = abc":            "некорректное число",
		"formula A = 1 | | | foo=1": "неизвестное свойство",
		"что-то = 1":                "param или formula",
		"# ok\n\nparam A = 1\nbad":  "строка 4",
	}
	for text, frag := range cases {
		if _, err := Parse(text); err == nil || !strings.Contains(err.Error(), frag) {
			t.Errorf("%q: ошибка %v, ожидалось «%s»", text, err, frag)
		}
	}
	// десятичная запятая в параметре допустима
	m, err := Parse("param A = 0,95 | | A")
	if err != nil || !near(m.Params[0].Value, 0.95) {
		t.Fatalf("запятая: %v %v", m, err)
	}
}

func TestSaveLoadBackup(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/sub/model.txt"
	if _, err := LoadOrCreate(path); err != nil { // создаёт файл
		t.Fatal(err)
	}
	m := Default().WithParam(Param{ID: "Kz", Value: 2}, "")
	if err := Save(path, m); err != nil {
		t.Fatal(err)
	}
	back, err := LoadOrCreate(path)
	if err != nil || back.FindParam("Kz") < 0 {
		t.Fatalf("не прочитали сохранённое: %v", err)
	}
	if err := Save(path, Default()); err != nil {
		t.Fatal(err)
	}
	old, _ := Parse(readFile(t, path+".bak"))
	if old.FindParam("Kz") < 0 {
		t.Fatal("в .bak не предыдущая версия")
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := osReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var osReadFile = os.ReadFile
