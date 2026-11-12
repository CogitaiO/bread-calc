package engine

import (
	"fmt"
	"strings"
)

// Preview — результат пробной проверки формулы или параметра до сохранения.
type Preview struct {
	OK       bool    // можно сохранять
	Message  string  // что показать пользователю: причина ошибки или пояснение
	Value    float64 // значение формулы на текущей рецептуре (если HasValue)
	HasValue bool
	Expanded string // выражение с подставленными числами (если HasValue)
}

// NewProblems возвращает ошибки, которые появились только в cand
// (уже существовавшие в m не считаются: их можно исправлять по одной).
func (m *Model) NewProblems(cand *Model) []Problem {
	base := map[string]bool{}
	for _, p := range m.Validate() {
		base[p.Kind+"|"+p.String()] = true
	}
	var out []Problem
	for _, p := range cand.Validate() {
		if !base[p.Kind+"|"+p.String()] {
			out = append(out, p)
		}
	}
	return out
}

func finalsText(res *Result) string {
	var parts []string
	for _, s := range res.Steps {
		if s.Formula.Final {
			parts = append(parts, strings.TrimSpace(fmt.Sprintf("%s = %.2f %s", s.Formula.ID, s.Value, s.Formula.Unit)))
		}
	}
	return strings.Join(parts, "; ")
}

// PreviewFormula проверяет формулу f (она заменяет oldID, а для новой oldID == "")
// и, если рецептура задана, считает её значение на входных данных in.
func (m *Model) PreviewFormula(f Formula, oldID string, in Inputs, haveRecipe bool) Preview {
	cand := m.WithFormula(f, oldID)
	if np := m.NewProblems(cand); len(np) > 0 {
		return Preview{Message: np[0].String()}
	}
	if !haveRecipe {
		return Preview{OK: true, Message: "Выражение верно. Чтобы увидеть значение, добавьте сырьё в рецептуру."}
	}
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
			return Preview{OK: true, Message: "Выражение верно (в других формулах модели есть ошибки — значение не вычислено)."}
		}
		return Preview{Message: err.Error()}
	}
	for _, s := range res.Steps {
		if s.Formula.ID == f.ID {
			return Preview{OK: true, Value: s.Value, HasValue: true, Expanded: s.Expanded}
		}
	}
	return Preview{OK: true, Message: "Выражение верно."}
}

// PreviewParam проверяет параметр p (заменяет oldID) и показывает итоги расчёта с его значением.
func (m *Model) PreviewParam(p Param, oldID string, in Inputs, haveRecipe bool) Preview {
	cand := m.WithParam(p, oldID)
	if np := m.NewProblems(cand); len(np) > 0 {
		return Preview{Message: np[0].String()}
	}
	if !haveRecipe {
		return Preview{OK: true, Message: "Параметр корректен."}
	}
	vals := map[string]float64{}
	for k, v := range in.Params {
		vals[k] = v
	}
	vals[p.ID] = p.Value
	in.Params = vals
	res, err := cand.Calculate(in)
	if err != nil {
		if len(cand.Validate()) > 0 {
			return Preview{OK: true, Message: "Параметр корректен (в модели есть другие ошибки — итоги не вычислены)."}
		}
		return Preview{Message: err.Error()}
	}
	if t := finalsText(res); t != "" {
		return Preview{OK: true, Message: "Итоги с этим значением: " + t}
	}
	return Preview{OK: true, Message: "Параметр корректен."}
}
