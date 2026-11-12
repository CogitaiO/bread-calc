package gui

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"breadcalc/recipe"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

func (a *App) recipeDocument() (recipe.Document, error) {
	d := recipe.Document{Version: recipe.DocumentVersion, Name: a.name,
		Items: append([]recipe.Ingredient{}, a.items...), Params: map[string]float64{}, Groups: map[string]bool{}}
	for _, p := range a.mdl.Params {
		v, err := parseNum(a.paramEntries[p.ID].Text)
		if err != nil {
			return recipe.Document{}, fmt.Errorf("параметр «%s»: %w", p.Title, err)
		}
		d.Params[p.ID] = v
	}
	for _, g := range a.mdl.Groups() {
		d.Groups[g] = a.groups[g]
	}
	return d, d.Validate()
}

// checkRecipeModel prevents silently dropping values belonging to another model.
func (a *App) checkRecipeModel(d recipe.Document) error {
	if len(d.Params) != len(a.mdl.Params) || len(d.Groups) != len(a.mdl.Groups()) {
		return fmt.Errorf("параметры или группы рецептуры не совпадают с текущей моделью формул")
	}
	for _, p := range a.mdl.Params {
		if _, ok := d.Params[p.ID]; !ok {
			return fmt.Errorf("в рецептуре отсутствует параметр текущей модели «%s»", p.ID)
		}
	}
	for _, g := range a.mdl.Groups() {
		if _, ok := d.Groups[g]; !ok {
			return fmt.Errorf("в рецептуре отсутствует группа текущей модели «%s»", g)
		}
	}
	return nil
}

func (a *App) applyRecipeDocument(d recipe.Document) error {
	if err := d.Validate(); err != nil {
		return err
	}
	if err := a.checkRecipeModel(d); err != nil {
		return err
	}
	a.presetList.UnselectAll()
	a.presetIndex = -1
	a.name = d.Name
	a.items = append([]recipe.Ingredient(nil), d.Items...)
	a.resetValues(d.Params, d.Groups)
	a.afterRecipeChange()
	a.rebuildParams()
	a.setHeader(a.name, "Рецептура из файла")
	a.tabs.SelectIndex(0)
	a.baseline = a.currentRecipeState()
	return nil
}

func (a *App) openRecipe() {
	d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if r == nil {
			return
		}
		doc, readErr := recipe.ReadDocument(r)
		closeErr := r.Close()
		if readErr != nil {
			dialog.ShowError(readErr, a.win)
			return
		}
		if closeErr != nil {
			dialog.ShowError(closeErr, a.win)
			return
		}
		if err := a.applyRecipeDocument(doc); err != nil {
			dialog.ShowError(err, a.win)
		}
	}, a.win)
	d.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
	d.Show()
}

func (a *App) saveRecipe() {
	doc, err := a.recipeDocument()
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	var buf bytes.Buffer
	if err := recipe.WriteDocument(&buf, doc); err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	baseline := a.currentRecipeState()
	d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if w == nil {
			return
		}
		n, writeErr := w.Write(buf.Bytes())
		if writeErr == nil && n != buf.Len() {
			writeErr = io.ErrShortWrite
		}
		closeErr := w.Close()
		if writeErr != nil {
			dialog.ShowError(writeErr, a.win)
			return
		}
		if closeErr != nil {
			dialog.ShowError(closeErr, a.win)
			return
		}
		a.baseline = baseline
	}, a.win)
	d.SetFileName("receptura.json")
	d.SetFilter(storage.NewExtensionFileFilter([]string{".json"}))
	d.Show()
}

func (a *App) renameRecipe() {
	entry := widget.NewEntry()
	entry.SetText(a.name)
	entry.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("введите название")
		}
		return nil
	}
	dialog.ShowForm("Название рецептуры", "Сохранить", "Отмена", []*widget.FormItem{widget.NewFormItem("Название", entry)}, func(ok bool) {
		if !ok {
			return
		}
		a.name = strings.TrimSpace(entry.Text)
		a.invalidateResult()
		a.setHeader(a.name, "")
	}, a.win)
}
