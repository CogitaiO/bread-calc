package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"breadcalc/engine"

	tea "github.com/charmbracelet/bubbletea"
)

// isolate не даёт тестам трогать настоящие настройки пользователя.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	t.Setenv("HOME", dir)
	cfg, _ := os.UserConfigDir()
	return filepath.Join(cfg, "breadcalc", "model.txt")
}

func key(s string) tea.Msg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestRecipeSaveOpenAndCompatibility(t *testing.T) {
	isolate(t)
	m := openPreset(t, 0)
	m.recipe[0].Mass = 7.5
	m.values["Wt"] = 48.5
	m.name = "Своя рецептура"
	m = send(m, "s")
	if m.screen != scFile {
		t.Fatal("сохранение не открыто")
	}
	m.fileInput.SetValue("custom.json")
	m = send(m, "enter")
	if m.screen != scRecipe || m.hasChanges() {
		t.Fatalf("не сохранено: %s", m.msg)
	}
	m.recipe[0].Mass = 1
	m = send(m, "o")
	if m.screen != scConfirm {
		t.Fatal("нет предупреждения об изменениях")
	}
	m = send(m, "y")
	if m.screen != scFile {
		t.Fatal("не открылся ввод пути")
	}
	m.fileInput.SetValue("custom.json")
	m = send(m, "enter")
	if m.screen != scRecipe || m.name != "Своя рецептура" || m.recipe[0].Mass != 7.5 || m.values["Wt"] != 48.5 || !m.groups["Сухарики"] || m.hasChanges() {
		t.Fatalf("рецептура не восстановлена: %s", m.msg)
	}
	m.mdl = m.mdl.WithoutParam("Wt")
	if err := m.loadRecipeFile("custom.json"); err == nil {
		t.Fatal("несовместимая модель принята")
	}
	if m.recipe[0].Mass != 7.5 {
		t.Fatal("ошибка открытия изменила рецептуру")
	}
}

func TestDiscardAndOverwriteConfirmation(t *testing.T) {
	isolate(t)
	m := openPreset(t, 0)
	m.recipe[0].Mass++
	m = send(m, "q")
	if m.screen != scConfirm {
		t.Fatal("выход без подтверждения")
	}
	m = send(m, "n")
	if m.screen != scRecipe || !m.hasChanges() {
		t.Fatal("отмена выхода потеряла данные")
	}
	if err := os.WriteFile("existing.json", []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	m = send(m, "s")
	m.fileInput.SetValue("existing.json")
	m = send(m, "enter")
	if m.screen != scConfirm {
		t.Fatal("нет подтверждения перезаписи")
	}
	m = send(m, "n")
	if b, _ := os.ReadFile("existing.json"); string(b) != "original" {
		t.Fatal("файл изменён после отмены")
	}
	m = send(m, "enter", "y")
	if m.screen != scRecipe || m.hasChanges() {
		t.Fatalf("перезапись не выполнена: %s", m.msg)
	}
	if err := m.loadRecipeFile("existing.json"); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidParameterCannotBeDiscardedByEscape(t *testing.T) {
	isolate(t)
	m := send(openPreset(t, 0), "p")
	m.pRows[0].input.SetValue("не число")
	m = send(m, "esc")
	if m.screen != scParams || !m.isErr {
		t.Fatal("ошибка параметра должна оставаться на экране")
	}
}

func TestResultInvalidatedAndReportKeepsNames(t *testing.T) {
	isolate(t)
	m := openPreset(t, 0)
	long := strings.Repeat("Длинное название ", 5)
	m.recipe[0].Name = long
	m = send(m, "c")
	if !strings.Contains(m.report, long) {
		t.Fatal("название обрезано")
	}
	m = send(m, "esc", "d", "d")
	if m.report != "" {
		t.Fatal("старый результат не сброшен")
	}
}

func TestPrintableReportEscapesUserText(t *testing.T) {
	isolate(t)
	m := openPreset(t, 0)
	m.name = "<script>test</script>"
	m = send(m, "c", "h")
	files, err := filepath.Glob("raschet_*.html")
	if err != nil || len(files) != 1 {
		t.Fatalf("файл HTML не создан: %v %v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil || strings.Contains(string(data), "<script>") || !strings.Contains(string(data), "&lt;script&gt;") {
		t.Fatalf("небезопасный HTML: %v", err)
	}
}

func TestChangedRecipeMenuSelectionNeedsConfirmation(t *testing.T) {
	isolate(t)
	m := openPreset(t, 0)
	m.recipe[0].Mass++
	m = send(m, "esc", "down", "enter")
	if m.screen != scConfirm {
		t.Fatal("смена шаблона без подтверждения")
	}
	m = send(m, "n")
	if m.screen != scMenu || m.recipe[0].Mass != 3 {
		t.Fatal("отмена потеряла изменения")
	}
	m = send(m, "enter", "y")
	if m.screen != scRecipe || !strings.Contains(m.name, "томатной") || m.hasChanges() {
		t.Fatal("шаблон не выбран после подтверждения")
	}
}

func send(m model, keys ...string) model {
	for _, k := range keys {
		nm, _ := m.Update(key(k))
		m = nm.(model)
	}
	return m
}

func openPreset(t *testing.T, idx int) model {
	t.Helper()
	m := initialModel()
	for i := 0; i < idx; i++ {
		m = send(m, "down")
	}
	m = send(m, "enter")
	if m.screen != scRecipe {
		t.Fatalf("после выбора рецептуры экран %v", m.screen)
	}
	return m
}

func TestCalculatePreset(t *testing.T) {
	isolate(t)
	m := send(openPreset(t, 0), "c")
	if m.screen != scResult {
		t.Fatalf("расчёт не открылся: %q", m.msg)
	}
	for _, want := range []string{"168.14", "117.70", "ИТОГОВЫЙ ВЫХОД ИЗДЕЛИЯ", "ИТОГОВЫЙ ВЫХОД СУХАРИКОВ"} {
		if !strings.Contains(m.report, want) {
			t.Errorf("в отчёте нет %q", want)
		}
	}
	// лепёшка: сухарики по умолчанию выключены
	m = send(openPreset(t, 2), "c")
	if strings.Contains(m.report, "ИТОГОВЫЙ ВЫХОД СУХАРИКОВ") {
		t.Error("у лепёшки не должно быть сухариков")
	}
	if !strings.Contains(m.report, "141.80") {
		t.Error("выход лепёшки «Майская» должен быть 141.80")
	}
}

func TestToggleGroupOnParamsScreen(t *testing.T) {
	isolate(t)
	m := openPreset(t, 2)
	m = send(m, "p")
	if m.screen != scParams {
		t.Fatal("экран параметров не открылся")
	}
	for i := 0; i < len(m.pRows)-1; i++ {
		m = send(m, "down")
	}
	m = send(m, " ", "enter") // включить «Сухарики» и рассчитать
	if m.screen != scResult || !strings.Contains(m.report, "ИТОГОВЫЙ ВЫХОД СУХАРИКОВ") {
		t.Fatalf("сухарики не включились: %q", m.msg)
	}
}

func TestAddFormulaTypoBlockedThenSaved(t *testing.T) {
	path := isolate(t)
	m := openPreset(t, 0)
	m = send(m, "f", "a")
	if m.screen != scDef {
		t.Fatal("форма формулы не открылась")
	}
	m = send(m, "Plan", "enter", "План выпуска", "enter", "Bres * Wtt")
	if txt, ok := m.preview(); ok || !strings.Contains(txt, "«Wt»") {
		t.Fatalf("опечатка не поймана или нет подсказки: ok=%v %q", ok, txt)
	}
	m = send(m, "ctrl+s")
	if m.screen != scDef || !strings.Contains(m.msg, "Не сохранено") {
		t.Fatalf("формула с опечаткой не должна сохраняться: screen=%v msg=%q", m.screen, m.msg)
	}
	if m.mdl.FindFormula("Plan") >= 0 {
		t.Fatal("плохая формула попала в модель")
	}

	// исправляем выражение: стираем 3 символа и дописываем верные
	for i := 0; i < len("Wtt"); i++ {
		m = send(m, "backspace")
	}
	m = send(m, "1.05")
	if txt, ok := m.preview(); !ok || !strings.Contains(txt, "176.54") {
		t.Fatalf("предпросмотр: ok=%v %q", ok, txt)
	}
	m = send(m, "ctrl+s")
	if m.screen != scFormulas || m.mdl.FindFormula("Plan") < 0 {
		t.Fatalf("не сохранилось: screen=%v msg=%q", m.screen, m.msg)
	}

	// формула записана в файл и участвует в расчёте
	saved, err := engine.LoadOrCreate(path)
	if err != nil || saved.FindFormula("Plan") < 0 {
		t.Fatalf("в файле нет формулы: %v", err)
	}
	m = send(m, "esc", "c")
	if m.screen != scResult || !strings.Contains(m.report, "Plan = 176.544") {
		t.Fatalf("новая формула не попала в отчёт: %q", m.msg)
	}
}

func TestAddParamAndUseIt(t *testing.T) {
	isolate(t)
	m := openPreset(t, 0)
	m = send(m, "f", "tab", "a")
	if m.screen != scDef || m.fTab != 1 {
		t.Fatal("форма параметра не открылась")
	}
	m = send(m, "Kz", "enter", "Коэффициент запаса", "enter", "enter", "1.05", "enter")
	m = send(m, "enter") // последняя строка — сохранить
	if m.mdl.FindParam("Kz") < 0 || m.values["Kz"] != 1.05 {
		t.Fatalf("параметр не создан: msg=%q", m.msg)
	}
	// теперь формула может использовать новый параметр
	m = send(m, "tab", "a", "Plan", "enter", "enter", "Bres * Kz")
	m = send(m, "ctrl+s")
	if m.mdl.FindFormula("Plan") < 0 {
		t.Fatalf("формула с параметром не сохранилась: %q", m.msg)
	}
	// удалить параметр, который используется, нельзя
	m = send(m, "tab") // на вкладку параметров
	n := m.mdl.FindParam("Kz")
	for i := 0; i < n; i++ {
		m = send(m, "down")
	}
	m = send(m, "d")
	if !strings.Contains(m.msg, "Нельзя удалить") || m.mdl.FindParam("Kz") < 0 {
		t.Fatalf("удаление используемого параметра не заблокировано: %q", m.msg)
	}
}

func TestDeleteNeedsConfirmAndReset(t *testing.T) {
	isolate(t)
	m := openPreset(t, 0)
	m = send(m, "f", "tab", "a", "Tmp", "enter", "enter", "enter", "5", "enter", "enter")
	if m.mdl.FindParam("Tmp") < 0 {
		t.Fatalf("не создался: %q", m.msg)
	}
	for i := 0; i < m.mdl.FindParam("Tmp"); i++ {
		m = send(m, "down")
	}
	m = send(m, "d")
	if m.mdl.FindParam("Tmp") < 0 {
		t.Fatal("удалилось без подтверждения")
	}
	m = send(m, "d")
	if m.mdl.FindParam("Tmp") >= 0 {
		t.Fatal("не удалилось после подтверждения")
	}
	m = send(m, "r", "r")
	if len(m.mdl.Validate()) != 0 || len(m.mdl.Formulas) != len(engine.Default().Formulas) {
		t.Fatal("сброс к методичке не сработал")
	}
}

func TestBrokenModelFileDoesNotCrash(t *testing.T) {
	path := isolate(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	broken := "formula A = 1 + \nмусор\n"
	if err := os.WriteFile(path, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	m := initialModel()
	if !m.isErr || !strings.Contains(m.msg, "не прочитан") {
		t.Fatalf("нет предупреждения о битом файле: %q", m.msg)
	}
	m = send(openPreset(t, 0), "c")
	if m.screen != scResult {
		t.Fatalf("со встроенной моделью расчёт должен работать: %q", m.msg)
	}
	// файл не затёрт без действий пользователя
	if data, _ := os.ReadFile(path); string(data) != broken {
		t.Fatal("битый файл был перезаписан сам")
	}
}

func TestEmptyRecipeMessage(t *testing.T) {
	isolate(t)
	m := initialModel()
	for i := 0; i < len(presets()); i++ {
		m = send(m, "down")
	}
	m = send(m, "enter", "c") // «Новая рецептура» и расчёт
	if m.screen != scRecipe || !strings.Contains(m.msg, "пуста") {
		t.Fatalf("ожидалось сообщение о пустой рецептуре: screen=%v %q", m.screen, m.msg)
	}
}

func TestEveryScreenRenders(t *testing.T) {
	isolate(t)
	for _, size := range [][2]int{{120, 40}, {80, 24}, {40, 12}} {
		m := openPreset(t, 0)
		nm, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		m = nm.(model)
		views := map[string]model{"recipe": m}
		views["edit"] = send(m, "a")
		views["params"] = send(m, "p")
		views["result"] = send(m, "c")
		views["formulas"] = send(m, "f")
		views["formula-edit"] = send(m, "f", "e")
		views["param-list"] = send(m, "f", "tab")
		views["param-edit"] = send(m, "f", "tab", "e")
		views["menu"] = send(m, "esc")
		for name, v := range views {
			if out := v.View(); strings.TrimSpace(out) == "" {
				t.Errorf("%s (%dx%d): пустой экран", name, size[0], size[1])
			}
		}
	}
}
