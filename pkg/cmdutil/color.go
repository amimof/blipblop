package cmdutil

import (
	"github.com/fatih/color"
)

type FG string

func (f FG) FgBlack() string     { return color.New(color.FgBlack).Sprint(f) }
func (f FG) FgRed() string       { return color.New(color.FgRed).Sprint(f) }
func (f FG) FgGreen() string     { return color.New(color.FgGreen).Sprint(f) }
func (f FG) FgYellow() string    { return color.New(color.FgYellow).Sprint(f) }
func (f FG) FgBlue() string      { return color.New(color.FgBlue).Sprint(f) }
func (f FG) FgMagenta() string   { return color.New(color.FgMagenta).Sprint(f) }
func (f FG) FgCyan() string      { return color.New(color.FgCyan).Sprint(f) }
func (f FG) FgWhite() string     { return color.New(color.FgWhite).Sprint(f) }
func (f FG) FgHiBlack() string   { return color.New(color.FgHiBlack).Sprint(f) }
func (f FG) FgHiRed() string     { return color.New(color.FgHiRed).Sprint(f) }
func (f FG) FgHiGreen() string   { return color.New(color.FgHiGreen).Sprint(f) }
func (f FG) FgHiYellow() string  { return color.New(color.FgHiYellow).Sprint(f) }
func (f FG) FgHiBlue() string    { return color.New(color.FgHiBlue).Sprint(f) }
func (f FG) FgHiMagenta() string { return color.New(color.FgHiMagenta).Sprint(f) }
func (f FG) FgHiCyan() string    { return color.New(color.FgHiCyan).Sprint(f) }
func (f FG) FgHiWhite() string   { return color.New(color.FgHiWhite).Sprint(f) }

type BG string

func (b BG) BgBlack() string     { return color.New(color.BgBlack).Sprint(b) }
func (b BG) BgRed() string       { return color.New(color.BgRed).Sprint(b) }
func (b BG) BgGreen() string     { return color.New(color.BgGreen).Sprint(b) }
func (b BG) BgYellow() string    { return color.New(color.BgYellow).Sprint(b) }
func (b BG) BgBlue() string      { return color.New(color.BgBlue).Sprint(b) }
func (b BG) BgMagenta() string   { return color.New(color.BgMagenta).Sprint(b) }
func (b BG) BgCyan() string      { return color.New(color.BgCyan).Sprint(b) }
func (b BG) BgWhite() string     { return color.New(color.BgWhite).Sprint(b) }
func (b BG) BgHiBlack() string   { return color.New(color.BgHiBlack).Sprint(b) }
func (b BG) BgHiRed() string     { return color.New(color.BgHiRed).Sprint(b) }
func (b BG) BgHiGreen() string   { return color.New(color.BgHiGreen).Sprint(b) }
func (b BG) BgHiYellow() string  { return color.New(color.BgHiYellow).Sprint(b) }
func (b BG) BgHiBlue() string    { return color.New(color.BgHiBlue).Sprint(b) }
func (b BG) BgHiMagenta() string { return color.New(color.BgHiMagenta).Sprint(b) }
func (b BG) BgHiCyan() string    { return color.New(color.BgHiCyan).Sprint(b) }
func (b BG) BgHiWhite() string   { return color.New(color.BgHiWhite).Sprint(b) }

type Attr string

func (a Attr) Reset() string           { return color.New(color.Reset).Sprint(a) }
func (a Attr) Bold() string            { return color.New(color.Bold).Sprint(a) }
func (a Attr) Faint() string           { return color.New(color.Faint).Sprint(a) }
func (a Attr) Italic() string          { return color.New(color.Italic).Sprint(a) }
func (a Attr) Underline() string       { return color.New(color.Underline).Sprint(a) }
func (a Attr) BlinkSlow() string       { return color.New(color.BlinkSlow).Sprint(a) }
func (a Attr) BlinkRapid() string      { return color.New(color.BlinkRapid).Sprint(a) }
func (a Attr) ReverseVideo() string    { return color.New(color.ReverseVideo).Sprint(a) }
func (a Attr) Concealed() string       { return color.New(color.Concealed).Sprint(a) }
func (a Attr) CrossedOut() string      { return color.New(color.CrossedOut).Sprint(a) }
func (a Attr) ResetBold() string       { return color.New(color.ResetBold).Sprint(a) }
func (a Attr) ResetItalic() string     { return color.New(color.ResetItalic).Sprint(a) }
func (a Attr) ResetUnderline() string  { return color.New(color.ResetUnderline).Sprint(a) }
func (a Attr) ResetBlinking() string   { return color.New(color.ResetBlinking).Sprint(a) }
func (a Attr) ResetReversed() string   { return color.New(color.ResetReversed).Sprint(a) }
func (a Attr) ResetConcealed() string  { return color.New(color.ResetConcealed).Sprint(a) }
func (a Attr) ResetCrossedOut() string { return color.New(color.ResetCrossedOut).Sprint(a) }
