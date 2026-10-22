package main

import "errors"

// Ingredient — сырьё рецептуры: наименование, масса (кг) и влажность (%).
type Ingredient struct {
	Name     string
	Mass     float64
	Moisture float64
}

// Params — нормативные параметры расчёта.
type Params struct {
	WTesta        float64 // влажность теста, %
	FlourMoisture float64 // базисная влажность муки, %
	GRazd         float64 // норма на разделку/подпыл, %
	GM            float64 // потери муки при приёмке
	GTM           float64 // потери от распыла муки и теста
	GCB           float64 // затраты сухих веществ на брожение
	GUp           float64 // упёк, %
	GUs           float64 // усушка, %
	GKr           float64 // крошка и лом, %
	GSht          float64 // отклонение штучной массы, %
	GBr           float64 // переработка брака, %
	CalcSuhariki  bool
	GSuhariki     float64 // норма потерь на сухарики, %
}

func DefaultParams() Params {
	return Params{
		WTesta: 47.5, FlourMoisture: 14.5, GRazd: 0.0,
		GM: 0.02, GTM: 0.05, GCB: 3.5, GUp: 10.0, GUs: 4.0,
		GKr: 0.03, GSht: 0.6, GBr: 0.02, GSuhariki: 30.0,
	}
}

// Result — все промежуточные и итоговые величины расчёта.
type Result struct {
	Mc, SumMW, Wcp, Bt            float64
	Pm, Ptm, Zbr, Zrazd           float64
	MassBeforeBake, Zup           float64
	MassHot, Zus                  float64
	MassCooled, Pkr               float64
	MassAfterKr, Psht             float64
	MassAfterSht, Pbr             float64
	Losses, Bres                  float64
	SuhLoss, SuhYield             float64
}

func Calculate(recipe []Ingredient, p Params) (Result, error) {
	var r Result
	if len(recipe) == 0 {
		return r, errors.New("рецептура пуста — добавьте хотя бы одно сырьё")
	}
	for _, ing := range recipe {
		r.Mc += ing.Mass
		r.SumMW += ing.Mass * ing.Moisture
	}
	if r.Mc <= 0 {
		return r, errors.New("суммарная масса сырья должна быть больше нуля")
	}
	if p.WTesta >= 100 || p.FlourMoisture >= 100 {
		return r, errors.New("влажность должна быть меньше 100 %")
	}

	r.Wcp = r.SumMW / r.Mc                                          // (5)
	r.Bt = r.Mc * (100 - r.Wcp) / (100 - p.WTesta)                  // (4)
	r.Pm = p.GM * (100 - p.FlourMoisture) / (100 - p.WTesta)        // (7)
	r.Ptm = p.GTM * (100 - p.FlourMoisture) / (100 - p.WTesta)      // (8)
	r.Zbr = (p.GCB * 0.95 * (r.Mc - 0.7) * (100 - r.Wcp)) /         // (9)
		(1.96 * 100 * (100 - p.WTesta))
	r.Zrazd = p.GRazd * (p.WTesta - p.FlourMoisture) / (100 - p.WTesta) // (10)

	r.MassBeforeBake = r.Bt - (r.Pm + r.Ptm + r.Zbr + r.Zrazd)
	r.Zup = p.GUp * r.MassBeforeBake / 100 // (11)
	r.MassHot = r.MassBeforeBake - r.Zup
	r.Zus = p.GUs * r.MassHot / 100 // (12)
	r.MassCooled = r.MassHot - r.Zus
	r.Pkr = p.GKr * r.MassCooled / 100 // (13)
	r.MassAfterKr = r.MassCooled - r.Pkr
	r.Psht = p.GSht * r.MassAfterKr / 100 // (14)
	r.MassAfterSht = r.MassAfterKr - r.Psht
	r.Pbr = p.GBr * r.MassAfterSht / 100 // (15)

	r.Losses = r.Pm + r.Ptm + r.Zbr + r.Zrazd + r.Zup + r.Zus + r.Pkr + r.Psht + r.Pbr
	r.Bres = r.Bt - r.Losses

	if p.CalcSuhariki {
		r.SuhLoss = r.Bres * p.GSuhariki / 100
		r.SuhYield = r.Bres - r.SuhLoss
	}
	return r, nil
}
