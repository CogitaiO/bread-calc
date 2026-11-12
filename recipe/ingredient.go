package recipe

// Ingredient — сырьё рецептуры: наименование, масса (кг) и влажность (%).
type Ingredient struct {
	Name     string  `json:"name"`
	Mass     float64 `json:"mass"`
	Moisture float64 `json:"moisture"`
}
