// Команда breadcalc-gui запускает графический интерфейс BreadCalc.
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"breadcalc/engine"
	"breadcalc/gui"
)

func main() {
	a := app.NewWithID("io.github.cogitaio.breadcalc")
	a.Settings().SetTheme(gui.NewTheme())
	w := a.NewWindow("BreadCalc")
	w.Resize(fyne.NewSize(1180, 760))

	path := engine.ModelPath()
	mdl, err := engine.LoadOrCreate(path)
	warn := ""
	if err != nil {
		mdl = engine.Default()
		warn = "Файл модели не прочитан:\n" + err.Error() +
			"\n\nИспользуется встроенная модель. При первом сохранении она заменит файл (старый останется в .bak)."
	}
	ui := gui.New(w, mdl, path)
	if warn != "" {
		ui.Warn("Файл модели повреждён", warn)
	}
	w.ShowAndRun()
}
