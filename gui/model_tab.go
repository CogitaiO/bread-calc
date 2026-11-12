package gui

import (
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"breadcalc/engine"
)

func (a *App) modelTab() fyne.CanvasObject {
	a.modelTabs = container.NewAppTabs(
		container.NewTabItem("Формулы", a.formulasPane()),
		container.NewTabItem("Список параметров", a.paramDefsPane()),
	)
	reset := widget.NewButtonWithIcon("Сбросить к методичке", theme.ViewRefreshIcon(), a.resetModel)
	where := "Файл модели: не используется"
	if a.path != "" {
		where = "Файл модели: " + a.path
	}
	pathLbl := widget.NewLabel(where)
	pathLbl.Importance = widget.LowImportance
	pathLbl.Truncation = fyne.TextTruncateEllipsis
	bottom := container.NewVBox(widget.NewSeparator(), a.problems,
		container.NewBorder(nil, nil, nil, reset, pathLbl), a.status)
	return container.NewBorder(nil, container.NewPadded(bottom), nil, nil, a.modelTabs)
}

func newCell() fyne.CanvasObject {
	l := widget.NewLabel("Название формулы")
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

func headerCell() fyne.CanvasObject {
	return widget.NewLabelWithStyle("Заголовок", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
}

func (a *App) formulasPane() fyne.CanvasObject {
	headers := []string{"Обозн.", "Имя", "Название", "Ед.", "Свойства"}
	a.formulaTable = widget.NewTable(
		func() (int, int) { return len(a.mdl.Formulas), 5 },
		newCell,
		func(id widget.TableCellID, o fyne.CanvasObject) {
			l := o.(*widget.Label)
			if id.Row >= len(a.mdl.Formulas) {
				return
			}
			f := a.mdl.Formulas[id.Row]
			l.Importance = widget.MediumImportance
			if a.bad[f.ID] {
				l.Importance = widget.DangerImportance
			}
			switch id.Col {
			case 0:
				l.SetText(f.Ref)
			case 1:
				l.SetText(f.ID)
			case 2:
				l.SetText(f.Title)
			case 3:
				l.SetText(f.Unit)
			case 4:
				var fl []string
				if f.Final {
					fl = append(fl, "★ итог")
				}
				if f.Group != "" {
					fl = append(fl, "["+f.Group+"]")
				}
				l.SetText(strings.Join(fl, " "))
			}
		})
	a.formulaTable.ShowHeaderRow = true
	a.formulaTable.CreateHeader = headerCell
	a.formulaTable.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		if id.Col >= 0 && id.Col < len(headers) {
			o.(*widget.Label).SetText(headers[id.Col])
		}
	}
	for c, w := range []float32{80, 110, 360, 60, 190} {
		a.formulaTable.SetColumnWidth(c, w)
	}
	a.formulaTable.OnSelected = func(id widget.TableCellID) {
		a.fSel = id.Row
		a.fEdit.Enable()
		a.fDel.Enable()
	}

	add := widget.NewButtonWithIcon("Добавить формулу", theme.ContentAddIcon(), func() { a.formulaDialog(-1) })
	a.fEdit = widget.NewButtonWithIcon("Изменить", theme.DocumentCreateIcon(), func() { a.formulaDialog(a.fSel) })
	a.fDel = widget.NewButtonWithIcon("Удалить", theme.DeleteIcon(), a.deleteFormula)
	a.fDel.Importance = widget.DangerImportance
	a.fEdit.Disable()
	a.fDel.Disable()
	return container.NewBorder(container.NewPadded(container.NewHBox(add, a.fEdit, a.fDel)), nil, nil, nil, a.formulaTable)
}

func (a *App) paramDefsPane() fyne.CanvasObject {
	headers := []string{"Имя", "Название", "Значение", "Ед."}
	a.paramTable = widget.NewTable(
		func() (int, int) { return len(a.mdl.Params), 4 },
		newCell,
		func(id widget.TableCellID, o fyne.CanvasObject) {
			l := o.(*widget.Label)
			if id.Row >= len(a.mdl.Params) {
				return
			}
			p := a.mdl.Params[id.Row]
			l.Importance = widget.MediumImportance
			if a.bad[p.ID] {
				l.Importance = widget.DangerImportance
			}
			l.Alignment = fyne.TextAlignLeading
			switch id.Col {
			case 0:
				l.SetText(p.ID)
			case 1:
				l.SetText(p.Title)
			case 2:
				l.Alignment = fyne.TextAlignTrailing
				l.SetText(fnum(p.Value))
			case 3:
				l.SetText(p.Unit)
			}
		})
	a.paramTable.ShowHeaderRow = true
	a.paramTable.CreateHeader = headerCell
	a.paramTable.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		l := o.(*widget.Label)
		l.Alignment = fyne.TextAlignLeading
		if id.Col == 2 {
			l.Alignment = fyne.TextAlignTrailing
		}
		if id.Col >= 0 && id.Col < len(headers) {
			l.SetText(headers[id.Col])
		}
	}
	for c, w := range []float32{110, 380, 120, 80} {
		a.paramTable.SetColumnWidth(c, w)
	}
	a.paramTable.OnSelected = func(id widget.TableCellID) {
		a.pSel = id.Row
		a.pEdit.Enable()
		a.pDel.Enable()
	}

	add := widget.NewButtonWithIcon("Добавить параметр", theme.ContentAddIcon(), func() { a.paramDialog(-1) })
	a.pEdit = widget.NewButtonWithIcon("Изменить", theme.DocumentCreateIcon(), func() { a.paramDialog(a.pSel) })
	a.pDel = widget.NewButtonWithIcon("Удалить", theme.DeleteIcon(), a.deleteParam)
	a.pDel.Importance = widget.DangerImportance
	a.pEdit.Disable()
	a.pDel.Disable()
	return container.NewBorder(container.NewPadded(container.NewHBox(add, a.pEdit, a.pDel)), nil, nil, nil, a.paramTable)
}

// refreshModelViews обновляет таблицы и сводку ошибок после любого изменения модели.
func (a *App) refreshModelViews() {
	probs := a.mdl.Validate()
	a.bad = map[string]bool{}
	for _, p := range probs {
		a.bad[p.ID] = true
	}
	a.fSel, a.pSel = -1, -1
	if a.formulaTable != nil {
		a.formulaTable.UnselectAll()
		a.formulaTable.Refresh()
		a.fEdit.Disable()
		a.fDel.Disable()
	}
	if a.paramTable != nil {
		a.paramTable.UnselectAll()
		a.paramTable.Refresh()
		a.pEdit.Disable()
		a.pDel.Disable()
	}
	var segs []widget.RichTextSegment
	if len(probs) == 0 {
		segs = append(segs, &widget.TextSegment{
			Text:  fmt.Sprintf("✓ Модель корректна: формул — %d, параметров — %d", len(a.mdl.Formulas), len(a.mdl.Params)),
			Style: widget.RichTextStyle{ColorName: theme.ColorNameSuccess},
		})
	} else {
		segs = append(segs, &widget.TextSegment{
			Text:  fmt.Sprintf("⚠ Ошибок в модели: %d", len(probs)),
			Style: widget.RichTextStyle{ColorName: theme.ColorNameError, TextStyle: fyne.TextStyle{Bold: true}},
		})
		for i, p := range probs {
			if i == 4 {
				break
			}
			segs = append(segs, &widget.TextSegment{Text: "   " + p.String(),
				Style: widget.RichTextStyle{ColorName: theme.ColorNameError}})
		}
	}
	a.problems.Segments = segs
	a.problems.Refresh()
}

func (a *App) deleteFormula() {
	if a.fSel < 0 || a.fSel >= len(a.mdl.Formulas) {
		return
	}
	id := a.mdl.Formulas[a.fSel].ID
	if users := a.mdl.UsedBy(id); len(users) > 0 {
		dialog.ShowInformation("Нельзя удалить", "«"+id+"» используется в формулах: "+strings.Join(users, ", ")+".", a.win)
		return
	}
	dialog.ShowConfirm("Удалить формулу?", "Формула «"+id+"» будет удалена.", func(ok bool) {
		if ok {
			a.apply(a.mdl.WithoutFormula(id), "Формула «"+id+"» удалена")
		}
	}, a.win)
}

func (a *App) deleteParam() {
	if a.pSel < 0 || a.pSel >= len(a.mdl.Params) {
		return
	}
	id := a.mdl.Params[a.pSel].ID
	if users := a.mdl.UsedBy(id); len(users) > 0 {
		dialog.ShowInformation("Нельзя удалить", "«"+id+"» используется в формулах: "+strings.Join(users, ", ")+".", a.win)
		return
	}
	dialog.ShowConfirm("Удалить параметр?", "Параметр «"+id+"» будет удалён.", func(ok bool) {
		if ok {
			a.apply(a.mdl.WithoutParam(id), "Параметр «"+id+"» удалён")
		}
	}, a.win)
}

func (a *App) resetModel() {
	dialog.ShowConfirm("Сбросить к методичке?",
		"Все свои формулы и параметры будут заменены формулами методички (4)–(15).\nПрежняя версия файла останется в копии .bak.",
		func(ok bool) {
			if ok {
				a.apply(engine.Default(), "Восстановлена модель по методичке")
			}
		}, a.win)
}

// ───────────────────────────── диалоги ─────────────────────────────

// renderPreview показывает результат проверки: красным — ошибка, зелёным — значение.
func renderPreview(rt *widget.RichText, pv engine.Preview, unit string, neutralText string) {
	var seg *widget.TextSegment
	switch {
	case neutralText != "":
		seg = &widget.TextSegment{Text: neutralText, Style: widget.RichTextStyle{ColorName: theme.ColorNamePlaceHolder}}
	case !pv.OK:
		seg = &widget.TextSegment{Text: "✗ " + pv.Message,
			Style: widget.RichTextStyle{ColorName: theme.ColorNameError, TextStyle: fyne.TextStyle{Bold: true}}}
	case pv.HasValue:
		seg = &widget.TextSegment{Text: fmt.Sprintf("✓ %s     ←     %s",
			withUnit("= "+engine.FormatNumber(pv.Value), unit), pv.Expanded),
			Style: widget.RichTextStyle{ColorName: theme.ColorNameSuccess, TextStyle: fyne.TextStyle{Bold: true}}}
	default:
		seg = &widget.TextSegment{Text: "✓ " + pv.Message, Style: widget.RichTextStyle{ColorName: theme.ColorNameSuccess}}
	}
	rt.Segments = []widget.RichTextSegment{seg}
	rt.Refresh()
}

type formulaForm struct {
	id, title, expr, unit, ref, group *widget.Entry
	final                             *widget.Check
	preview                           *widget.RichText
	dlg                               *dialog.FormDialog
}

// formulaDialog открывает форму формулы. idx < 0 — новая формула.
// Кнопка «Сохранить» активна, только пока engine подтверждает, что формула корректна.
func (a *App) formulaDialog(idx int) *formulaForm {
	var f engine.Formula
	old := ""
	if idx >= 0 && idx < len(a.mdl.Formulas) {
		f = a.mdl.Formulas[idx]
		old = f.ID
	}
	ff := &formulaForm{
		id: widget.NewEntry(), title: widget.NewEntry(), expr: widget.NewMultiLineEntry(),
		unit: widget.NewEntry(), ref: widget.NewEntry(), group: widget.NewEntry(),
		final:   widget.NewCheck("Итоговый результат (выделяется в отчёте)", nil),
		preview: widget.NewRichText(),
	}
	ff.preview.Wrapping = fyne.TextWrapWord
	ff.id.SetText(f.ID)
	ff.title.SetText(f.Title)
	ff.expr.SetText(f.Expr)
	ff.unit.SetText(f.Unit)
	ff.ref.SetText(f.Ref)
	ff.group.SetText(f.Group)
	ff.final.SetChecked(f.Final)
	ff.id.SetPlaceHolder("например: Plan")
	ff.title.SetPlaceHolder("как будет называться в отчёте")
	ff.expr.SetPlaceHolder("например: Bres * 1.05")
	ff.expr.SetMinRowsVisible(3)
	ff.expr.Wrapping = fyne.TextWrapWord
	ff.unit.SetPlaceHolder("кг")
	ff.ref.SetPlaceHolder("например: (5)")
	ff.group.SetPlaceHolder("пусто — считать всегда")

	candidate := func() engine.Formula {
		t := strings.TrimSpace
		fm := engine.Formula{ID: t(ff.id.Text), Title: t(ff.title.Text), Expr: t(ff.expr.Text),
			Unit: t(ff.unit.Text), Ref: t(ff.ref.Text), Group: t(ff.group.Text), Final: ff.final.Checked}
		if fm.Title == "" {
			fm.Title = fm.ID
		}
		return fm
	}
	check := func() error {
		fm := candidate()
		if fm.ID == "" && fm.Expr == "" {
			renderPreview(ff.preview, engine.Preview{}, "", "Введите имя и выражение, например:  Plan = Bres * 1.05")
			return errors.New("введите имя и выражение")
		}
		pv := a.mdl.PreviewFormula(fm, old, a.inputs(), len(a.items) > 0)
		renderPreview(ff.preview, pv, fm.Unit, "")
		if !pv.OK {
			return errors.New(pv.Message)
		}
		return nil
	}
	ff.expr.Validator = func(string) error { return check() }
	revalidate := func(string) { _ = ff.expr.Validate() }
	for _, e := range []*widget.Entry{ff.id, ff.title, ff.unit, ff.ref, ff.group} {
		e.OnChanged = revalidate
	}
	ff.final.OnChanged = func(bool) { _ = ff.expr.Validate() }

	var names []string
	for _, v := range a.mdl.Names() {
		names = append(names, v.ID)
	}
	avail := widget.NewLabel(strings.Join(names, ", "))
	avail.Wrapping = fyne.TextWrapWord
	avail.Importance = widget.LowImportance

	idItem := widget.NewFormItem("Имя", ff.id)
	idItem.HintText = "латиница, цифры и «_», без пробелов; регистр важен"
	exprItem := widget.NewFormItem("Выражение", ff.expr)
	exprItem.HintText = "+  -  *  /  ^  ( )   min  max  abs  sqrt  round  if(условие; да; нет)"
	items := []*widget.FormItem{
		idItem, widget.NewFormItem("Название", ff.title), exprItem,
		widget.NewFormItem("Единица", ff.unit), widget.NewFormItem("Обозначение", ff.ref),
		widget.NewFormItem("Группа", ff.group), widget.NewFormItem("", ff.final),
		widget.NewFormItem("Проверка", ff.preview), widget.NewFormItem("Доступные имена", avail),
	}

	title := "Новая формула"
	if old != "" {
		title = "Формула «" + old + "»"
	}
	ff.dlg = dialog.NewForm(title, "Сохранить", "Отмена", items, func(ok bool) {
		if !ok {
			return
		}
		if err := check(); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		fm := candidate()
		a.apply(a.mdl.WithFormula(fm, old), "Формула «"+fm.ID+"» сохранена")
	}, a.win)
	ff.dlg.Resize(fyne.NewSize(760, 640))
	ff.dlg.Show()
	_ = ff.expr.Validate()
	return ff
}

type paramForm struct {
	id, title, unit, value, note *widget.Entry
	preview                      *widget.RichText
	dlg                          *dialog.FormDialog
}

// paramDialog открывает форму параметра. idx < 0 — новый параметр.
func (a *App) paramDialog(idx int) *paramForm {
	var p engine.Param
	old := ""
	if idx >= 0 && idx < len(a.mdl.Params) {
		p = a.mdl.Params[idx]
		old = p.ID
	}
	pf := &paramForm{
		id: widget.NewEntry(), title: widget.NewEntry(), unit: widget.NewEntry(),
		value: widget.NewEntry(), note: widget.NewEntry(), preview: widget.NewRichText(),
	}
	pf.preview.Wrapping = fyne.TextWrapWord
	pf.id.SetText(p.ID)
	pf.title.SetText(p.Title)
	pf.unit.SetText(p.Unit)
	if old != "" {
		pf.value.SetText(fnum(p.Value))
	}
	pf.note.SetText(p.Note)
	pf.id.SetPlaceHolder("например: Kz")
	pf.title.SetPlaceHolder("например: Коэффициент запаса")
	pf.unit.SetPlaceHolder("%, кг, …")
	pf.value.SetPlaceHolder("например: 1.05")
	pf.note.SetPlaceHolder("необязательно")

	candidate := func() (engine.Param, error) {
		t := strings.TrimSpace
		v, err := parseNum(pf.value.Text)
		if err != nil {
			return engine.Param{}, errors.New("значение: введите число, например 0.95")
		}
		np := engine.Param{ID: t(pf.id.Text), Title: t(pf.title.Text), Unit: t(pf.unit.Text), Note: t(pf.note.Text), Value: v}
		if np.Title == "" {
			np.Title = np.ID
		}
		return np, nil
	}
	check := func() error {
		if strings.TrimSpace(pf.id.Text) == "" && strings.TrimSpace(pf.value.Text) == "" {
			renderPreview(pf.preview, engine.Preview{}, "", "Введите имя и значение, например:  Kz = 1.05")
			return errors.New("введите имя и значение")
		}
		np, err := candidate()
		if err != nil {
			renderPreview(pf.preview, engine.Preview{Message: err.Error()}, "", "")
			return err
		}
		pv := a.mdl.PreviewParam(np, old, a.inputs(), len(a.items) > 0)
		renderPreview(pf.preview, pv, "", "")
		if !pv.OK {
			return errors.New(pv.Message)
		}
		return nil
	}
	pf.value.Validator = func(string) error { return check() }
	revalidate := func(string) { _ = pf.value.Validate() }
	for _, e := range []*widget.Entry{pf.id, pf.title, pf.unit, pf.note} {
		e.OnChanged = revalidate
	}

	idItem := widget.NewFormItem("Имя", pf.id)
	idItem.HintText = "латиница, цифры и «_», без пробелов; регистр важен"
	valItem := widget.NewFormItem("Значение", pf.value)
	valItem.HintText = "по умолчанию; перед расчётом его можно изменить"
	items := []*widget.FormItem{
		idItem, widget.NewFormItem("Название", pf.title), widget.NewFormItem("Единица", pf.unit),
		valItem, widget.NewFormItem("Подсказка", pf.note), widget.NewFormItem("Проверка", pf.preview),
	}
	title := "Новый параметр"
	if old != "" {
		title = "Параметр «" + old + "»"
	}
	pf.dlg = dialog.NewForm(title, "Сохранить", "Отмена", items, func(ok bool) {
		if !ok {
			return
		}
		if err := check(); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		np, _ := candidate()
		a.apply(a.mdl.WithParam(np, old), "Параметр «"+np.ID+"» сохранён")
		a.values[np.ID] = np.Value
		a.rebuildParams()
	}, a.win)
	pf.dlg.Resize(fyne.NewSize(640, 480))
	pf.dlg.Show()
	_ = pf.value.Validate()
	return pf
}

var _ = layout.NewSpacer
