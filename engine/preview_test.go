package engine

import (
	"strings"
	"testing"
)

func zlakovikInputs() Inputs {
	g := goldens[0]
	return Inputs{Mc: g.mc, SumMW: g.summw, Cnt: g.cnt,
		Params: map[string]float64{"Wt": g.wt, "Wm": g.wm, "GRazd": g.rz},
		Groups: map[string]bool{"Сухарики": true}}
}

func TestPreviewFormulaTypo(t *testing.T) {
	p := Default().PreviewFormula(Formula{ID: "X", Expr: "Bres * Wtt"}, "", zlakovikInputs(), true)
	if p.OK || !strings.Contains(p.Message, "«Wt»") {
		t.Fatalf("опечатка не поймана: %+v", p)
	}
}

func TestPreviewFormulaValue(t *testing.T) {
	p := Default().PreviewFormula(Formula{ID: "Plan", Expr: "Bres * 1.05"}, "", zlakovikInputs(), true)
	if !p.OK || !p.HasValue || !near(p.Value, goldens[0].bres*1.05) {
		t.Fatalf("значение: %+v", p)
	}
	if !strings.Contains(p.Expanded, "* 1.05") {
		t.Fatalf("подстановка: %q", p.Expanded)
	}
}

func TestPreviewWithoutRecipe(t *testing.T) {
	p := Default().PreviewFormula(Formula{ID: "Plan", Expr: "Bres * 1.05"}, "", Inputs{}, false)
	if !p.OK || p.HasValue {
		t.Fatalf("без рецептуры: %+v", p)
	}
}

func TestPreviewDivisionByZero(t *testing.T) {
	p := Default().PreviewFormula(Formula{ID: "D", Expr: "1 / (Mc - Mc)"}, "", zlakovikInputs(), true)
	if p.OK || !strings.Contains(p.Message, "деление на ноль") {
		t.Fatalf("деление на ноль: %+v", p)
	}
}

func TestPreviewIgnoresOldProblems(t *testing.T) {
	m := Default().WithFormula(Formula{ID: "Bad", Expr: "Nope + 1"}, "")
	p := m.PreviewFormula(Formula{ID: "Plan", Expr: "Bres * 2"}, "", zlakovikInputs(), true)
	if !p.OK {
		t.Fatalf("старая ошибка не должна блокировать новую формулу: %+v", p)
	}
}

func TestPreviewParam(t *testing.T) {
	m := Default()
	p := m.PreviewParam(Param{ID: "Kz", Value: 1.05}, "", zlakovikInputs(), true)
	if !p.OK || !strings.Contains(p.Message, "Bres =") {
		t.Fatalf("параметр: %+v", p)
	}
	bad := m.PreviewParam(Param{ID: "bad name", Value: 1}, "", zlakovikInputs(), true)
	if bad.OK {
		t.Fatalf("плохое имя параметра принято: %+v", bad)
	}
	// изменение значения существующего параметра меняет итог
	p2 := m.PreviewParam(Param{ID: "GUp", Value: 20}, "GUp", zlakovikInputs(), true)
	if !p2.OK || p2.Message == p.Message {
		t.Fatalf("итоги не изменились: %q vs %q", p2.Message, p.Message)
	}
}
