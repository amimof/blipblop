package cmdutil

import (
	"github.com/fatih/color"
)

type fg string

func (f fg) FgBlack() string     { return color.New(color.FgBlack).Sprint(f) }
func (f fg) FgRed() string       { return color.New(color.FgRed).Sprint(f) }
func (f fg) FgGreen() string     { return color.New(color.FgGreen).Sprint(f) }
func (f fg) FgYellow() string    { return color.New(color.FgYellow).Sprint(f) }
func (f fg) FgBlue() string      { return color.New(color.FgBlue).Sprint(f) }
func (f fg) FgMagenta() string   { return color.New(color.FgMagenta).Sprint(f) }
func (f fg) FgCyan() string      { return color.New(color.FgCyan).Sprint(f) }
func (f fg) FgWhite() string     { return color.New(color.FgWhite).Sprint(f) }
func (f fg) FgHiBlack() string   { return color.New(color.FgHiBlack).Sprint(f) }
func (f fg) FgHiRed() string     { return color.New(color.FgHiRed).Sprint(f) }
func (f fg) FgHiGreen() string   { return color.New(color.FgHiGreen).Sprint(f) }
func (f fg) FgHiYellow() string  { return color.New(color.FgHiYellow).Sprint(f) }
func (f fg) FgHiBlue() string    { return color.New(color.FgHiBlue).Sprint(f) }
func (f fg) FgHiMagenta() string { return color.New(color.FgHiMagenta).Sprint(f) }
func (f fg) FgHiCyan() string    { return color.New(color.FgHiCyan).Sprint(f) }
func (f fg) FgHiWhite() string   { return color.New(color.FgHiWhite).Sprint(f) }

type bg string

func (b bg) BgBlack() string     { return color.New(color.BgBlack).Sprint(b) }
func (b bg) BgRed() string       { return color.New(color.BgRed).Sprint(b) }
func (b bg) BgGreen() string     { return color.New(color.BgGreen).Sprint(b) }
func (b bg) BgYellow() string    { return color.New(color.BgYellow).Sprint(b) }
func (b bg) BgBlue() string      { return color.New(color.BgBlue).Sprint(b) }
func (b bg) BgMagenta() string   { return color.New(color.BgMagenta).Sprint(b) }
func (b bg) BgCyan() string      { return color.New(color.BgCyan).Sprint(b) }
func (b bg) BgWhite() string     { return color.New(color.BgWhite).Sprint(b) }
func (b bg) BgHiBlack() string   { return color.New(color.BgHiBlack).Sprint(b) }
func (b bg) BgHiRed() string     { return color.New(color.BgHiRed).Sprint(b) }
func (b bg) BgHiGreen() string   { return color.New(color.BgHiGreen).Sprint(b) }
func (b bg) BgHiYellow() string  { return color.New(color.BgHiYellow).Sprint(b) }
func (b bg) BgHiBlue() string    { return color.New(color.BgHiBlue).Sprint(b) }
func (b bg) BgHiMagenta() string { return color.New(color.BgHiMagenta).Sprint(b) }
func (b bg) BgHiCyan() string    { return color.New(color.BgHiCyan).Sprint(b) }
func (b bg) BgHiWhite() string   { return color.New(color.BgHiWhite).Sprint(b) }

type attr string

func (a attr) Reset() string           { return color.New(color.Reset).Sprint(a) }
func (a attr) Bold() string            { return color.New(color.Bold).Sprint(a) }
func (a attr) Faint() string           { return color.New(color.Faint).Sprint(a) }
func (a attr) Italic() string          { return color.New(color.Italic).Sprint(a) }
func (a attr) Underline() string       { return color.New(color.Underline).Sprint(a) }
func (a attr) BlinkSlow() string       { return color.New(color.BlinkSlow).Sprint(a) }
func (a attr) BlinkRapid() string      { return color.New(color.BlinkRapid).Sprint(a) }
func (a attr) ReverseVideo() string    { return color.New(color.ReverseVideo).Sprint(a) }
func (a attr) Concealed() string       { return color.New(color.Concealed).Sprint(a) }
func (a attr) CrossedOut() string      { return color.New(color.CrossedOut).Sprint(a) }
func (a attr) ResetBold() string       { return color.New(color.ResetBold).Sprint(a) }
func (a attr) ResetItalic() string     { return color.New(color.ResetItalic).Sprint(a) }
func (a attr) ResetUnderline() string  { return color.New(color.ResetUnderline).Sprint(a) }
func (a attr) ResetBlinking() string   { return color.New(color.ResetBlinking).Sprint(a) }
func (a attr) ResetReversed() string   { return color.New(color.ResetReversed).Sprint(a) }
func (a attr) ResetConcealed() string  { return color.New(color.ResetConcealed).Sprint(a) }
func (a attr) ResetCrossedOut() string { return color.New(color.ResetCrossedOut).Sprint(a) }
