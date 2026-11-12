package main

// Ingredient — сырьё рецептуры: наименование, масса (кг) и влажность (%).
type Ingredient struct {
	Name     string
	Mass     float64
	Moisture float64
}

// aggregates возвращает суммарную массу сырья m_c и Σ(масса·влажность).
// Сами формулы расчёта живут в пакете engine и в файле модели.
func aggregates(rec []Ingredient) (mc, sumMW float64) {
	for _, ing := range rec {
		mc += ing.Mass
		sumMW += ing.Mass * ing.Moisture
	}
	return
}
