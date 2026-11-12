package main

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"breadcalc/recipe"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type recipeDocument = recipe.Document

func (m model) document() recipeDocument {
	d := recipeDocument{Version: recipe.DocumentVersion, Name: m.name, Items: []recipe.Ingredient{}, Params: map[string]float64{}, Groups: map[string]bool{}}
	for _, item := range m.recipe {
		d.Items = append(d.Items, recipe.Ingredient{Name: item.Name, Mass: item.Mass, Moisture: item.Moisture})
	}
	for _, p := range m.mdl.Params {
		d.Params[p.ID] = m.values[p.ID]
	}
	for _, g := range m.mdl.Groups() {
		d.Groups[g] = m.groups[g]
	}
	return d
}

func (m *model) markSaved() { d := m.document(); m.baseline = &d }
func (m model) hasChanges() bool {
	return m.baseline != nil && !reflect.DeepEqual(*m.baseline, m.document())
}
func (m *model) invalidateReport() { m.report = ""; m.vp.SetContent("") }

func (m *model) requestAction(action string) tea.Cmd {
	if m.hasChanges() || (action == "quit" && (m.screen == scEdit || m.screen == scParams || m.screen == scDef)) {
		m.pending, m.confirmBack, m.screen = action, m.screen, scConfirm
		m.setMsg("Несохранённые изменения могут быть потеряны. Продолжить без сохранения?", true)
		return nil
	}
	return m.performAction(action)
}

func (m *model) performAction(action string) tea.Cmd {
	switch action {
	case "quit":
		return tea.Quit
	case "open":
		return m.openFileInput("open")
	case "preset":
		p := presets()[m.pendingIndex]
		m.name = p.Name
		m.recipe = append([]Ingredient(nil), p.Recipe...)
		m.resetValues(p.Overrides, p.Groups)
	case "new":
		m.name, m.recipe = "Новая рецептура", nil
		m.resetValues(nil, nil)
	case "overwrite":
		m.saveRecipeFile(m.pendingPath, true)
		return nil
	default:
		return nil
	}
	m.cursor, m.screen = 0, scRecipe
	m.invalidateReport()
	m.markSaved()
	m.setMsg("", false)
	return nil
}

func (m model) updateConfirmation(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "д", "Д":
		m.screen = m.confirmBack
		m.setMsg("", false)
		return m, m.performAction(m.pending)
	case "n", "N", "н", "Н", "esc":
		m.screen = m.confirmBack
		m.setMsg("Отменено.", false)
	}
	return m, nil
}

func (m *model) openFileInput(action string) tea.Cmd {
	m.fileBack, m.fileAction, m.screen = m.screen, action, scFile
	m.fileInput = textinput.New()
	m.fileInput.Prompt = ""
	m.fileInput.CharLimit = 4096
	m.fileInput.Width = 60
	if action == "rename" {
		m.fileInput.SetValue(m.name)
	}
	m.fileInput.Focus()
	m.setMsg("", false)
	return textinput.Blink
}

func (m model) viewFile() string {
	title := map[string]string{"open": "Открыть рецептуру: путь к JSON", "save": "Сохранить рецептуру: путь к JSON", "rename": "Название рецептуры"}[m.fileAction]
	return m.header(title) + m.fileInput.View() + m.footer("enter подтвердить • esc отмена")
}

func (m model) updateFile(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "esc":
			m.screen = m.fileBack
			m.setMsg("", false)
			return m, nil
		case "enter":
			value := strings.TrimSpace(m.fileInput.Value())
			if value == "" {
				m.setMsg("Введите значение", true)
				return m, nil
			}
			switch m.fileAction {
			case "rename":
				m.name = value
				m.invalidateReport()
				m.screen = m.fileBack
			case "open":
				if err := m.loadRecipeFile(value); err != nil {
					m.setMsg("Не удалось открыть: "+err.Error(), true)
				}
			case "save":
				if _, err := os.Lstat(value); err == nil {
					m.pendingPath, m.pending, m.confirmBack, m.screen = value, "overwrite", scFile, scConfirm
					m.setMsg("Файл уже существует. Перезаписать?", true)
				} else if !os.IsNotExist(err) {
					m.setMsg("Не удалось проверить файл: "+err.Error(), true)
				} else {
					m.saveRecipeFile(value, false)
				}
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.fileInput, cmd = m.fileInput.Update(msg)
	return m, cmd
}

func (m *model) loadRecipeFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	d, readErr := recipe.ReadDocument(f)
	closeErr := f.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if len(d.Params) != len(m.mdl.Params) || len(d.Groups) != len(m.mdl.Groups()) {
		return fmt.Errorf("параметры или группы не совпадают с текущей моделью")
	}
	for _, p := range m.mdl.Params {
		if _, ok := d.Params[p.ID]; !ok {
			return fmt.Errorf("отсутствует параметр текущей модели «%s»", p.ID)
		}
	}
	for _, g := range m.mdl.Groups() {
		if _, ok := d.Groups[g]; !ok {
			return fmt.Errorf("отсутствует группа текущей модели «%s»", g)
		}
	}
	m.name, m.recipe = d.Name, nil
	for _, item := range d.Items {
		m.recipe = append(m.recipe, Ingredient{Name: item.Name, Mass: item.Mass, Moisture: item.Moisture})
	}
	m.resetValues(d.Params, d.Groups)
	m.cursor, m.screen = 0, scRecipe
	m.invalidateReport()
	m.markSaved()
	m.setMsg("Рецептура открыта: "+path, false)
	return nil
}

func (m *model) saveRecipeFile(path string, overwrite bool) {
	var buf bytes.Buffer
	if err := recipe.WriteDocument(&buf, m.document()); err != nil {
		m.setMsg("Не удалось сохранить: "+err.Error(), true)
		return
	}
	if overwrite {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			m.setMsg("Можно перезаписать только обычный файл", true)
			return
		}
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".breadcalc-recipe-*")
	if err != nil {
		m.setMsg("Не удалось сохранить: "+err.Error(), true)
		return
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	_, err = io.Copy(f, &buf)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		if overwrite {
			err = os.Rename(tmp, path)
		} else {
			err = os.Link(tmp, path)
		}
	}
	if err != nil {
		m.setMsg("Не удалось сохранить: "+err.Error(), true)
		return
	}
	m.markSaved()
	m.screen = m.fileBack
	m.setMsg("Рецептура сохранена: "+path, false)
}

func (m *model) savePrintableReport() {
	if m.report == "" {
		m.setMsg("Сначала выполните расчёт", true)
		return
	}
	plain := ansiRe.ReplaceAllString(m.report, "")
	content := "<!doctype html><html lang=\"ru\"><head><meta charset=\"utf-8\"><title>Расчёт BreadCalc</title><style>body{font-family:sans-serif;margin:2em}pre{white-space:pre-wrap;overflow-wrap:anywhere;font-size:11pt}@media print{body{margin:0}}</style></head><body><pre>" + html.EscapeString(plain) + "</pre></body></html>\n"
	name := "raschet_" + time.Now().Format("20060102_150405") + ".html"
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		m.setMsg("Не удалось сохранить: "+err.Error(), true)
		return
	}
	_, err = io.WriteString(f, content)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		m.setMsg("Не удалось сохранить: "+err.Error(), true)
		return
	}
	m.setMsg("Отчёт сохранён в "+name+"; откройте его в браузере для печати или PDF.", false)
}
