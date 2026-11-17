package extensions

import (
	"text/template"

	"github.com/jedib0t/go-pretty/v6/text"
)

func FuncMap() template.FuncMap {
	return template.FuncMap{
		"blue":      Blue,
		"cyan":      Cyan,
		"green":     Green,
		"magenta":   Magenta,
		"purple":    Magenta,
		"red":       Red,
		"yellow":    Yellow,
		"white":     White,
		"hiblue":    HighBlue,
		"hicyan":    HighCyan,
		"higreen":   HighGreen,
		"himagenta": HighMagenta,
		"hipurple":  HighMagenta,
		"hired":     HighRed,
		"hiyellow":  HighYellow,
		"hiwhite":   HighWhite,
	}
}

func HighBlue(v interface{}) string {
	return text.FgHiBlue.Sprint(v)
}

func HighCyan(v interface{}) string {
	return text.FgHiCyan.Sprint(v)
}

func HighGreen(v interface{}) string {
	return text.FgHiGreen.Sprint(v)
}

func HighMagenta(v interface{}) string {
	return text.FgHiMagenta.Sprint(v)
}

func HighRed(v interface{}) string {
	return text.FgHiRed.Sprint(v)
}

func HighYellow(v interface{}) string {
	return text.FgHiYellow.Sprint(v)
}

func HighWhite(v interface{}) string {
	return text.FgHiWhite.Sprint(v)
}

func Blue(v interface{}) string {
	return text.FgBlue.Sprint(v)
}

func Cyan(v interface{}) string {
	return text.FgCyan.Sprint(v)
}

func Green(v interface{}) string {
	return text.FgGreen.Sprint(v)
}

func Magenta(v interface{}) string {
	return text.FgMagenta.Sprint(v)
}

func Red(v interface{}) string {
	return text.FgRed.Sprint(v)
}

func Yellow(v interface{}) string {
	return text.FgYellow.Sprint(v)
}

func White(v interface{}) string {
	return text.FgWhite.Sprint(v)
}
