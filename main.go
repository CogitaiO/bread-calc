package main

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"breadcalc/engine"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ───────────────────────────── стили ─────────────────────────────

var (
	cAccent = lipgloss.Color("#FFB86C")
	cCyan   = lipgloss.Color("#8BE9FD")
	cGreen  = lipgloss.Color("#50FA7B")
	cRed    = lipgloss.Color("#FF5555")
	cMuted  = lipgloss.Color("#6272A4")

	stBanner = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#282A36")).
			Background(cAccent).Padding(0, 2)
	stSub   = lipgloss.NewStyle().Foreground(cMuted)
	stTitle = lipgloss.NewStyle().Bold(true).Foreground(cCyan)
	stSel   = lipgloss.NewStyle().Bold(true).Foreground(cGreen)
	stHdr   = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	stErr   = lipgloss.NewStyle().Bold(true).Foreground(cRed)
	stOK    = lipgloss.NewStyle().Foreground(cGreen)
	stHelp  = lipgloss.NewStyle().Foreground(cMuted)
	stPanel = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cMuted).Padding(0, 1)
	ansiRe  = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// ───────────────────────────── меню ─────────────────────────────

type menuItem struct {
	title, desc string
	idx         int // ≥0 — готовая рецептура, -1 — новая, -2 — формулы и параметры
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

// ───────────────────────────── форма ─────────────────────────────

type row struct {
	label  string
	hint   string
	input  textinput.Model
	toggle bool
	on     bool
}

func newRow(label, hint, value string) row {
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 200
	ti.Width = 40
	ti.SetValue(value)
	return row{label: label, hint: hint, input: ti}
}

func newToggle(label, hint string, on bool) row {
	return row{label: label, hint: hint, toggle: true, on: on}
}

func focusRow(rows []row, idx int) {
	for i := range rows {
		rows[i].input.Blur()
	}
	if idx >= 0 && idx < len(rows) && !rows[idx].toggle {
		rows[idx].input.Focus()
	}
}

func fnum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func parseF(s string) (float64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("введите число")
	}
	return v, nil
}

// ───────────────────────────── модель ─────────────────────────────

type screen int

const (
	scMenu screen = iota
	scRecipe
	scEdit
	scParams
	scResult
	scFormulas
	scDef
)

type model struct {
	screen screen
	back   screen // куда вернуться с экрана результата
	w, h   int

	menu list.Model

	// рецептура
	name   string
	recipe []Ingredient
	cursor int

	// модель расчёта (формулы и параметры) и текущие значения
	mdl     *engine.Model
	mdlPath string
	values  map[string]float64
	groups  map[string]bool

	// редактор сырья
	editRows []row
	editIdx  int
	editNew  bool

	// экран параметров перед расчётом
	pRows  []row
	pFocus int

	// экран «Формулы и параметры»
	fTab    int // 0 — формулы, 1 — параметры
	fCur    [2]int
	fBack   screen
	confirm string
	dRows   []row
	dIdx    int
	dOld    string // прежнее имя редактируемого элемента ("" — новый)

	vp     viewport.Model
	report string

	msg   string
	isErr bool
}

func initialModel() model {
	items := []list.Item{}
	for i, p := range presets() {
		items = append(items, menuItem{p.Name, p.Desc, i})
	}
	items = append(items,
		menuItem{"＋ Новая рецептура", "ввести сырьё и параметры вручную", -1},
		menuItem{"⚙  Формулы и параметры", "добавить свои формулы и параметры, проверка выражений", -2},
	)

	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.Foreground(cGreen).BorderForeground(cGreen)
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.Foreground(cMuted).BorderForeground(cGreen)

	l := list.New(items, d, 80, 20)
	l.Title = "Выберите рецептуру"
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(cCyan)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	m := model{menu: l, vp: viewport.New(80, 20), w: 100, h: 30}

	m.mdlPath = engine.ModelPath()
	mdl, err := engine.LoadOrCreate(m.mdlPath)
	if err != nil {
		mdl = engine.Default()
		m.setMsg("Файл модели не прочитан ("+err.Error()+"). Используется встроенная модель; "+
			"при первом сохранении она заменит файл (старый останется в .bak).", true)
	} else if n := len(mdl.Validate()); n > 0 {
		m.setMsg(fmt.Sprintf("В файле модели есть ошибки (%d). Откройте «Формулы и параметры».", n), true)
	}
	m.mdl = mdl
	m.resetValues(nil, nil)
	return m
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m *model) setMsg(s string, isErr bool) { m.msg, m.isErr = s, isErr }

func (m *model) layout() {
	m.menu.SetSize(m.w-4, max(m.h-8, 6))
	m.vp.Width = m.w - 4
	m.vp.Height = max(m.h-9, 5)
}

// resetValues задаёт значения параметров по умолчанию с учётом рецептуры.
func (m *model) resetValues(over map[string]float64, groups map[string]bool) {
	m.values = map[string]float64{}
	for _, p := range m.mdl.Params {
		m.values[p.ID] = p.Value
	}
	for k, v := range over {
		if _, ok := m.values[k]; ok {
			m.values[k] = v
		}
	}
	m.groups = map[string]bool{}
	for k, v := range groups {
		m.groups[k] = v
	}
}

// syncValues согласует текущие значения с составом параметров модели.
func (m *model) syncValues() {
	if m.values == nil {
		m.values = map[string]float64{}
	}
	ids := map[string]bool{}
	for _, p := range m.mdl.Params {
		ids[p.ID] = true
		if _, ok := m.values[p.ID]; !ok {
			m.values[p.ID] = p.Value
		}
	}
	for id := range m.values {
		if !ids[id] {
			delete(m.values, id)
		}
	}
}

func (m model) inputs() engine.Inputs {
	mc, sum := aggregates(m.recipe)
	return engine.Inputs{Mc: mc, SumMW: sum, Cnt: len(m.recipe), Params: m.values, Groups: m.groups}
}

func (m *model) calculate() {
	if len(m.recipe) == 0 {
		m.setMsg("Рецептура пуста — добавьте хотя бы одно сырьё (клавиша a).", true)
		return
	}
	if mc, _ := aggregates(m.recipe); mc <= 0 {
		m.setMsg("Суммарная масса сырья должна быть больше нуля.", true)
		return
	}
	in := m.inputs()
	res, err := m.mdl.Calculate(in)
	if err != nil {
		m.setMsg(err.Error(), true)
		return
	}
	m.report = BuildReport(m.name, m.recipe, m.mdl, in, res)
	m.vp.SetContent(m.report)
	m.vp.GotoTop()
	m.setMsg("", false)
	m.screen = scResult
}

func (m *model) saveReport() {
	fn := "raschet_" + time.Now().Format("20060102_150405") + ".txt"
	plain := ansiRe.ReplaceAllString(m.report, "")
	if err := os.WriteFile(fn, []byte(plain), 0o644); err != nil {
		m.setMsg("Не удалось сохранить: "+err.Error(), true)
		return
	}
	m.setMsg("Отчёт сохранён в файл "+fn, false)
}

// ───────────────────────────── update ─────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.layout()
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	switch m.screen {
	case scMenu:
		return m.updateMenu(msg)
	case scRecipe:
		return m.updateRecipe(msg)
	case scEdit:
		return m.updateEdit(msg)
	case scParams:
		return m.updateParams(msg)
	case scFormulas:
		return m.updateFormulas(msg)
	case scDef:
		return m.updateDef(msg)
	default:
		return m.updateResult(msg)
	}
}

func (m model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "q":
			return m, tea.Quit
		case "enter":
			it, ok := m.menu.SelectedItem().(menuItem)
			if !ok {
				return m, nil
			}
			m.setMsg("", false)
			switch {
			case it.idx >= 0:
				p := presets()[it.idx]
				m.name = p.Name
				m.recipe = append([]Ingredient(nil), p.Recipe...)
				m.resetValues(p.Overrides, p.Groups)
				m.cursor = 0
				m.screen = scRecipe
			case it.idx == -1:
				m.name = "Новая рецептура"
				m.recipe = nil
				m.resetValues(nil, nil)
				m.cursor = 0
				m.screen = scRecipe
			default:
				m.fBack = scMenu
				m.screen = scFormulas
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.menu, cmd = m.menu.Update(msg)
	return m, cmd
}

func (m *model) openEdit(isNew bool) tea.Cmd {
	ing := Ingredient{}
	if !isNew && len(m.recipe) > 0 {
		ing = m.recipe[m.cursor]
	}
	m.editNew = isNew
	mass, wet := "", ""
	if !isNew {
		mass, wet = fnum(ing.Mass), fnum(ing.Moisture)
	}
	m.editRows = []row{
		newRow("Наименование сырья", "", ing.Name),
		newRow("Масса, кг", "", mass),
		newRow("Влажность, %", "", wet),
	}
	m.editRows[0].input.Width = 50
	m.editIdx = 0
	focusRow(m.editRows, 0)
	m.setMsg("", false)
	m.screen = scEdit
	return textinput.Blink
}

func (m model) updateRecipe(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.recipe)-1 {
			m.cursor++
		}
	case "a":
		return m, m.openEdit(true)
	case "e", "enter":
		return m, m.openEdit(len(m.recipe) == 0)
	case "d", "delete":
		if len(m.recipe) > 0 {
			m.recipe = append(m.recipe[:m.cursor], m.recipe[m.cursor+1:]...)
			if m.cursor >= len(m.recipe) && m.cursor > 0 {
				m.cursor--
			}
		}
	case "p":
		m.pRows = m.buildParamRows()
		if len(m.pRows) == 0 {
			m.setMsg("В модели нет параметров — добавьте их в «Формулы и параметры» (клавиша f).", true)
			return m, nil
		}
		m.pFocus = 0
		m.setMsg("", false)
		m.screen = scParams
		return m, textinput.Blink
	case "f":
		m.fBack = scRecipe
		m.setMsg("", false)
		m.screen = scFormulas
	case "c":
		m.back = scRecipe
		m.calculate()
	case "esc":
		m.setMsg("", false)
		m.screen = scMenu
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// navForm — навигация по строкам формы; возвращает true, если клавиша обработана.
func navForm(rows []row, idx *int, key string) bool {
	switch key {
	case "up", "shift+tab":
		if *idx > 0 {
			*idx--
		}
	case "down", "tab":
		if *idx < len(rows)-1 {
			*idx++
		}
	default:
		return false
	}
	focusRow(rows, *idx)
	return true
}

func (m model) updateEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			m.screen = scRecipe
			return m, nil
		case "enter":
			if m.editIdx < len(m.editRows)-1 {
				m.editIdx++
				focusRow(m.editRows, m.editIdx)
				return m, nil
			}
			name := strings.TrimSpace(m.editRows[0].input.Value())
			mass, e1 := parseF(m.editRows[1].input.Value())
			wet, e2 := parseF(m.editRows[2].input.Value())
			switch {
			case name == "":
				m.setMsg("Введите наименование сырья", true)
			case e1 != nil || mass < 0:
				m.setMsg("Масса — неотрицательное число", true)
			case e2 != nil || wet < 0 || wet > 100:
				m.setMsg("Влажность — число от 0 до 100", true)
			default:
				ing := Ingredient{name, mass, wet}
				if m.editNew || len(m.recipe) == 0 {
					m.recipe = append(m.recipe, ing)
					m.cursor = len(m.recipe) - 1
				} else {
					m.recipe[m.cursor] = ing
				}
				m.setMsg("", false)
				m.screen = scRecipe
			}
			return m, nil
		}
		if navForm(m.editRows, &m.editIdx, k.String()) {
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.editRows[m.editIdx].input, cmd = m.editRows[m.editIdx].input.Update(msg)
	return m, cmd
}

// ── экран параметров перед расчётом ──

func (m *model) buildParamRows() []row {
	var rows []row
	for _, p := range m.mdl.Params {
		label := p.Title
		if p.Unit != "" {
			label += ", " + p.Unit
		}
		hint := p.ID
		if p.Note != "" {
			hint += " • " + p.Note
		}
		rows = append(rows, newRow(label, hint, fnum(m.values[p.ID])))
	}
	for _, g := range m.mdl.Groups() {
		rows = append(rows, newToggle("Считать: "+g, "пробел — включить или выключить группу формул", m.groups[g]))
	}
	focusRow(rows, 0)
	return rows
}

func (m *model) applyParamRows() error {
	np := len(m.mdl.Params)
	vals := map[string]float64{}
	for i, p := range m.mdl.Params {
		v, err := parseF(m.pRows[i].input.Value())
		if err != nil {
			return fmt.Errorf("поле «%s»: введите число", p.Title)
		}
		vals[p.ID] = v
	}
	groups := map[string]bool{}
	for j, g := range m.mdl.Groups() {
		groups[g] = m.pRows[np+j].on
	}
	m.values, m.groups = vals, groups
	return nil
}

func (m model) updateParams(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			_ = m.applyParamRows()
			m.setMsg("", false)
			m.screen = scRecipe
			return m, nil
		case "enter", "ctrl+r":
			if err := m.applyParamRows(); err != nil {
				m.setMsg(err.Error(), true)
				return m, nil
			}
			m.back = scParams
			m.calculate()
			return m, nil
		case " ":
			if m.pRows[m.pFocus].toggle {
				m.pRows[m.pFocus].on = !m.pRows[m.pFocus].on
				return m, nil
			}
		}
		if navForm(m.pRows, &m.pFocus, k.String()) {
			return m, nil
		}
		if m.pRows[m.pFocus].toggle {
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.pRows[m.pFocus].input, cmd = m.pRows[m.pFocus].input.Update(msg)
	return m, cmd
}

func (m model) updateResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc", "backspace":
			m.setMsg("", false)
			m.screen = m.back
			return m, nil
		case "m":
			m.setMsg("", false)
			m.screen = scMenu
			return m, nil
		case "s":
			m.saveReport()
			return m, nil
		case "q":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// ── экран «Формулы и параметры» ──

func (m *model) saveModel(nm *engine.Model, okMsg string) bool {
	if err := engine.Save(m.mdlPath, nm); err != nil {
		m.setMsg("Не удалось сохранить файл модели: "+err.Error(), true)
		return false
	}
	m.mdl = nm
	m.syncValues()
	m.setMsg(okMsg, false)
	return true
}

func (m model) listLen() int {
	if m.fTab == 0 {
		return len(m.mdl.Formulas)
	}
	return len(m.mdl.Params)
}

func (m model) updateFormulas(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	prev := m.confirm
	m.confirm = ""
	m.setMsg("", false)
	cur := &m.fCur[m.fTab]

	switch k.String() {
	case "up", "k":
		if *cur > 0 {
			*cur--
		}
	case "down", "j":
		if *cur < m.listLen()-1 {
			*cur++
		}
	case "tab", "left", "right", "h", "l":
		m.fTab = 1 - m.fTab
	case "a":
		return m, m.openDef(true)
	case "e", "enter":
		return m, m.openDef(m.listLen() == 0)
	case "d", "delete":
		if m.listLen() == 0 {
			return m, nil
		}
		id := ""
		if m.fTab == 0 {
			id = m.mdl.Formulas[*cur].ID
		} else {
			id = m.mdl.Params[*cur].ID
		}
		if users := m.mdl.UsedBy(id); len(users) > 0 {
			m.setMsg(fmt.Sprintf("Нельзя удалить «%s»: имя используется в формулах: %s", id, strings.Join(users, ", ")), true)
			return m, nil
		}
		if prev != "del:"+id {
			m.confirm = "del:" + id
			m.setMsg("Нажмите d ещё раз, чтобы удалить «"+id+"».", false)
			return m, nil
		}
		var nm *engine.Model
		if m.fTab == 0 {
			nm = m.mdl.WithoutFormula(id)
		} else {
			nm = m.mdl.WithoutParam(id)
		}
		if m.saveModel(nm, "Удалено: "+id) && *cur >= m.listLen() && *cur > 0 {
			*cur--
		}
	case "r":
		if prev != "reset" {
			m.confirm = "reset"
			m.setMsg("Сбросить все формулы и параметры к методичке? Нажмите r ещё раз (прежняя версия останется в .bak).", true)
			return m, nil
		}
		if m.saveModel(engine.Default(), "Восстановлена модель по методичке.") {
			m.fCur = [2]int{}
		}
	case "esc":
		m.screen = m.fBack
	}
	return m, nil
}

const (
	fID = iota
	fTitle
	fExpr
	fUnit
	fRef
	fGroup
	fFinal
)

func (m *model) openDef(isNew bool) tea.Cmd {
	m.dOld = ""
	m.dIdx = 0
	m.setMsg("", false)
	cur := m.fCur[m.fTab]
	if m.fTab == 0 {
		f := engine.Formula{}
		if !isNew {
			f = m.mdl.Formulas[cur]
			m.dOld = f.ID
		}
		m.dRows = []row{
			newRow("Имя (латиница)", "например: Bres. Без пробелов; регистр важен", f.ID),
			newRow("Название", "как выводится в отчёте", f.Title),
			newRow("Выражение", "+ - * / ^ ( ), функции min max abs sqrt round if", f.Expr),
			newRow("Единица", "кг, %, …", f.Unit),
			newRow("Обозначение", "номер формулы в методичке, например (5); можно пусто", f.Ref),
			newRow("Группа", "пусто — считать всегда; иначе формула включается переключателем", f.Group),
			newToggle("Итоговый результат", "пробел — переключить; итоги выделяются рамкой в отчёте", f.Final),
		}
		m.dRows[fExpr].input.Width = 64
	} else {
		p := engine.Param{}
		if !isNew {
			p = m.mdl.Params[cur]
			m.dOld = p.ID
		}
		val := ""
		if !isNew {
			val = fnum(p.Value)
		}
		m.dRows = []row{
			newRow("Имя (латиница)", "например: Kz. Без пробелов; регистр важен", p.ID),
			newRow("Название", "как показывается в списке параметров", p.Title),
			newRow("Единица", "%, кг, …", p.Unit),
			newRow("Значение", "по умолчанию; перед расчётом можно изменить", val),
			newRow("Подсказка", "необязательно", p.Note),
		}
	}
	focusRow(m.dRows, 0)
	m.screen = scDef
	return textinput.Blink
}

func (m model) candFormula() engine.Formula {
	v := func(i int) string { return strings.TrimSpace(m.dRows[i].input.Value()) }
	f := engine.Formula{ID: v(fID), Title: v(fTitle), Expr: v(fExpr), Unit: v(fUnit),
		Ref: v(fRef), Group: v(fGroup), Final: m.dRows[fFinal].on}
	if f.Title == "" {
		f.Title = f.ID
	}
	return f
}

func (m model) candParam() (engine.Param, error) {
	v := func(i int) string { return strings.TrimSpace(m.dRows[i].input.Value()) }
	val, err := parseF(v(3))
	if err != nil {
		return engine.Param{}, fmt.Errorf("значение: введите число (например 0.95)")
	}
	p := engine.Param{ID: v(0), Title: v(1), Unit: v(2), Value: val, Note: v(4)}
	if p.Title == "" {
		p.Title = p.ID
	}
	return p, nil
}

// newProblems — ошибки, появившиеся из-за правки (уже существовавшие не считаются).
func (m model) newProblems(cand *engine.Model) []engine.Problem {
	base := map[string]bool{}
	for _, p := range m.mdl.Validate() {
		base[p.Kind+"|"+p.String()] = true
	}
	var out []engine.Problem
	for _, p := range cand.Validate() {
		if !base[p.Kind+"|"+p.String()] {
			out = append(out, p)
		}
	}
	return out
}

func finalsText(res *engine.Result) string {
	var parts []string
	for _, s := range res.Steps {
		if s.Formula.Final {
			parts = append(parts, withUnit(fmt.Sprintf("%s = %.2f", s.Formula.ID, s.Value), s.Formula.Unit))
		}
	}
	return strings.Join(parts, "; ")
}

// preview проверяет редактируемый элемент и считает его на текущей рецептуре.
func (m model) preview() (string, bool) {
	if m.fTab == 0 {
		f := m.candFormula()
		cand := m.mdl.WithFormula(f, m.dOld)
		if np := m.newProblems(cand); len(np) > 0 {
			return np[0].String(), false
		}
		if len(m.recipe) == 0 {
			return "Выражение верно. Чтобы увидеть значение, добавьте сырьё в рецептуру.", true
		}
		in := m.inputs()
		if f.Group != "" {
			g := map[string]bool{}
			for k, v := range in.Groups {
				g[k] = v
			}
			g[f.Group] = true
			in.Groups = g
		}
		res, err := cand.Calculate(in)
		if err != nil {
			if len(cand.Validate()) > 0 {
				return "Выражение верно (в других формулах модели есть ошибки — значение не вычислено).", true
			}
			return err.Error(), false
		}
		for _, s := range res.Steps {
			if s.Formula.ID == f.ID {
				return fmt.Sprintf("%s   ←   %s", withUnit("= "+engine.FormatNumber(s.Value), f.Unit), s.Expanded), true
			}
		}
		return "Выражение верно.", true
	}

	p, err := m.candParam()
	if err != nil {
		return err.Error(), false
	}
	cand := m.mdl.WithParam(p, m.dOld)
	if np := m.newProblems(cand); len(np) > 0 {
		return np[0].String(), false
	}
	if len(m.recipe) == 0 {
		return "Параметр корректен.", true
	}
	in := m.inputs()
	vals := map[string]float64{}
	for k, v := range in.Params {
		vals[k] = v
	}
	vals[p.ID] = p.Value
	in.Params = vals
	res, err := cand.Calculate(in)
	if err != nil {
		if len(cand.Validate()) > 0 {
			return "Параметр корректен (в модели есть другие ошибки — итоги не вычислены).", true
		}
		return err.Error(), false
	}
	if t := finalsText(res); t != "" {
		return "Итоги с этим значением: " + t, true
	}
	return "Параметр корректен.", true
}

func (m *model) saveDef() {
	if text, ok := m.preview(); !ok {
		m.setMsg("Не сохранено: "+text, true)
		return
	}
	var nm *engine.Model
	savedID := ""
	if m.fTab == 0 {
		f := m.candFormula()
		nm = m.mdl.WithFormula(f, m.dOld)
		savedID = f.ID
	} else {
		p, _ := m.candParam()
		nm = m.mdl.WithParam(p, m.dOld)
		savedID = p.ID
	}
	if !m.saveModel(nm, "Сохранено: "+savedID+"  (файл: "+m.mdlPath+")") {
		return
	}
	if m.fTab == 1 {
		p, _ := m.candParam()
		m.values[p.ID] = p.Value
		m.fCur[1] = nm.FindParam(p.ID)
	} else {
		m.fCur[0] = nm.FindFormula(savedID)
	}
	m.screen = scFormulas
}

func (m model) updateDef(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			m.setMsg("", false)
			m.screen = scFormulas
			return m, nil
		case "ctrl+s":
			m.saveDef()
			return m, nil
		case "enter":
			if m.dIdx < len(m.dRows)-1 {
				m.dIdx++
				focusRow(m.dRows, m.dIdx)
				return m, nil
			}
			m.saveDef()
			return m, nil
		case " ":
			if m.dRows[m.dIdx].toggle {
				m.dRows[m.dIdx].on = !m.dRows[m.dIdx].on
				return m, nil
			}
		}
		if navForm(m.dRows, &m.dIdx, k.String()) {
			return m, nil
		}
		if m.dRows[m.dIdx].toggle {
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.dRows[m.dIdx].input, cmd = m.dRows[m.dIdx].input.Update(msg)
	return m, cmd
}

// ───────────────────────────── view ─────────────────────────────

func (m model) header(sub string) string {
	return stBanner.Render("🍞 BreadCalc") + "  " + stSub.Render(sub) + "\n\n"
}

func (m model) footer(help string) string {
	out := ""
	if m.msg != "" {
		if m.isErr {
			out += stErr.Render("✗ "+m.msg) + "\n"
		} else {
			out += stOK.Render("✓ "+m.msg) + "\n"
		}
	}
	return "\n" + out + stHelp.Render(help)
}

func (m model) View() string {
	switch m.screen {
	case scMenu:
		return m.header("Расчёт выхода хлебобулочных изделий") + m.menu.View() +
			m.footer("↑/↓ выбор • enter открыть • q выход")
	case scRecipe:
		return m.viewRecipe()
	case scEdit:
		title := "Редактирование сырья"
		if m.editNew {
			title = "Новое сырьё"
		}
		return m.header(title) + renderForm(m.editRows, m.editIdx, 24) +
			m.footer("tab/↑/↓ поле • enter далее/сохранить • esc отмена")
	case scParams:
		return m.header("Параметры расчёта — "+m.name) + renderForm(m.pRows, m.pFocus, 40) +
			m.footer("↑/↓ поле • пробел переключить • enter рассчитать • esc назад")
	case scFormulas:
		return m.viewFormulas()
	case scDef:
		return m.viewDef()
	default:
		pct := fmt.Sprintf("%3.0f%%", m.vp.ScrollPercent()*100)
		return m.header("Результат — "+m.name) + m.vp.View() +
			m.footer("↑/↓ PgUp/PgDn прокрутка ("+pct+") • s сохранить в .txt • esc назад • m меню • q выход")
	}
}

func renderForm(rows []row, focus int, lw int) string {
	var b strings.Builder
	for i, r := range rows {
		mark, ls := "  ", lipgloss.NewStyle().Width(lw)
		if i == focus {
			mark, ls = stSel.Render("▸ "), ls.Foreground(cGreen).Bold(true)
		}
		var val string
		switch {
		case r.toggle && r.on:
			val = stOK.Render("[x] Да")
		case r.toggle:
			val = stSub.Render("[ ] Нет")
		default:
			val = r.input.View()
		}
		line := mark + ls.Render(truncate(r.label, lw-1)) + " " + val
		if r.hint != "" && i == focus {
			line += "\n    " + stHelp.Render(r.hint)
		}
		b.WriteString(line + "\n")
	}
	return stPanel.Render(strings.TrimRight(b.String(), "\n")) + "\n"
}

func cell(s string, w int, right bool) string {
	st := lipgloss.NewStyle().Width(w)
	if right {
		st = st.Align(lipgloss.Right)
	}
	return st.Render(truncate(s, w))
}

func window(cursor, total, visible int) (int, int) {
	if total <= visible {
		return 0, total
	}
	start := cursor - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > total {
		start = total - visible
	}
	return start, start + visible
}

func yesNo(b bool) string {
	if b {
		return "да"
	}
	return "нет"
}

func (m model) viewRecipe() string {
	nameW := min(max(m.w-40, 24), 56)
	var b strings.Builder
	hdr := cell("№", 4, false) + cell("Наименование", nameW, false) +
		cell("Масса, кг", 11, true) + cell("Влажн., %", 11, true)
	b.WriteString(stHdr.Render(hdr) + "\n")
	b.WriteString(stSub.Render(strings.Repeat("─", 4+nameW+22)) + "\n")

	if len(m.recipe) == 0 {
		b.WriteString(stSub.Render("  (пусто — нажмите a, чтобы добавить сырьё)") + "\n")
	}
	for i, ing := range m.recipe {
		line := cell(strconv.Itoa(i+1), 4, false) + cell(ing.Name, nameW, false) +
			cell(fmt.Sprintf("%.2f", ing.Mass), 11, true) + cell(fmt.Sprintf("%.2f", ing.Moisture), 11, true)
		if i == m.cursor {
			line = stSel.Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(stSub.Render(strings.Repeat("─", 4+nameW+22)) + "\n")
	mc, smw := aggregates(m.recipe)
	wcp := 0.0
	if mc > 0 {
		wcp = smw / mc
	}
	b.WriteString(stTitle.Render(fmt.Sprintf("Σ масса сырья: %.2f кг   W_ср: %.2f %%", mc, wcp)) + "\n")

	info := fmt.Sprintf("параметров: %d • формул: %d", len(m.mdl.Params), len(m.mdl.Formulas))
	for _, g := range m.mdl.Groups() {
		info += " • " + g + ": " + yesNo(m.groups[g])
	}

	return m.header(m.name) + stPanel.Render(strings.TrimRight(b.String(), "\n")) + "\n" +
		stSub.Render(info) + "\n" +
		m.footer("↑/↓ выбор • a добавить • e изменить • d удалить • p параметры • f формулы • c рассчитать • esc меню")
}

func (m model) viewFormulas() string {
	tab := func(label string, n int, active bool) string {
		t := fmt.Sprintf(" %s (%d) ", label, n)
		if active {
			return stBanner.Render(t)
		}
		return stSub.Render(t)
	}
	var b strings.Builder
	b.WriteString(tab("Формулы", len(m.mdl.Formulas), m.fTab == 0) + " " +
		tab("Параметры", len(m.mdl.Params), m.fTab == 1) + "\n\n")

	titleW := min(max(m.w-52, 16), 46)
	visible := max(m.h-17, 5)
	cur := m.fCur[m.fTab]
	probs := m.mdl.Validate()
	bad := map[string]bool{}
	for _, p := range probs {
		bad[p.ID] = true
	}

	if m.fTab == 0 {
		b.WriteString(stHdr.Render(cell("", 2, false)+cell("Обозн.", 7, false)+cell("Имя", 11, false)+
			cell("Название", titleW, false)+cell("Ед.", 5, false)+"Свойства") + "\n")
		start, end := window(cur, len(m.mdl.Formulas), visible)
		for i := start; i < end; i++ {
			f := m.mdl.Formulas[i]
			flags := ""
			if f.Final {
				flags += "★ итог "
			}
			if f.Group != "" {
				flags += "[" + f.Group + "]"
			}
			mark := "  "
			if bad[f.ID] {
				mark = "⚠ "
			}
			line := cell(mark, 2, false) + cell(f.Ref, 7, false) + cell(f.ID, 11, false) +
				cell(f.Title, titleW, false) + cell(f.Unit, 5, false) + flags
			if i == cur {
				line = stSel.Render(line)
			} else if bad[f.ID] {
				line = stErr.Render(line)
			}
			b.WriteString(line + "\n")
		}
		if len(m.mdl.Formulas) == 0 {
			b.WriteString(stSub.Render("  (нет формул — нажмите a, чтобы добавить)") + "\n")
		}
	} else {
		b.WriteString(stHdr.Render(cell("", 2, false)+cell("Имя", 11, false)+cell("Название", titleW, false)+
			cell("Значение", 11, true)+"  Ед.") + "\n")
		start, end := window(cur, len(m.mdl.Params), visible)
		for i := start; i < end; i++ {
			p := m.mdl.Params[i]
			mark := "  "
			if bad[p.ID] {
				mark = "⚠ "
			}
			line := cell(mark, 2, false) + cell(p.ID, 11, false) + cell(p.Title, titleW, false) +
				cell(fnum(p.Value), 11, true) + "  " + p.Unit
			if i == cur {
				line = stSel.Render(line)
			} else if bad[p.ID] {
				line = stErr.Render(line)
			}
			b.WriteString(line + "\n")
		}
		if len(m.mdl.Params) == 0 {
			b.WriteString(stSub.Render("  (нет параметров — нажмите a, чтобы добавить)") + "\n")
		}
	}

	out := m.header("Формулы и параметры") + stPanel.Render(strings.TrimRight(b.String(), "\n")) + "\n"
	if len(probs) > 0 {
		out += stErr.Render(fmt.Sprintf("⚠ Ошибок в модели: %d", len(probs))) + "\n"
		for i, p := range probs {
			if i == 3 {
				out += stErr.Render("  …") + "\n"
				break
			}
			out += stErr.Render("  "+p.String()) + "\n"
		}
	}
	out += stSub.Render("Файл модели: "+m.mdlPath) + "\n"
	return out + m.footer("↑/↓ выбор • tab формулы/параметры • a добавить • e изменить • d удалить • r сброс к методичке • esc назад")
}

func (m model) viewDef() string {
	kind := "формулы"
	if m.fTab == 1 {
		kind = "параметра"
	}
	title := "Новая формула"
	switch {
	case m.fTab == 0 && m.dOld != "":
		title = "Редактирование формулы «" + m.dOld + "»"
	case m.fTab == 1 && m.dOld == "":
		title = "Новый параметр"
	case m.fTab == 1:
		title = "Редактирование параметра «" + m.dOld + "»"
	}
	_ = kind

	text, ok := m.preview()
	st, mark := stOK, "✓ "
	if !ok {
		st, mark = stErr, "✗ "
	}
	wrap := lipgloss.NewStyle().Width(max(m.w-6, 30))

	out := m.header(title) + renderForm(m.dRows, m.dIdx, 24)
	out += "\n" + stTitle.Render("Проверка") + "\n" + st.Render(wrap.Render(mark+text)) + "\n"
	if m.fTab == 0 {
		var names []string
		for _, v := range m.mdl.Names() {
			names = append(names, v.ID)
		}
		out += "\n" + stSub.Render(wrap.Render("Доступные имена: "+strings.Join(names, ", "))) + "\n"
	}
	return out + m.footer("tab/↑/↓ поле • enter далее/сохранить • ctrl+s сохранить сразу • esc отмена")
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Ошибка:", err)
		os.Exit(1)
	}
}
