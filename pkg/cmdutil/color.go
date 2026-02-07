package cmdutil

import (
	"fmt"

	"github.com/fatih/color"
)

type Attribute uint32

// Foreground text colors
const (
	ColorFgBlack Attribute = iota + 30
	ColorFgRed
	ColorFgGreen
	ColorFgYellow
	ColorFgBlue
	ColorFgMagenta
	ColorFgCyan
	ColorFgWhite
)

// Foreground Hi-Intensity text colors
const (
	ColorFgHiBlack Attribute = iota + 90
	ColorFgHiRed
	ColorFgHiGreen
	ColorFgHiYellow
	ColorFgHiBlue
	ColorFgHiMagenta
	ColorFgHiCyan
	ColorFgHiWhite
)

// Background text colors
const (
	ColorBgBlack Attribute = iota + 40
	ColorBgRed
	ColorBgGreen
	ColorBgYellow
	ColorBgBlue
	ColorBgMagenta
	ColorBgCyan
	ColorBgWhite
)

// Background Hi-Intensity text colors
const (
	ColorBgHiBlack Attribute = iota + 100
	ColorBgHiRed
	ColorBgHiGreen
	ColorBgHiYellow
	ColorBgHiBlue
	ColorBgHiMagenta
	ColorBgHiCyan
	ColorBgHiWhite
)

// Attributes
const (
	AttrReset Attribute = iota
	AttrBold
	AttrFaint
	AttrItalic
	AttrUnderline
	AttrBlinkSlow
	AttrBlinkRapid
	AttrReverseVideo
	AttrConcealed
	AttrCrossedOut
	AttrResetBold Attribute = iota + 22
	AttrResetItalic
	AttrResetUnderline
	AttrResetBlinking
	AttrResetReversed
	AttrResetConcealed
	AttrResetCrossedOut
)

type Colorer interface {
	Color()
}

type (
	ColorANSI  int
	ColorBasic uint8
	ColorRGB   struct{ R, G, B uint8 }
)

type FG string

// Basic
func Basic(c ColorBasic) Colorer {
	return ColorBasic(c)
}

func (c ColorBasic) Color() {}

func (c ColorBasic) Sprint(text string) string {
	s := fmt.Sprint(color.New(color.Attribute(c)).Sprint(text))
	return s
}

func (c ColorBasic) Sprintf(format string, args ...any) string {
	return fmt.Sprint(color.New(color.Attribute(c)).Sprintf(format, args...))
}

// ANSI
func ANSI(c ColorANSI) Colorer {
	return c
}

func (ColorANSI) Color() {}

func (c ColorANSI) Sprint(text string) string {
	return fmt.Sprint(color.New(color.Attribute(c)).Sprint(text))
}

func (c ColorANSI) Sprintf(format string, args ...any) string {
	return fmt.Sprint(color.New(color.Attribute(c)).Sprintf(format, args...))
}

// RGB
func RGB(r, g, b uint8) Colorer {
	return ColorRGB{r, g, b}
}
func (ColorRGB) Color() {}

func (c ColorRGB) Sprint(text string) string {
	return fmt.Sprint(color.RGB(int(c.R), int(c.G), int(c.B)).Sprint(text))
}

func (c ColorRGB) Sprintf(format string, args ...any) string {
	return fmt.Sprint(color.RGB(int(c.R), int(c.G), int(c.B)).Sprintf(format, args...))
}

func FgBlack(s string) Colorer     { return ColorBasic(ColorFgBlack) }
func FgRed(s string) Colorer       { return ColorBasic(ColorFgRed) }
func FgGreen(s string) Colorer     { return ColorBasic(ColorFgGreen) }
func FgYellow(s string) Colorer    { return ColorBasic(ColorFgYellow) }
func FgBlue(s string) Colorer      { return ColorBasic(ColorFgBlue) }
func FgMagenta(s string) Colorer   { return ColorBasic(ColorFgMagenta) }
func FgCyan(s string) Colorer      { return ColorBasic(ColorFgCyan) }
func FgWhite(s string) Colorer     { return ColorBasic(ColorFgWhite) }
func FgHiBlack(s string) Colorer   { return ColorBasic(ColorFgHiBlack) }
func FgHiRed(s string) Colorer     { return ColorBasic(ColorFgHiRed) }
func FgHiGreen(s string) Colorer   { return ColorBasic(ColorFgHiGreen) }
func FgHiYellow(s string) Colorer  { return ColorBasic(ColorFgHiYellow) }
func FgHiBlue(s string) Colorer    { return ColorBasic(ColorFgHiBlue) }
func FgHiMagenta(s string) Colorer { return ColorBasic(ColorFgHiMagenta) }
func FgHiCyan(s string) Colorer    { return ColorBasic(ColorFgHiCyan) }
func FgHiWhite(s string) Colorer   { return ColorBasic(ColorFgHiWhite) }

func BgBlack(s string) Colorer     { return ColorBasic(ColorBgBlack) }
func BgRed(s string) Colorer       { return ColorBasic(ColorBgRed) }
func BgGreen(s string) Colorer     { return ColorBasic(ColorBgGreen) }
func BgYellow(s string) Colorer    { return ColorBasic(ColorBgYellow) }
func BgBlue(s string) Colorer      { return ColorBasic(ColorBgBlue) }
func BgMagenta(s string) Colorer   { return ColorBasic(ColorBgMagenta) }
func BgCyan(s string) Colorer      { return ColorBasic(ColorBgCyan) }
func BgWhite(s string) Colorer     { return ColorBasic(ColorBgWhite) }
func BgHiBlack(s string) Colorer   { return ColorBasic(ColorBgHiBlack) }
func BgHiRed(s string) Colorer     { return ColorBasic(ColorBgHiRed) }
func BgHiGreen(s string) Colorer   { return ColorBasic(ColorBgHiGreen) }
func BgHiYellow(s string) Colorer  { return ColorBasic(ColorBgHiYellow) }
func BgHiBlue(s string) Colorer    { return ColorBasic(ColorBgHiBlue) }
func BgHiMagenta(s string) Colorer { return ColorBasic(ColorBgHiMagenta) }
func BgHiCyan(s string) Colorer    { return ColorBasic(ColorBgHiCyan) }
func BgHiWhite(s string) Colorer   { return ColorBasic(ColorBgHiWhite) }

func Reset(s string) Colorer           { return ColorBasic(AttrReset) }
func Bold(s string) Colorer            { return ColorBasic(AttrBold) }
func Faint(s string) Colorer           { return ColorBasic(AttrFaint) }
func Italic(s string) Colorer          { return ColorBasic(AttrItalic) }
func Underline(s string) Colorer       { return ColorBasic(AttrUnderline) }
func BlinkSlow(s string) Colorer       { return ColorBasic(AttrBlinkSlow) }
func BlinkRapid(s string) Colorer      { return ColorBasic(AttrBlinkRapid) }
func ReverseVideo(s string) Colorer    { return ColorBasic(AttrReverseVideo) }
func Concealed(s string) Colorer       { return ColorBasic(AttrConcealed) }
func CrossedOut(s string) Colorer      { return ColorBasic(AttrCrossedOut) }
func ResetBold(s string) Colorer       { return ColorBasic(AttrResetBold) }
func ResetItalic(s string) Colorer     { return ColorBasic(AttrResetItalic) }
func ResetUnderline(s string) Colorer  { return ColorBasic(AttrResetUnderline) }
func ResetBlinking(s string) Colorer   { return ColorBasic(AttrResetBlinking) }
func ResetReversed(s string) Colorer   { return ColorBasic(AttrResetReversed) }
func ResetConcealed(s string) Colorer  { return ColorBasic(AttrResetConcealed) }
func ResetCrossedOut(s string) Colorer { return ColorBasic(AttrResetCrossedOut) }
