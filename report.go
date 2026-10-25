package main

import (
	"fmt"
	"strings"

	"breadcalc/engine"
	"github.com/charmbracelet/lipgloss"
)

var (
	stSection = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFB86C"))
	stFormula = lipgloss.NewStyle().Foreground(lipgloss.Color("#8BE9FD")).Bold(true)
	stExpr    = lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9"))
	stValue   = lipgloss.NewStyle().Foreground(lipgloss.Color("#50FA7B")).Bold(true)
	stDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6272A4"))
	stBox     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#50FA7B")).Padding(0, 2)
)

func withUnit(s, unit string) string {
	if unit == "" {
		return s
	}
	return s + " " + unit
}

// BuildReport формирует подробный пошаговый отчёт (со стилями).
func BuildReport(name string, rec []Ingredient, mdl *engine.Model, in engine.Inputs, res *engine.Result) string {
	var b strings.Builder
	rule := stDim.Render(strings.Repeat("─", 72))

	fmt.Fprintf(&b, "%s\n%s\n\n", stSection.Render("ПОДРОБНЫЙ ПОШАГОВЫЙ РАСЧЁТ"), rule)
	fmt.Fprintf(&b, "Изделие: %s\n\n", stValue.Render(name))

	fmt.Fprintf(&b, "%s\n", stSection.Render("Рецептура"))
	for i, ing := range rec {
		fmt.Fprintf(&b, "  %2d. %-52s %8.2f кг  %6.2f %%\n", i+1, truncate(ing.Name, 52), ing.Mass, ing.Moisture)
	}
	fmt.Fprintf(&b, "      Σ масса m_c = %s,  Σ(масса·влажность) = %s\n\n",
		engine.FormatNumber(in.Mc), engine.FormatNumber(in.SumMW))

	fmt.Fprintf(&b, "%s\n", stSection.Render("Параметры расчёта"))
	for _, p := range mdl.Params {
		v, ok := in.Params[p.ID]
		if !ok {
			v = p.Value
		}
		fmt.Fprintf(&b, "  %-40s %s\n", truncate(p.Title, 40), withUnit(engine.FormatNumber(v), p.Unit))
	}
	for _, g := range mdl.Groups() {
		state := "нет"
		if in.Groups[g] {
			state = "да"
		}
		fmt.Fprintf(&b, "  %-40s %s\n", "Считать: "+g, state)
	}
	b.WriteString("\n" + rule + "\n\n")

	for _, s := range res.Steps {
		head := "Формула"
		if s.Formula.Ref != "" {
			head += " " + s.Formula.Ref
		}
		fmt.Fprintf(&b, "%s %s\n", stFormula.Render(head), s.Formula.Title)
		fmt.Fprintf(&b, "   %s\n", stExpr.Render(s.Formula.ID+" = "+s.Formula.Expr))
		fmt.Fprintf(&b, "   %s\n", stExpr.Render("= "+s.Expanded))
		fmt.Fprintf(&b, "   %s\n\n", stValue.Render("→ "+s.Formula.ID+" = "+withUnit(engine.FormatNumber(s.Value), s.Formula.Unit)))
	}

	b.WriteString(rule + "\n")
	for _, s := range res.Steps {
		if !s.Formula.Final {
			continue
		}
		b.WriteString("\n" + stBox.Render(fmt.Sprintf("%s\n%s = %s",
			strings.ToUpper(s.Formula.Title), s.Formula.ID,
			stValue.Render(withUnit(fmt.Sprintf("%.2f", s.Value), s.Formula.Unit)))) + "\n")
	}
	return b.String()
}

func truncate(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n-1]) + "…"
}
