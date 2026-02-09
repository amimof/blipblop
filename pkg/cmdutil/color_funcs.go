package cmdutil

import (
	"github.com/fatih/color"
)

func FgBlack(s string) string     { return applyColor(s, color.FgBlack) }
func FgRed(s string) string       { return applyColor(s, color.FgRed) }
func FgGreen(s string) string     { return applyColor(s, color.FgGreen) }
func FgYellow(s string) string    { return applyColor(s, color.FgYellow) }
func FgBlue(s string) string      { return applyColor(s, color.FgBlue) }
func FgMagenta(s string) string   { return applyColor(s, color.FgMagenta) }
func FgCyan(s string) string      { return applyColor(s, color.FgCyan) }
func FgWhite(s string) string     { return applyColor(s, color.FgWhite) }
func FgHiBlack(s string) string   { return applyColor(s, color.FgHiBlack) }
func FgHiRed(s string) string     { return applyColor(s, color.FgHiRed) }
func FgHiGreen(s string) string   { return applyColor(s, color.FgHiGreen) }
func FgHiYellow(s string) string  { return applyColor(s, color.FgHiYellow) }
func FgHiBlue(s string) string    { return applyColor(s, color.FgHiBlue) }
func FgHiMagenta(s string) string { return applyColor(s, color.FgHiMagenta) }
func FgHiCyan(s string) string    { return applyColor(s, color.FgHiCyan) }
func FgHiWhite(s string) string   { return applyColor(s, color.FgHiWhite) }

func BgBlack(s string) string     { return applyColor(s, color.BgBlack) }
func BgRed(s string) string       { return applyColor(s, color.BgRed) }
func BgGreen(s string) string     { return applyColor(s, color.BgGreen) }
func BgYellow(s string) string    { return applyColor(s, color.BgYellow) }
func BgBlue(s string) string      { return applyColor(s, color.BgBlue) }
func BgMagenta(s string) string   { return applyColor(s, color.BgMagenta) }
func BgCyan(s string) string      { return applyColor(s, color.BgCyan) }
func BgWhite(s string) string     { return applyColor(s, color.BgWhite) }
func BgHiBlack(s string) string   { return applyColor(s, color.BgHiBlack) }
func BgHiRed(s string) string     { return applyColor(s, color.BgHiRed) }
func BgHiGreen(s string) string   { return applyColor(s, color.BgHiGreen) }
func BgHiYellow(s string) string  { return applyColor(s, color.BgHiYellow) }
func BgHiBlue(s string) string    { return applyColor(s, color.BgHiBlue) }
func BgHiMagenta(s string) string { return applyColor(s, color.BgHiMagenta) }
func BgHiCyan(s string) string    { return applyColor(s, color.BgHiCyan) }
func BgHiWhite(s string) string   { return applyColor(s, color.BgWhite) }

func Reset(s string) string        { return applyColor(s, color.Reset) }
func Bold(s string) string         { return applyColor(s, color.Bold) }
func Faint(s string) string        { return applyColor(s, color.Faint) }
func Italic(s string) string       { return applyColor(s, color.Italic) }
func Underline(s string) string    { return applyColor(s, color.Underline) }
func BlinkSlow(s string) string    { return applyColor(s, color.BlinkSlow) }
func BlinkRapid(s string) string   { return applyColor(s, color.BlinkRapid) }
func ReverseVideo(s string) string { return applyColor(s, color.ReverseVideo) }
func Concealed(s string) string    { return applyColor(s, color.Concealed) }
func CrossedOut(s string) string   { return applyColor(s, color.CrossedOut) }

// Fg256 applies a 256-color palette foreground color
// index should be 0-255
func Fg256(index int, s string) string {
	if index < 0 || index > 255 {
		return s
	}

	// Check color level and downgrade if needed
	switch currentColorLevel {
	case LevelNone:
		return s
	case Level16:
		attr := color256To16(uint8(index))
		return applyColor(s, attr)
	case Level256:
		// Use extended color code: ESC[38;5;<index>m
		c := color.New(color.Attribute(38), color.Attribute(5), color.Attribute(index))
		return c.Sprint(s)
	default:
		return s
	}
}

// Bg256 applies a 256-color palette background color
// index should be 0-255
func Bg256(index int, s string) string {
	if index < 0 || index > 255 {
		return s
	}

	switch currentColorLevel {
	case LevelNone:
		return s
	case Level16:
		// Convert to 16-color background (add 10 to foreground code)
		attr := color256To16(uint8(index))
		bgAttr := attr + 10 // Convert Fg to Bg
		return applyColor(s, bgAttr)
	case Level256:
		// Use extended color code: ESC[48;5;<index>m
		c := color.New(color.Attribute(48), color.Attribute(5), color.Attribute(index))
		return c.Sprint(s)
	default:
		return s
	}
}
