package main

import (
	"fmt"
	"strings"

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

func step(b *strings.Builder, num, title, expr, result string) {
	fmt.Fprintf(b, "%s %s\n", stFormula.Render("Формула "+num), title)
	fmt.Fprintf(b, "   %s\n", stExpr.Render(expr))
	fmt.Fprintf(b, "   %s\n\n", stValue.Render("→ "+result))
}

// BuildReport формирует подробный пошаговый отчёт (со стилями).
func BuildReport(name string, rec []Ingredient, p Params, r Result) string {
	var b strings.Builder
	rule := stDim.Render(strings.Repeat("─", 72))

	fmt.Fprintf(&b, "%s\n%s\n\n", stSection.Render("ПОДРОБНЫЙ ПОШАГОВЫЙ РАСЧЁТ ПО ФОРМУЛАМ МЕТОДИЧКИ"), rule)
	fmt.Fprintf(&b, "Изделие: %s\n", stValue.Render(name))
	fmt.Fprintf(&b, "Влажность теста: %.1f %%, влажность муки: %.1f %%, разделка: %.1f %%\n\n",
		p.WTesta, p.FlourMoisture, p.GRazd)

	fmt.Fprintf(&b, "%s\n", stSection.Render("Рецептура"))
	for i, ing := range rec {
		fmt.Fprintf(&b, "  %2d. %-52s %8.2f кг  %6.2f %%\n", i+1, truncate(ing.Name, 52), ing.Mass, ing.Moisture)
	}
	b.WriteString("\n")

	step(&b, "(5)", "Средневзвешенная влажность сырья W_ср",
		fmt.Sprintf("W_ср = Σ(m·W) / m_c = %.2f / %.2f", r.SumMW, r.Mc),
		fmt.Sprintf("W_ср = %.2f %%", r.Wcp))
	step(&b, "(4)", "Расчётный выход теста B_т",
		fmt.Sprintf("B_т = %.2f · (100 − %.2f) / (100 − %.1f)", r.Mc, r.Wcp, p.WTesta),
		fmt.Sprintf("B_т = %.2f кг", r.Bt))
	step(&b, "(7)", "Потери муки при приёмке П_м",
		fmt.Sprintf("П_м = %g · (100 − %.1f) / (100 − %.1f)", p.GM, p.FlourMoisture, p.WTesta),
		fmt.Sprintf("П_м = %.3f кг", r.Pm))
	step(&b, "(8)", "Потери мучной пыли и теста П_тм",
		fmt.Sprintf("П_тм = %g · (100 − %.1f) / (100 − %.1f)", p.GTM, p.FlourMoisture, p.WTesta),
		fmt.Sprintf("П_тм = %.3f кг", r.Ptm))
	step(&b, "(9)", "Затраты сухих веществ на брожение З_бр",
		fmt.Sprintf("З_бр = [%g · 0.95 · (%.1f − 0.7) · (100 − %.2f)] / [1.96 · 100 · (100 − %.1f)]", p.GCB, r.Mc, r.Wcp, p.WTesta),
		fmt.Sprintf("З_бр = %.2f кг", r.Zbr))
	step(&b, "(10)", "Затраты муки при разделке теста З_разд",
		fmt.Sprintf("З_разд = %g · (%.1f − %.1f) / (100 − %.1f)", p.GRazd, p.WTesta, p.FlourMoisture, p.WTesta),
		fmt.Sprintf("З_разд = %.3f кг", r.Zrazd))
	step(&b, "(11)", "Упёк З_уп",
		fmt.Sprintf("Масса перед посадкой в печь = %.2f кг;  З_уп = %g %% от %.2f", r.MassBeforeBake, p.GUp, r.MassBeforeBake),
		fmt.Sprintf("З_уп = %.2f кг", r.Zup))
	step(&b, "(12)", "Усушка при остывании З_ус",
		fmt.Sprintf("Масса горячего хлеба = %.2f кг;  З_ус = %g %% от %.2f", r.MassHot, p.GUs, r.MassHot),
		fmt.Sprintf("З_ус = %.2f кг", r.Zus))
	step(&b, "(13)", "Потери крошки и лома П_кр",
		fmt.Sprintf("Масса остывшего хлеба = %.2f кг;  П_кр = %g %% от %.2f", r.MassCooled, p.GKr, r.MassCooled),
		fmt.Sprintf("П_кр = %.3f кг", r.Pkr))
	step(&b, "(14)", "Отклонение штучной массы П_шт",
		fmt.Sprintf("П_шт = %g %% от %.2f", p.GSht, r.MassAfterKr),
		fmt.Sprintf("П_шт = %.3f кг", r.Psht))
	step(&b, "(15)", "Потери от переработки брака П_бр",
		fmt.Sprintf("П_бр = %g %% от %.2f", p.GBr, r.MassAfterSht),
		fmt.Sprintf("П_бр = %.4f кг", r.Pbr))

	b.WriteString(rule + "\n")
	fmt.Fprintf(&b, "Сумма всех технологических потерь: %s\n\n", stValue.Render(fmt.Sprintf("%.2f кг", r.Losses)))
	b.WriteString(stBox.Render(fmt.Sprintf("ИТОГОВЫЙ ВЫХОД ИЗДЕЛИЯ (В_хл^расч)\nB_т − потери = %.2f − %.2f = %s",
		r.Bt, r.Losses, stValue.Render(fmt.Sprintf("%.2f кг", r.Bres)))) + "\n")

	if p.CalcSuhariki {
		b.WriteString("\n" + rule + "\n")
		fmt.Fprintf(&b, "%s\n", stSection.Render(fmt.Sprintf("РАСЧЁТ СУХАРИКОВ (норма затрат и потерь: %g %%)", p.GSuhariki)))
		fmt.Fprintf(&b, "Затраты и потери = %.2f · %g %% = %.2f кг\n", r.Bres, p.GSuhariki, r.SuhLoss)
		b.WriteString(stBox.Render(fmt.Sprintf("ИТОГОВЫЙ ВЫХОД СУХАРИКОВ\n%.2f − %.2f = %s",
			r.Bres, r.SuhLoss, stValue.Render(fmt.Sprintf("%.2f кг", r.SuhYield)))) + "\n")
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
