package gui

import (
	"fmt"
	"html"
	"strings"

	"breadcalc/engine"
	"breadcalc/recipe"
)

func withUnit(s, unit string) string {
	if unit == "" {
		return s
	}
	return s + " " + unit
}

// plainReport — подробный отчёт без оформления (для файла и буфера обмена).
func plainReport(name string, items []recipe.Ingredient, mdl *engine.Model, in engine.Inputs, res *engine.Result) string {
	var b strings.Builder
	rule := strings.Repeat("─", 72)

	fmt.Fprintf(&b, "ПОДРОБНЫЙ ПОШАГОВЫЙ РАСЧЁТ\n%s\n\nИзделие: %s\n\nРецептура\n", rule, name)
	for i, ing := range items {
		fmt.Fprintf(&b, "  %2d. %s — %.2f кг, %.2f %%\n", i+1, ing.Name, ing.Mass, ing.Moisture)
	}
	fmt.Fprintf(&b, "      Σ масса m_c = %s,  Σ(масса·влажность) = %s\n\nПараметры расчёта\n",
		engine.FormatNumber(in.Mc), engine.FormatNumber(in.SumMW))
	for _, p := range mdl.Params {
		v, ok := in.Params[p.ID]
		if !ok {
			v = p.Value
		}
		fmt.Fprintf(&b, "  %s: %s\n", p.Title, withUnit(engine.FormatNumber(v), p.Unit))
	}
	for _, g := range mdl.Groups() {
		state := "нет"
		if in.Groups[g] {
			state = "да"
		}
		fmt.Fprintf(&b, "  %-40s %s\n", "Считать: "+g, state)
	}
	fmt.Fprintf(&b, "\n%s\n\n", rule)

	for _, s := range res.Steps {
		head := "Формула"
		if s.Formula.Ref != "" {
			head += " " + s.Formula.Ref
		}
		fmt.Fprintf(&b, "%s: %s\n   %s = %s\n   = %s\n   → %s = %s\n\n", head, s.Formula.Title,
			s.Formula.ID, s.Formula.Expr, s.Expanded,
			s.Formula.ID, withUnit(engine.FormatNumber(s.Value), s.Formula.Unit))
	}
	b.WriteString(rule + "\n")
	for _, s := range res.Steps {
		if s.Formula.Final {
			fmt.Fprintf(&b, "\n%s\n%s = %s\n", strings.ToUpper(s.Formula.Title), s.Formula.ID,
				withUnit(fmt.Sprintf("%.2f", s.Value), s.Formula.Unit))
		}
	}
	return b.String()
}

// printableReport можно открыть в браузере и напечатать или сохранить как PDF.
func printableReport(report string) string {
	return "<!doctype html>\n<html lang=\"ru\"><head><meta charset=\"utf-8\">" +
		"<title>Расчёт BreadCalc</title><style>body{font-family:sans-serif;margin:2em}" +
		"pre{white-space:pre-wrap;overflow-wrap:anywhere;font-size:11pt}" +
		"@media print{body{margin:0}}</style></head><body><pre>" +
		html.EscapeString(report) + "</pre></body></html>\n"
}
