package engine

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed default_model.txt
var defaultText string

// DefaultText — текст встроенной модели (формулы (4)–(15) методички).
func DefaultText() string { return defaultText }

// Default возвращает встроенную модель.
func Default() *Model {
	m, err := Parse(defaultText)
	if err != nil {
		panic("встроенная модель повреждена: " + err.Error())
	}
	return m
}

func parseNumber(s string) (float64, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("некорректное число «%s»", s)
	}
	return v, nil
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "да":
		return true
	}
	return false
}

// Parse читает модель из текста. Семантика (имена, циклы) проверяется отдельно через Validate.
func Parse(text string) (*Model, error) {
	m := &Model{}
	for i, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		n := i + 1
		kw, rest, _ := strings.Cut(line, " ")
		parts := strings.Split(rest, "|")
		id, body, ok := strings.Cut(strings.TrimSpace(parts[0]), "=")
		if !ok {
			return nil, fmt.Errorf("строка %d: ожидается «ИМЯ = ...»", n)
		}
		id, body = strings.TrimSpace(id), strings.TrimSpace(body)
		field := func(k int) string {
			if k < len(parts) {
				return strings.TrimSpace(parts[k])
			}
			return ""
		}
		var extras []string
		if len(parts) > 3 {
			extras = parts[3:]
		}
		title := field(2)
		if title == "" {
			title = id
		}

		switch strings.ToLower(kw) {
		case "param":
			v, err := parseNumber(body)
			if err != nil {
				return nil, fmt.Errorf("строка %d: %v", n, err)
			}
			p := Param{ID: id, Value: v, Unit: field(1), Title: title}
			for _, ex := range extras {
				k, val, _ := strings.Cut(strings.TrimSpace(ex), "=")
				switch strings.TrimSpace(k) {
				case "":
				case "note":
					p.Note = strings.TrimSpace(val)
				default:
					return nil, fmt.Errorf("строка %d: неизвестное свойство «%s» у параметра", n, strings.TrimSpace(k))
				}
			}
			m.Params = append(m.Params, p)

		case "formula":
			f := Formula{ID: id, Expr: body, Unit: field(1), Title: title}
			for _, ex := range extras {
				k, val, _ := strings.Cut(strings.TrimSpace(ex), "=")
				switch strings.TrimSpace(k) {
				case "":
				case "ref":
					f.Ref = strings.TrimSpace(val)
				case "group":
					f.Group = strings.TrimSpace(val)
				case "final":
					f.Final = parseBool(val)
				default:
					return nil, fmt.Errorf("строка %d: неизвестное свойство «%s» у формулы", n, strings.TrimSpace(k))
				}
			}
			m.Formulas = append(m.Formulas, f)

		default:
			return nil, fmt.Errorf("строка %d: строка должна начинаться со слова param или formula", n)
		}
	}
	return m, nil
}

const header = `# BreadCalc: модель расчёта (параметры и формулы)
# Файл можно править вручную; порядок формул не важен — программа сама определит порядок вычислений.
# Комментарии при сохранении из программы не сохраняются.
#
# param   ИМЯ = значение | единица | название | note=подсказка
# formula ИМЯ = выражение | единица | название | ref=(5) | group=Группа | final=1

`

// Format записывает модель в текстовый формат.
func Format(m *Model) string {
	var b strings.Builder
	b.WriteString(header)
	for _, p := range m.Params {
		fmt.Fprintf(&b, "param %s = %s | %s | %s", p.ID, strconv.FormatFloat(p.Value, 'f', -1, 64), p.Unit, p.Title)
		if p.Note != "" {
			fmt.Fprintf(&b, " | note=%s", p.Note)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	for _, f := range m.Formulas {
		fmt.Fprintf(&b, "formula %s = %s | %s | %s", f.ID, f.Expr, f.Unit, f.Title)
		if f.Ref != "" {
			fmt.Fprintf(&b, " | ref=%s", f.Ref)
		}
		if f.Group != "" {
			fmt.Fprintf(&b, " | group=%s", f.Group)
		}
		if f.Final {
			b.WriteString(" | final=1")
		}
		b.WriteString("\n")
	}
	return b.String()
}

const localFile = "breadcalc-model.txt"

// ModelPath — где лежит файл модели. Если рядом с программой (в текущей папке)
// есть breadcalc-model.txt, используется он (переносимый режим); иначе файл
// хранится в папке настроек пользователя.
func ModelPath() string {
	if _, err := os.Stat(localFile); err == nil {
		return localFile
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return localFile
	}
	return filepath.Join(dir, "breadcalc", "model.txt")
}

// LoadOrCreate читает модель; если файла нет, создаёт его из встроенной модели.
func LoadOrCreate(path string) (*Model, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		m := Default()
		_ = Save(path, m) // не страшно, если папка недоступна для записи
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	m, err := Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// Save записывает модель атомарно и оставляет предыдущую версию в path+".bak".
func Save(path string, m *Model) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if old, err := os.ReadFile(path); err == nil {
		_ = os.WriteFile(path+".bak", old, 0o644)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(Format(m)), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
