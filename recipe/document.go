package recipe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
)

// Document хранит исходные данные расчёта; формулы хранятся отдельно.
type Document struct {
	Version int                `json:"version"`
	Name    string             `json:"name"`
	Items   []Ingredient       `json:"ingredients"`
	Params  map[string]float64 `json:"parameters"`
	Groups  map[string]bool    `json:"groups"`
}

const DocumentVersion = 1
const maxDocumentSize = 4 << 20

func (d Document) Validate() error {
	if d.Version != DocumentVersion {
		return fmt.Errorf("неподдерживаемая версия рецептуры: %d", d.Version)
	}
	if strings.TrimSpace(d.Name) == "" || d.Items == nil || d.Params == nil || d.Groups == nil {
		return fmt.Errorf("в рецептуре отсутствуют обязательные данные")
	}
	for i, item := range d.Items {
		if strings.TrimSpace(item.Name) == "" || math.IsNaN(item.Mass) || math.IsInf(item.Mass, 0) || item.Mass < 0 || math.IsNaN(item.Moisture) || math.IsInf(item.Moisture, 0) || item.Moisture < 0 || item.Moisture > 100 {
			return fmt.Errorf("некорректное сырьё в строке %d", i+1)
		}
	}
	for id, value := range d.Params {
		if strings.TrimSpace(id) == "" || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("некорректный параметр %q", id)
		}
	}
	for id := range d.Groups {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("некорректное имя группы")
		}
	}
	return nil
}

func ReadDocument(r io.Reader) (Document, error) {
	var d Document
	data, err := io.ReadAll(io.LimitReader(r, maxDocumentSize+1))
	if err != nil {
		return Document{}, fmt.Errorf("не удалось прочитать рецептуру: %w", err)
	}
	if len(data) > maxDocumentSize {
		return Document{}, fmt.Errorf("файл рецептуры слишком велик")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return Document{}, fmt.Errorf("не удалось прочитать рецептуру: %w", err)
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		return Document{}, fmt.Errorf("после рецептуры обнаружены лишние данные")
	}
	if err := d.Validate(); err != nil {
		return Document{}, err
	}
	return d, nil
}

func WriteDocument(w io.Writer, d Document) error {
	if err := d.Validate(); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return err
	}
	if buf.Len() > maxDocumentSize {
		return fmt.Errorf("файл рецептуры слишком велик")
	}
	n, err := w.Write(buf.Bytes())
	if err == nil && n != buf.Len() {
		return io.ErrShortWrite
	}
	return err
}
