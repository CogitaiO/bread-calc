// Package gui — графический интерфейс BreadCalc на Fyne.
// Вся логика расчёта и проверки живёт в пакете engine; здесь только отображение.
package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type appTheme struct {
	fyne.Theme
	force *fyne.ThemeVariant // nil — следовать настройке системы
}

// NewTheme — тема приложения, следует светлой/тёмной настройке системы.
func NewTheme() fyne.Theme { return &appTheme{Theme: theme.DefaultTheme()} }

// NewThemeVariant — та же тема, но с принудительным вариантом (для скриншотов и тестов).
func NewThemeVariant(v fyne.ThemeVariant) fyne.Theme {
	return &appTheme{Theme: theme.DefaultTheme(), force: &v}
}

func (t *appTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if t.force != nil {
		v = *t.force
	}
	dark := v == theme.VariantDark
	switch n {
	case theme.ColorNamePrimary, theme.ColorNameHyperlink:
		if dark {
			return color.NRGBA{R: 0xFB, G: 0x92, B: 0x3C, A: 0xFF}
		}
		return color.NRGBA{R: 0xC2, G: 0x41, B: 0x0C, A: 0xFF}
	case theme.ColorNameFocus, theme.ColorNameSelection:
		if dark {
			return color.NRGBA{R: 0xFB, G: 0x92, B: 0x3C, A: 0x40}
		}
		return color.NRGBA{R: 0xC2, G: 0x41, B: 0x0C, A: 0x30}
	case theme.ColorNameSuccess:
		if dark {
			return color.NRGBA{R: 0x4A, G: 0xDE, B: 0x80, A: 0xFF}
		}
		return color.NRGBA{R: 0x15, G: 0x80, B: 0x3D, A: 0xFF}
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		// подписи «низкой важности» в стандартной теме слишком бледные
		if dark {
			return color.NRGBA{R: 0x9C, G: 0xA3, B: 0xAF, A: 0xFF}
		}
		return color.NRGBA{R: 0x7A, G: 0x80, B: 0x8C, A: 0xFF}
	case theme.ColorNameForegroundOnPrimary:
		if dark {
			return color.NRGBA{R: 0x1F, G: 0x14, B: 0x0A, A: 0xFF}
		}
		return color.White
	}
	return t.Theme.Color(n, v)
}
