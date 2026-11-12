package gui

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"breadcalc/engine"
	"breadcalc/recipe"
)

// App — главное окно. Хранит состояние; расчёт и проверки выполняет engine.
type App struct {
	win  fyne.Window
	mdl  *engine.Model
	path string // файл модели; "" — не сохранять на диск (тесты)

	presets []recipe.Preset
	name    string
	items   []recipe.Ingredient
	values  map[string]float64
	groups  map[string]bool
	report  string

	tabs       *container.AppTabs
	presetList *widget.List
	title      *widget.RichText
	subtitle   *widget.Label

	recipeTable     *widget.Table
	recipeSel       int
	totals          *widget.Label
	editBtn, delBtn *widget.Button

	modelTabs    *container.AppTabs
	paramsHost   *fyne.Container
	paramEntries map[string]*widget.Entry
	resultHost   *fyne.Container
	formulaTable *widget.Table
	fSel         int
	fEdit, fDel  *widget.Button
	paramTable   *widget.Table
	pSel         int
	pEdit, pDel  *widget.Button
	problems     *widget.RichText
	status       *widget.Label
	bad          map[string]bool
}

// New создаёт интерфейс в окне win. path — файл модели для сохранения изменений.
func New(win fyne.Window, mdl *engine.Model, path string) *App {
	a := &App{win: win, mdl: mdl, path: path, recipeSel: -1, fSel: -1, pSel: -1,
		presets: recipe.Presets(), bad: map[string]bool{}}
	a.resetValues(nil, nil)
	a.build()
	return a
}

// Warn показывает предупреждение поверх окна (например, о повреждённом файле модели).
func (a *App) Warn(title, msg string) { dialog.ShowInformation(title, msg, a.win) }

// ───────────────────────────── вспомогательное ─────────────────────────────

func fnum(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

func parseNum(s string) (float64, error) {
	v, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(s), ",", "."), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, errors.New("введите число")
	}
	return v, nil
}

func numberValidator(lo, hi float64) fyne.StringValidator {
	return func(s string) error {
		v, err := parseNum(s)
		if err != nil {
			return err
		}
		if v < lo || v > hi {
			if math.IsInf(hi, 1) {
				return fmt.Errorf("значение не меньше %g", lo)
			}
			return fmt.Errorf("значение от %g до %g", lo, hi)
		}
		return nil
	}
}

func (a *App) resetValues(over map[string]float64, groups map[string]bool) {
	a.values = map[string]float64{}
	for _, p := range a.mdl.Params {
		a.values[p.ID] = p.Value
	}
	for k, v := range over {
		if _, ok := a.values[k]; ok {
			a.values[k] = v
		}
	}
	a.groups = map[string]bool{}
	for k, v := range groups {
		a.groups[k] = v
	}
}

func (a *App) syncValues() {
	ids := map[string]bool{}
	for _, p := range a.mdl.Params {
		ids[p.ID] = true
		if _, ok := a.values[p.ID]; !ok {
			a.values[p.ID] = p.Value
		}
	}
	for id := range a.values {
		if !ids[id] {
			delete(a.values, id)
		}
	}
}

func (a *App) inputs() engine.Inputs {
	mc, sum := recipe.Aggregates(a.items)
	return engine.Inputs{Mc: mc, SumMW: sum, Cnt: len(a.items), Params: a.values, Groups: a.groups}
}

// apply сохраняет новую модель (на диск, если задан путь) и обновляет все вкладки.
func (a *App) apply(nm *engine.Model, msg string) {
	if a.path != "" {
		if err := engine.Save(a.path, nm); err != nil {
			dialog.ShowError(fmt.Errorf("не удалось сохранить файл модели: %w", err), a.win)
			return
		}
	}
	a.mdl = nm
	a.invalidateResult()
	a.syncValues()
	a.rebuildParams()
	a.refreshModelViews()
	a.status.SetText(msg)
}

// ───────────────────────────── каркас окна ─────────────────────────────

func (a *App) build() {
	a.title = widget.NewRichTextFromMarkdown("# BreadCalc")
	a.subtitle = widget.NewLabel("")
	a.subtitle.Importance = widget.LowImportance

	brand := widget.NewRichTextFromMarkdown("## 🍞 BreadCalc")
	tag := widget.NewLabel("Расчёт выхода по методичке")
	tag.Importance = widget.LowImportance

	a.presetList = widget.NewList(
		func() int { return len(a.presets) },
		func() fyne.CanvasObject {
			n := widget.NewLabelWithStyle("Название рецептуры", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			d := widget.NewLabel("описание рецептуры")
			d.Importance = widget.LowImportance
			n.Truncation, d.Truncation = fyne.TextTruncateEllipsis, fyne.TextTruncateEllipsis
			return container.NewVBox(n, d)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			box := o.(*fyne.Container)
			box.Objects[0].(*widget.Label).SetText(a.presets[i].Name)
			box.Objects[1].(*widget.Label).SetText(a.presets[i].Desc)
		})
	a.presetList.OnSelected = a.loadPreset

	newBtn := widget.NewButtonWithIcon("Новая рецептура", theme.ContentAddIcon(), a.newRecipe)
	sidebar := container.NewBorder(
		container.NewVBox(brand, tag, widget.NewSeparator()),
		container.NewPadded(newBtn), nil, nil, a.presetList)

	a.status = widget.NewLabel("")
	a.status.Importance = widget.LowImportance
	a.problems = widget.NewRichText()

	a.tabs = container.NewAppTabs(
		container.NewTabItemWithIcon("Рецептура", theme.ListIcon(), a.recipeTab()),
		container.NewTabItemWithIcon("Параметры", theme.SettingsIcon(), a.paramsTab()),
		container.NewTabItemWithIcon("Результат", theme.DocumentIcon(), a.resultTab()),
		container.NewTabItemWithIcon("Формулы", theme.DocumentCreateIcon(), a.modelTab()),
	)

	head := container.NewVBox(a.title, a.subtitle)
	main := container.NewBorder(container.NewPadded(head), nil, nil, nil, a.tabs)
	split := container.NewHSplit(sidebar, main)
	split.Offset = 0.25
	a.win.SetContent(split)

	a.rebuildParams()
	a.refreshModelViews()
	a.presetList.Select(0)
}

func (a *App) setHeader(name, sub string) {
	a.title.ParseMarkdown("# " + name)
	a.subtitle.SetText(sub)
}

func (a *App) loadPreset(i widget.ListItemID) {
	p := a.presets[i]
	a.name = p.Name
	a.items = append([]recipe.Ingredient(nil), p.Recipe...)
	a.resetValues(p.Overrides, p.Groups)
	a.afterRecipeChange()
	a.rebuildParams()
	a.setHeader(p.Name, p.Desc)
	a.tabs.SelectIndex(0)
}

func (a *App) newRecipe() {
	a.presetList.UnselectAll()
	a.name = "Новая рецептура"
	a.items = nil
	a.resetValues(nil, nil)
	a.afterRecipeChange()
	a.rebuildParams()
	a.setHeader(a.name, "добавьте сырьё и нажмите «Рассчитать»")
	a.tabs.SelectIndex(0)
}

// ───────────────────────────── вкладка «Рецептура» ─────────────────────────────

func (a *App) recipeTab() fyne.CanvasObject {
	headers := []string{"№", "Наименование", "Масса, кг", "Влажность, %"}
	a.recipeTable = widget.NewTable(
		func() (int, int) { return len(a.items), 4 },
		func() fyne.CanvasObject {
			l := widget.NewLabel("Наименование сырья")
			l.Truncation = fyne.TextTruncateEllipsis
			return l
		},
		func(id widget.TableCellID, o fyne.CanvasObject) {
			l := o.(*widget.Label)
			if id.Row >= len(a.items) {
				return
			}
			it := a.items[id.Row]
			l.Alignment = fyne.TextAlignLeading
			switch id.Col {
			case 0:
				l.SetText(strconv.Itoa(id.Row + 1))
			case 1:
				l.SetText(it.Name)
			case 2:
				l.Alignment = fyne.TextAlignTrailing
				l.SetText(fmt.Sprintf("%.2f", it.Mass))
			case 3:
				l.Alignment = fyne.TextAlignTrailing
				l.SetText(fmt.Sprintf("%.2f", it.Moisture))
			}
		})
	a.recipeTable.ShowHeaderRow = true
	a.recipeTable.CreateHeader = func() fyne.CanvasObject {
		return widget.NewLabelWithStyle("Заголовок", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	a.recipeTable.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		l := o.(*widget.Label)
		l.Alignment = fyne.TextAlignLeading
		if id.Col >= 2 {
			l.Alignment = fyne.TextAlignTrailing
		}
		if id.Col >= 0 && id.Col < len(headers) {
			l.SetText(headers[id.Col])
		}
	}
	for c, w := range []float32{56, 420, 130, 140} {
		a.recipeTable.SetColumnWidth(c, w)
	}
	a.recipeTable.OnSelected = func(id widget.TableCellID) {
		a.recipeSel = id.Row
		a.editBtn.Enable()
		a.delBtn.Enable()
	}

	addBtn := widget.NewButtonWithIcon("Добавить", theme.ContentAddIcon(), func() { a.ingredientDialog(-1) })
	a.editBtn = widget.NewButtonWithIcon("Изменить", theme.DocumentCreateIcon(), func() { a.ingredientDialog(a.recipeSel) })
	a.delBtn = widget.NewButtonWithIcon("Удалить", theme.DeleteIcon(), a.deleteIngredient)
	a.delBtn.Importance = widget.DangerImportance
	a.editBtn.Disable()
	a.delBtn.Disable()
	calc := widget.NewButtonWithIcon("Рассчитать", theme.MediaPlayIcon(), a.calculate)
	calc.Importance = widget.HighImportance

	a.totals = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	toolbar := container.NewHBox(addBtn, a.editBtn, a.delBtn, layout.NewSpacer(), calc)
	footer := container.NewVBox(widget.NewSeparator(), a.totals)
	return container.NewBorder(container.NewPadded(toolbar), container.NewPadded(footer), nil, nil, a.recipeTable)
}

func (a *App) afterRecipeChange() {
	a.invalidateResult()
	a.recipeSel = -1
	a.recipeTable.UnselectAll()
	a.recipeTable.Refresh()
	a.editBtn.Disable()
	a.delBtn.Disable()
	mc, sum := recipe.Aggregates(a.items)
	wcp := 0.0
	if mc > 0 {
		wcp = sum / mc
	}
	a.totals.SetText(fmt.Sprintf("Σ масса сырья: %.2f кг     W_ср: %.2f %%     позиций: %d", mc, wcp, len(a.items)))
}

func (a *App) deleteIngredient() {
	if a.recipeSel < 0 || a.recipeSel >= len(a.items) {
		return
	}
	it := a.items[a.recipeSel]
	dialog.ShowConfirm("Удалить сырьё?", "«"+it.Name+"» будет удалено из рецептуры.", func(ok bool) {
		if !ok {
			return
		}
		a.items = append(a.items[:a.recipeSel], a.items[a.recipeSel+1:]...)
		a.afterRecipeChange()
	}, a.win)
}

func (a *App) ingredientDialog(idx int) {
	var it recipe.Ingredient
	isNew := idx < 0 || idx >= len(a.items)
	if !isNew {
		it = a.items[idx]
	}
	name := widget.NewEntry()
	mass := widget.NewEntry()
	wet := widget.NewEntry()
	name.SetText(it.Name)
	if !isNew {
		mass.SetText(fnum(it.Mass))
		wet.SetText(fnum(it.Moisture))
	}
	name.SetPlaceHolder("например: Мука пшеничная в/с")
	mass.SetPlaceHolder("кг")
	wet.SetPlaceHolder("0–100")
	name.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("введите наименование")
		}
		return nil
	}
	mass.Validator = numberValidator(0, math.Inf(1))
	wet.Validator = numberValidator(0, 100)

	title := "Изменить сырьё"
	if isNew {
		title = "Новое сырьё"
	}
	d := dialog.NewForm(title, "Сохранить", "Отмена", []*widget.FormItem{
		widget.NewFormItem("Наименование", name),
		widget.NewFormItem("Масса, кг", mass),
		widget.NewFormItem("Влажность, %", wet),
	}, func(ok bool) {
		if !ok {
			return
		}
		m, _ := parseNum(mass.Text)
		w, _ := parseNum(wet.Text)
		ni := recipe.Ingredient{Name: strings.TrimSpace(name.Text), Mass: m, Moisture: w}
		if isNew {
			a.items = append(a.items, ni)
		} else {
			a.items[idx] = ni
		}
		a.afterRecipeChange()
	}, a.win)
	d.Resize(fyne.NewSize(520, 260))
	d.Show()
}

// ───────────────────────────── вкладка «Параметры» ─────────────────────────────

func (a *App) paramsTab() fyne.CanvasObject {
	a.paramsHost = container.NewVBox()
	a.paramEntries = map[string]*widget.Entry{}
	reset := widget.NewButtonWithIcon("Значения по умолчанию", theme.ViewRefreshIcon(), func() {
		a.invalidateResult()
		a.resetValues(nil, a.groups)
		a.rebuildParams()
	})
	calc := widget.NewButtonWithIcon("Рассчитать", theme.MediaPlayIcon(), a.calculate)
	calc.Importance = widget.HighImportance
	bottom := container.NewPadded(container.NewHBox(reset, layout.NewSpacer(), calc))
	return container.NewBorder(nil, bottom, nil, nil, container.NewVScroll(container.NewPadded(a.paramsHost)))
}

func (a *App) rebuildParams() {
	if a.paramsHost == nil {
		return
	}
	a.paramEntries = map[string]*widget.Entry{}
	form := widget.NewForm()
	for _, p := range a.mdl.Params {
		id := p.ID
		e := widget.NewEntry()
		e.SetText(fnum(a.values[id]))
		e.Validator = numberValidator(math.Inf(-1), math.Inf(1))
		e.OnChanged = func(s string) {
			a.invalidateResult()
			if v, err := parseNum(s); err == nil {
				a.values[id] = v
			}
		}
		a.paramEntries[id] = e
		label := p.Title
		if p.Unit != "" {
			label += ", " + p.Unit
		}
		hint := p.ID
		if p.Note != "" {
			hint += " • " + p.Note
		}
		item := widget.NewFormItem(label, e)
		item.HintText = hint
		form.AppendItem(item)
	}
	objs := []fyne.CanvasObject{
		widget.NewCard("Нормативные значения", "Используются при расчёте. Изменить сами параметры можно на вкладке «Формулы».", form),
	}
	if gs := a.mdl.Groups(); len(gs) > 0 {
		box := container.NewVBox()
		for _, g := range gs {
			g := g
			c := widget.NewCheck("Считать: "+g, func(on bool) {
				if a.groups[g] != on {
					a.invalidateResult()
				}
				a.groups[g] = on
			})
			c.SetChecked(a.groups[g])
			box.Add(c)
		}
		objs = append(objs, widget.NewCard("Группы формул", "Формулы группы считаются, только если она включена.", box))
	}
	a.paramsHost.Objects = objs
	a.paramsHost.Refresh()
}

// ───────────────────────────── вкладка «Результат» ─────────────────────────────

func (a *App) resultTab() fyne.CanvasObject {
	hint := widget.NewLabelWithStyle("Здесь появится подробный расчёт.\nНажмите «Рассчитать» на вкладке «Рецептура» или «Параметры».",
		fyne.TextAlignCenter, fyne.TextStyle{})
	hint.Importance = widget.LowImportance
	a.resultHost = container.NewStack(container.NewCenter(hint))
	return a.resultHost
}

// invalidateResult не позволяет сохранить расчёт после изменения входных данных.
func (a *App) invalidateResult() {
	if a.report == "" || a.resultHost == nil {
		return
	}
	a.report = ""
	hint := widget.NewLabel("Данные изменены. Рассчитайте результат заново.")
	hint.Wrapping = fyne.TextWrapWord
	a.resultHost.Objects = []fyne.CanvasObject{container.NewCenter(hint)}
	a.resultHost.Refresh()
}

func (a *App) calculate() {
	for _, p := range a.mdl.Params {
		entry := a.paramEntries[p.ID]
		if err := entry.Validate(); err != nil {
			a.tabs.SelectIndex(1)
			a.win.Canvas().Focus(entry)
			dialog.ShowError(fmt.Errorf("параметр «%s»: %w", p.Title, err), a.win)
			return
		}
	}
	if len(a.items) == 0 {
		dialog.ShowInformation("Рецептура пуста", "Добавьте хотя бы одно сырьё.", a.win)
		return
	}
	if mc, _ := recipe.Aggregates(a.items); mc <= 0 {
		dialog.ShowInformation("Нечего считать", "Суммарная масса сырья должна быть больше нуля.", a.win)
		return
	}
	in := a.inputs()
	res, err := a.mdl.Calculate(in)
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	a.report = plainReport(a.name, a.items, a.mdl, in, res)
	a.resultHost.Objects = []fyne.CanvasObject{a.buildResult(res)}
	a.resultHost.Refresh()
	a.tabs.SelectIndex(2)
}

func (a *App) buildResult(res *engine.Result) fyne.CanvasObject {
	var cards []fyne.CanvasObject
	for _, s := range res.Steps {
		if !s.Formula.Final {
			continue
		}
		val := widget.NewRichText(&widget.TextSegment{
			Text: withUnit(fmt.Sprintf("%.2f", s.Value), s.Formula.Unit),
			Style: widget.RichTextStyle{ColorName: theme.ColorNamePrimary, SizeName: theme.SizeNameHeadingText,
				TextStyle: fyne.TextStyle{Bold: true}},
		})
		// заголовок в обычной подписи с переносом: длинные названия не налезают на соседнюю карточку
		name := widget.NewLabelWithStyle(s.Formula.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		name.Wrapping = fyne.TextWrapWord
		id := widget.NewLabel(s.Formula.ID)
		id.Importance = widget.LowImportance
		cards = append(cards, widget.NewCard("", "", container.NewVBox(name, id, val)))
	}
	steps := widget.NewList(
		func() int { return len(res.Steps) },
		func() fyne.CanvasObject {
			t := widget.NewLabelWithStyle("Формула", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			e := widget.NewLabelWithStyle("выражение", fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
			t.Truncation, e.Truncation = fyne.TextTruncateEllipsis, fyne.TextTruncateEllipsis
			return container.NewVBox(t, e)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			s := res.Steps[i]
			box := o.(*fyne.Container)
			head := s.Formula.Title
			if s.Formula.Ref != "" {
				head = s.Formula.Ref + "  " + head
			}
			box.Objects[0].(*widget.Label).SetText(head)
			box.Objects[1].(*widget.Label).SetText(fmt.Sprintf("%s = %s = %s",
				s.Formula.ID, s.Expanded, withUnit(engine.FormatNumber(s.Value), s.Formula.Unit)))
		})

	save := widget.NewButtonWithIcon("Сохранить отчёт…", theme.DocumentSaveIcon(), a.saveReport)
	cp := widget.NewButtonWithIcon("Копировать", theme.ContentCopyIcon(), func() {
		a.win.Clipboard().SetContent(a.report)
		a.status.SetText("Отчёт скопирован в буфер обмена")
	})
	bottom := container.NewPadded(container.NewHBox(save, cp))

	var top fyne.CanvasObject = layout.NewSpacer()
	if len(cards) > 0 {
		top = container.NewPadded(container.NewGridWithColumns(len(cards), cards...))
	}
	return container.NewBorder(top, bottom, nil, nil, steps)
}

func (a *App) saveReport() {
	if a.report == "" {
		return
	}
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if w == nil {
			return
		}
		defer w.Close()
		if _, err := w.Write([]byte(a.report)); err != nil {
			dialog.ShowError(err, a.win)
		}
	}, a.win)
	d.SetFileName("raschet_" + time.Now().Format("20060102_150405") + ".txt")
	d.Show()
}
