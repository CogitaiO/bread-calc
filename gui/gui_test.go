package gui

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"breadcalc/engine"
)

func newTestApp(t *testing.T, variant fyne.ThemeVariant) (*App, fyne.Window) {
	t.Helper()
	ta := test.NewApp()
	ta.Settings().SetTheme(NewThemeVariant(variant))
	w := ta.NewWindow("BreadCalc")
	a := New(w, engine.Default(), "") // path == "": на диск не пишем
	w.Resize(fyne.NewSize(1180, 760))
	return a, w
}

func previewText(rt *widget.RichText) string {
	var b strings.Builder
	for _, s := range rt.Segments {
		if ts, ok := s.(*widget.TextSegment); ok {
			b.WriteString(ts.Text)
		}
	}
	return b.String()
}

func TestStartsWithFirstPreset(t *testing.T) {
	a, _ := newTestApp(t, theme.VariantLight)
	if len(a.items) != 9 || !strings.Contains(a.name, "Злаковик") {
		t.Fatalf("стартовая рецептура: %q, позиций %d", a.name, len(a.items))
	}
	if !strings.Contains(a.totals.Text, "119.00") {
		t.Fatalf("итоги: %q", a.totals.Text)
	}
}

func TestCalculateShowsResult(t *testing.T) {
	a, _ := newTestApp(t, theme.VariantLight)
	a.calculate()
	if a.tabs.SelectedIndex() != 2 {
		t.Fatal("после расчёта должна открыться вкладка «Результат»")
	}
	for _, want := range []string{"168.14", "117.70", "ИТОГОВЫЙ ВЫХОД"} {
		if !strings.Contains(a.report, want) {
			t.Errorf("в отчёте нет %q", want)
		}
	}
}

func TestInvalidParameterBlocksCalculation(t *testing.T) {
	a, _ := newTestApp(t, theme.VariantLight)
	e := a.paramEntries["Wt"]
	old := a.values["Wt"]
	e.SetText("не число")
	a.calculate()
	if a.report != "" || a.tabs.SelectedIndex() != 1 || a.values["Wt"] != old {
		t.Fatal("расчёт с некорректным параметром не должен выполняться")
	}
	e.SetText("47,5")
	a.calculate()
	if a.report == "" || a.tabs.SelectedIndex() != 2 {
		t.Fatal("корректный параметр должен позволять расчёт")
	}
}

func TestResultInvalidatedWhenInputsChange(t *testing.T) {
	tests := []struct {
		name   string
		change func(*App)
	}{
		{"рецептура", func(a *App) { a.items[0].Mass++; a.afterRecipeChange() }},
		{"параметр", func(a *App) { a.paramEntries["Wt"].SetText("48") }},
		{"некорректный параметр", func(a *App) { a.paramEntries["Wt"].SetText("ошибка") }},
		{"модель", func(a *App) { a.apply(a.mdl.Clone(), "") }},
		{"шаблон", func(a *App) { a.loadPreset(1) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := newTestApp(t, theme.VariantLight)
			a.calculate()
			if a.report == "" {
				t.Fatal("нет исходного результата")
			}
			tc.change(a)
			if a.report != "" {
				t.Fatal("устаревший отчёт остался доступен")
			}
		})
	}
}

func TestUnsavedChangesConfirmation(t *testing.T) {
	a, _ := newTestApp(t, theme.VariantLight)
	if a.hasRecipeChanges() {
		t.Fatal("выбранный шаблон не должен считаться изменённым")
	}
	called := false
	if d := a.confirmDiscard(func() { called = true }, nil); d != nil || !called {
		t.Fatal("действие без изменений должно выполняться сразу")
	}
	a.paramEntries["Wt"].SetText("ошибка")
	if !a.hasRecipeChanges() {
		t.Fatal("некорректный ввод тоже является изменением")
	}
	called, cancelled := false, false
	d := a.confirmDiscard(func() { called = true }, func() { cancelled = true })
	if d == nil || called {
		t.Fatal("изменённые данные требуют подтверждения")
	}
	d.Hide()
	if !cancelled || called {
		t.Fatal("отмена не должна продолжать действие")
	}
	a.paramEntries["Wt"].SetText(a.baseline.params["Wt"])
	if a.hasRecipeChanges() {
		t.Fatal("возврат исходного значения должен убрать признак изменений")
	}
	a.items[0].Mass++
	if !a.hasRecipeChanges() {
		t.Fatal("изменение рецептуры не обнаружено")
	}
}

func TestEmptyRecipeDoesNotCalculate(t *testing.T) {
	a, _ := newTestApp(t, theme.VariantLight)
	a.newRecipe()
	a.calculate()
	if a.report != "" || a.tabs.SelectedIndex() == 2 {
		t.Fatal("пустая рецептура не должна считаться")
	}
}

func TestFormulaTypoBlocksSaving(t *testing.T) {
	a, _ := newTestApp(t, theme.VariantLight)
	ff := a.formulaDialog(-1)
	test.Type(ff.id, "Plan")
	test.Type(ff.expr, "Bres * Wtt")
	if txt := previewText(ff.preview); !strings.Contains(txt, "«Wt»") {
		t.Fatalf("нет подсказки про опечатку: %q", txt)
	}
	ff.dlg.Submit()
	if a.mdl.FindFormula("Plan") >= 0 {
		t.Fatal("формула с опечаткой сохранилась")
	}
}

func TestFormulaSavedAndUsed(t *testing.T) {
	a, _ := newTestApp(t, theme.VariantLight)
	ff := a.formulaDialog(-1)
	test.Type(ff.id, "Plan")
	test.Type(ff.expr, "Bres * 1.05")
	if txt := previewText(ff.preview); !strings.Contains(txt, "176.54") {
		t.Fatalf("предпросмотр: %q", txt)
	}
	ff.dlg.Submit()
	if a.mdl.FindFormula("Plan") < 0 {
		t.Fatal("корректная формула не сохранилась")
	}
	a.calculate()
	if !strings.Contains(a.report, "Plan = 176.54") {
		t.Fatalf("формула не попала в отчёт:\n%s", a.report)
	}
}

func TestParamSavedAndUsedInFormula(t *testing.T) {
	a, _ := newTestApp(t, theme.VariantLight)
	pf := a.paramDialog(-1)
	test.Type(pf.id, "Kz")
	test.Type(pf.title, "Коэффициент запаса")
	test.Type(pf.value, "1,05")
	pf.dlg.Submit()
	if a.mdl.FindParam("Kz") < 0 || a.values["Kz"] != 1.05 {
		t.Fatal("параметр не создан")
	}
	if a.paramEntries["Kz"] == nil {
		t.Fatal("параметр не появился на вкладке «Параметры»")
	}
	ff := a.formulaDialog(-1)
	test.Type(ff.id, "Plan")
	test.Type(ff.expr, "Bres * Kz")
	ff.dlg.Submit()
	if a.mdl.FindFormula("Plan") < 0 {
		t.Fatal("формула с новым параметром не сохранилась")
	}
}

func TestUsedItemCannotBeDeleted(t *testing.T) {
	a, _ := newTestApp(t, theme.VariantLight)
	a.pSel = a.mdl.FindParam("GUp")
	a.deleteParam() // покажет «Нельзя удалить» и ничего не изменит
	if a.mdl.FindParam("GUp") < 0 {
		t.Fatal("используемый параметр удалился")
	}
}

// Скриншоты: BREADCALC_SNAPSHOTS=/путь go test ./gui -run TestSnapshots
func TestSnapshots(t *testing.T) {
	dir := os.Getenv("BREADCALC_SNAPSHOTS")
	if dir == "" {
		t.Skip("BREADCALC_SNAPSHOTS не задан")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	shot := func(w fyne.Window, name string) {
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, w.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []struct {
		name    string
		variant fyne.ThemeVariant
	}{{"light", theme.VariantLight}, {"dark", theme.VariantDark}} {
		a, w := newTestApp(t, v.variant)
		shot(w, v.name+"-1-recipe")
		a.tabs.SelectIndex(1)
		shot(w, v.name+"-2-params")
		a.calculate()
		shot(w, v.name+"-3-result")
		a.tabs.SelectIndex(3)
		shot(w, v.name+"-4-formulas")
		a.modelTabs.SelectIndex(1)
		shot(w, v.name+"-5-paramlist")
		a.modelTabs.SelectIndex(0)

		ff := a.formulaDialog(-1)
		test.Type(ff.id, "Plan")
		test.Type(ff.title, "План выпуска")
		test.Type(ff.expr, "Bres * Wtt")
		shot(w, v.name+"-6-formula-typo")
		ff.expr.SetText("Bres * 1.05")
		test.Type(ff.unit, "кг")
		shot(w, v.name+"-7-formula-ok")
	}
}
