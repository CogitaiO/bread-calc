package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	stSub    = lipgloss.NewStyle().Foreground(cMuted)
	stTitle  = lipgloss.NewStyle().Bold(true).Foreground(cCyan)
	stSel    = lipgloss.NewStyle().Bold(true).Foreground(cGreen)
	stHdr    = lipgloss.NewStyle().Bold(true).Foreground(cAccent)
	stErr    = lipgloss.NewStyle().Bold(true).Foreground(cRed)
	stOK     = lipgloss.NewStyle().Foreground(cGreen)
	stHelp   = lipgloss.NewStyle().Foreground(cMuted)
	stPanel  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cMuted).Padding(0, 1)
	ansiRe   = regexp.MustCompile(`\x1b\[[0-9;]*m`)
)

// ───────────────────────────── меню ─────────────────────────────

type menuItem struct {
	title, desc string
	idx         int // -1 — новая рецептура
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
	ti.CharLimit = 80
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
	if !rows[idx].toggle {
		rows[idx].input.Focus()
	}
}

func fnum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func parseF(s string) (float64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
	return strconv.ParseFloat(s, 64)
}

func paramRows(p Params) []row {
	rows := []row{
		newRow("Влажность теста W_т, %", "для формового ≈ 47.5, для лепёшек ≈ 36–38", fnum(p.WTesta)),
		newRow("Влажность муки, %", "базисная влажность муки", fnum(p.FlourMoisture)),
		newRow("Разделка/подпыл, %", "0.6 — подовый хлеб и лепёшки, 0.0 — формовой", fnum(p.GRazd)),
		newRow("Потери муки при приёмке (7)", "норма, %", fnum(p.GM)),
		newRow("Распыл муки и теста (8)", "норма, %", fnum(p.GTM)),
		newRow("Затраты на брожение (9)", "норма, %", fnum(p.GCB)),
		newRow("Упёк (11), %", "", fnum(p.GUp)),
		newRow("Усушка (12), %", "", fnum(p.GUs)),
		newRow("Крошка и лом (13), %", "", fnum(p.GKr)),
		newRow("Отклонение штучной массы (14), %", "", fnum(p.GSht)),
		newRow("Переработка брака (15), %", "", fnum(p.GBr)),
		newToggle("Рассчитать сухарики", "пробел — переключить", p.CalcSuhariki),
		newRow("Потери и затраты на сухарики, %", "норма 27.0 – 35.0", fnum(p.GSuhariki)),
	}
	focusRow(rows, 0)
	return rows
}

func rowsToParams(rows []row) (Params, error) {
	var p Params
	targets := []*float64{&p.WTesta, &p.FlourMoisture, &p.GRazd, &p.GM, &p.GTM, &p.GCB,
		&p.GUp, &p.GUs, &p.GKr, &p.GSht, &p.GBr}
	for i, t := range targets {
		v, err := parseF(rows[i].input.Value())
		if err != nil {
			return p, fmt.Errorf("поле «%s»: введите число", rows[i].label)
		}
		*t = v
	}
	p.CalcSuhariki = rows[11].on
	v, err := parseF(rows[12].input.Value())
	if err != nil {
		return p, fmt.Errorf("поле «%s»: введите число", rows[12].label)
	}
	p.GSuhariki = v
	return p, nil
}

// ───────────────────────────── модель ─────────────────────────────

type screen int

const (
	scMenu screen = iota
	scRecipe
	scEdit
	scParams
	scResult
)

type model struct {
	screen screen
	back   screen // куда вернуться с экрана результата
	w, h   int

	menu list.Model

	name   string
	recipe []Ingredient
	params Params
	cursor int

	editRows []row
	editIdx  int
	editNew  bool

	pRows  []row
	pFocus int

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
	items = append(items, menuItem{"＋ Новая рецептура", "ввести сырьё и параметры вручную", -1})

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

	return model{menu: l, vp: viewport.New(80, 20), w: 100, h: 30}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m *model) setMsg(s string, isErr bool) { m.msg, m.isErr = s, isErr }

func (m *model) layout() {
	m.menu.SetSize(m.w-4, max(m.h-8, 6))
	m.vp.Width = m.w - 4
	m.vp.Height = max(m.h-9, 5)
}

func (m *model) calculate() {
	res, err := Calculate(m.recipe, m.params)
	if err != nil {
		m.setMsg(err.Error(), true)
		return
	}
	m.report = BuildReport(m.name, m.recipe, m.params, res)
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
			if it.idx >= 0 {
				p := presets()[it.idx]
				m.name = p.Name
				m.recipe = append([]Ingredient(nil), p.Recipe...)
				m.params = p.Params
			} else {
				m.name = "Новая рецептура"
				m.recipe = nil
				m.params = DefaultParams()
			}
			m.cursor = 0
			m.setMsg("", false)
			m.screen = scRecipe
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
	m.editRows = []row{
		newRow("Наименование сырья", "", ing.Name),
		newRow("Масса, кг", "", ifNew(isNew, "", fnum(ing.Mass))),
		newRow("Влажность, %", "", ifNew(isNew, "", fnum(ing.Moisture))),
	}
	m.editRows[0].input.Width = 50
	m.editIdx = 0
	focusRow(m.editRows, 0)
	m.setMsg("", false)
	m.screen = scEdit
	return textinput.Blink
}

func ifNew(isNew bool, a, b string) string {
	if isNew {
		return a
	}
	return b
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
		if len(m.recipe) > 0 {
			return m, m.openEdit(false)
		}
		return m, m.openEdit(true)
	case "d", "delete":
		if len(m.recipe) > 0 {
			m.recipe = append(m.recipe[:m.cursor], m.recipe[m.cursor+1:]...)
			if m.cursor >= len(m.recipe) && m.cursor > 0 {
				m.cursor--
			}
		}
	case "p":
		m.pRows = paramRows(m.params)
		m.pFocus = 0
		m.setMsg("", false)
		m.screen = scParams
		return m, textinput.Blink
	case "c":
		m.back = scRecipe
		m.calculate()
	case "esc":
		m.screen = scMenu
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// навигация по строкам формы; возвращает true, если клавиша обработана
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

func (m model) updateParams(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			if p, err := rowsToParams(m.pRows); err == nil {
				m.params = p
			}
			m.setMsg("", false)
			m.screen = scRecipe
			return m, nil
		case "enter", "ctrl+r":
			p, err := rowsToParams(m.pRows)
			if err != nil {
				m.setMsg(err.Error(), true)
				return m, nil
			}
			m.params = p
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
		return m.header(title) + renderForm(m.editRows, m.editIdx, true) +
			m.footer("tab/↑/↓ поле • enter далее/сохранить • esc отмена")
	case scParams:
		return m.header("Параметры расчёта — "+m.name) + renderForm(m.pRows, m.pFocus, false) +
			m.footer("↑/↓ поле • пробел переключить • enter рассчитать • esc назад")
	default:
		pct := fmt.Sprintf("%3.0f%%", m.vp.ScrollPercent()*100)
		return m.header("Результат — "+m.name) + m.vp.View() +
			m.footer("↑/↓ PgUp/PgDn прокрутка ("+pct+") • s сохранить в .txt • esc назад • m меню • q выход")
	}
}

func renderForm(rows []row, focus int, wide bool) string {
	var b strings.Builder
	lw := 38
	if wide {
		lw = 22
	}
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
		line := mark + ls.Render(r.label) + " " + val
		if r.hint != "" && i == focus {
			line += "  " + stHelp.Render(r.hint)
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
	var mc, smw float64
	for i, ing := range m.recipe {
		mc += ing.Mass
		smw += ing.Mass * ing.Moisture
		line := cell(strconv.Itoa(i+1), 4, false) + cell(ing.Name, nameW, false) +
			cell(fmt.Sprintf("%.2f", ing.Mass), 11, true) + cell(fmt.Sprintf("%.2f", ing.Moisture), 11, true)
		if i == m.cursor {
			line = stSel.Render(line)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(stSub.Render(strings.Repeat("─", 4+nameW+22)) + "\n")
	wcp := 0.0
	if mc > 0 {
		wcp = smw / mc
	}
	b.WriteString(stTitle.Render(fmt.Sprintf("Σ масса сырья: %.2f кг   W_ср: %.2f %%", mc, wcp)) + "\n")

	p := m.params
	info := fmt.Sprintf("W_т = %.1f %% • W_муки = %.1f %% • разделка = %.1f %% • сухарики: %s",
		p.WTesta, p.FlourMoisture, p.GRazd, map[bool]string{true: "да", false: "нет"}[p.CalcSuhariki])

	return m.header(m.name) + stPanel.Render(strings.TrimRight(b.String(), "\n")) + "\n" +
		stSub.Render(info) + "\n" +
		m.footer("↑/↓ выбор • a добавить • e изменить • d удалить • p параметры • c рассчитать • esc меню")
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Ошибка:", err)
		os.Exit(1)
	}
}
