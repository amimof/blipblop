package cmdutil

import (
	"github.com/fatih/color"
)

type ColorLevel int

const (
	LevelNone ColorLevel = iota
	Level16
	Level256
)

// Foreground text colors
const (
	Color16FgBlack color.Attribute = iota + 30
	Color16FgRed
	Color16FgGreen
	Color16FgYellow
	Color16FgBlue
	Color16FgMagenta
	Color16FgCyan
	Color16FgWhite
)

// Foreground Hi-Intensity text colors
const (
	Color16FgHiBlack color.Attribute = iota + 90
	Color16FgHiRed
	Color16FgHiGreen
	Color16FgHiYellow
	Color16FgHiBlue
	Color16FgHiMagenta
	Color16FgHiCyan
	Color16FgHiWhite
)

// Background text colors
const (
	Color16BgBlack color.Attribute = iota + 40
	Color16BgRed
	Color16BgGreen
	Color16BgYellow
	Color16BgBlue
	Color16BgMagenta
	Color16BgCyan
	Color16BgWhite
)

// Background Hi-Intensity text colors
const (
	Color16BgHiBlack color.Attribute = iota + 100
	Color16BgHiRed
	Color16BgHiGreen
	Color16BgHiYellow
	Color16BgHiBlue
	Color16BgHiMagenta
	Color16BgHiCyan
	Color16BgHiWhite
)

// Attributes
const (
	AttrReset color.Attribute = iota
	AttrBold
	AttrFaint
	AttrItalic
	AttrUnderline
	AttrBlinkSlow
	AttrBlinkRapid
	AttrReverseVideo
	AttrConcealed
	AttrCrossedOut
	AttrResetBold color.Attribute = iota + 22
	AttrResetItalic
	AttrResetUnderline
	AttrResetBlinking
	AttrResetReversed
	AttrResetConcealed
	AttrResetCrossedOut
)

var (
	currentColorLevel = Level256
	resetCode         = "\x1b[0m"
)

type (
	Color16  uint8 // ANSI 16-color (0-15)
	Color256 uint8 // ANSI 256-color palette (0-255)
)

// SetColorLevel sets the global color level for auto-downgrade support
func SetColorLevel(level ColorLevel) {
	currentColorLevel = level
}

// StyleBg256 helper for 256-color backgrounds
func StyleBg256(index uint8) []color.Attribute {
	return []color.Attribute{48, 5, color.Attribute(index)}
}

// StyleFg256 helper for 256-color foregrounds
func StyleFg256(index uint8) []color.Attribute {
	return []color.Attribute{38, 5, color.Attribute(index)}
}

// applyColor applies color/attributes to text.
// It handles chaining by detecting existing ANSI codes and merging
func applyColor(text string, attrs ...color.Attribute) string {
	if len(attrs) == 0 {
		return text
	}
	c := color.New(attrs...)
	return c.Sprint(text)
}

// color256To16 converts 256-color index to 16-color
func color256To16(index uint8) color.Attribute {
	if index < 16 {
		if index < 8 {
			return color.Attribute(30 + index) // Normal colors
		}
		return color.Attribute(90 + index - 8) // Bright colors
	}
	return color.Attribute(index)
}
