package cmdutil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"
)

// Data holds information used when templating
type Data map[string]any

// Frames for the spinner
var frames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// Component describes how objects are rendered frame by frame
type Component interface {
	// Loop runs the rendering loop until context is cancelled
	Loop(context.Context)

	// Wait blocks until the renderer is done
	Wait()

	// Close cleans up resources
	Close() error
}

// Renderer describes how objects are renderd on to the screen
type Renderer interface {
	Render(any) []byte
}

// App is the top most item and renders child containers.
type App struct {
	data       Data
	containers []*Container
	done       chan struct{}
	lastLines  int
	frameIdx   int // current spinner frame; only written inside renderFrame under mu
	mu         sync.Mutex
	Writer     io.Writer
	level      ColorLevel
}

// Container renders elements on the screen. Contaner holds layout information such
// as dimensions and padding.
type Container struct {
	Layout
	Style
	ContentWidth int
	data         Data
	mu           sync.Mutex
	elements     []*Element
}

type Element struct {
	Template string             // Raw template string for this column
	Width    int                // Max width (0 = unlimited)
	Parsed   *template.Template // Compiled template (set during initialization)
}

type Layout struct {
	Dimensions [2]int // Width, Height
	Padding    [4]int // Top, right, bottom, left
}

type Style struct {
	Bg   []color.Attribute
	Fg   []color.Attribute
	Attr color.Attribute
}

func spinnerFrame(idx int) string {
	return fmt.Sprintf("%c", frames[idx%len(frames)])
}

// Render implements [Renderer].
// frameIdx is the current animation frame owned by the App; it is passed in
// rather than read from a shared variable to avoid data races.
func (e *Element) Render(d any, frameIdx int) []byte {
	b := &bytes.Buffer{}

	// Build a per-call FuncMap that overrides only the spinner function so we
	// never mutate the shared templateFuncs map from multiple goroutines.
	localFuncs := template.FuncMap{
		"spinner": func() string {
			return spinnerFrame(frameIdx)
		},
	}

	err := e.Parsed.Funcs(localFuncs).Execute(b, d)
	if err != nil {
		return nil
	}

	return b.Bytes()
}

// Count returns the total amount of lines all child elements render.
// Always returns 1 since elements never span across multiple lines.
func (e *Element) Count() int {
	return 1
}

// applyBgColor wraps a string with background color ANSI codes
func (c *Container) applyBgColor(s string, max int) string {
	// bgAttr := c.Bg

	// Check if background color is set
	if len(c.Bg) == 0 {
		return padToWidth(s, max)
	}

	if max < c.Dimensions[0] {
		max = c.Dimensions[0]
	}

	// Get the background ANSI start code
	bgCode := extractANSIStart(color.New(c.Bg...).Sprint(""))
	// if bgCode == "" {
	// 	return padToWidth(s, max)
	// }

	// Replace both simple and 256-color reset codes
	result := bgCode + s
	result = strings.ReplaceAll(result, "\x1b[0m", "\x1b[0m"+bgCode)
	result = strings.ReplaceAll(result, "\x1b[0;25;0m", "\x1b[0;25;0m"+bgCode)

	// Pad to width with background-colored spaces
	visibleLen := utf8.RuneCount([]byte(stripANSI(result)))
	if visibleLen < max {
		padding := strings.Repeat(" ", max-visibleLen)
		result = result + padding
	}

	// Final reset to clean up
	result = result + resetCode

	return result
}

func (c *Container) RenderLines(data Data, frameIdx int) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Top padding
	topPad := c.Padding[0]
	botPad := c.Padding[2]
	lines := make([]string, 0, topPad+len(c.elements)+botPad)

	// Render all elements in a single pass, building the merged data map once
	// per element. Track max visible width as we go — no second render pass needed.
	type renderedLine struct {
		text     string
		visWidth int
	}
	renderedLines := make([]renderedLine, len(c.elements))
	c.ContentWidth = 0

	for i, e := range c.elements {
		d := make(map[string]any, len(data)+1)
		maps.Copy(d, data)
		d["Container"] = c.data

		b := e.Render(d, frameIdx)
		text := string(b)
		vis := utf8.RuneCount([]byte(stripANSI(text)))
		renderedLines[i] = renderedLine{text, vis}
		if vis > c.ContentWidth {
			c.ContentWidth = vis
		}
	}

	// Top padding
	for range topPad {
		lines = append(lines, c.applyBgColor("", c.Dimensions[0]))
	}

	// Content — apply padding and truncation to the already-rendered text.
	// Elements that render to empty/whitespace are skipped entirely so they
	// don't leave blank rows (e.g. detail fields suppressed on failure/done).
	for _, r := range renderedLines {
		if strings.TrimSpace(stripANSI(r.text)) == "" {
			continue
		}
		padded := fmt.Sprintf("  %s  ", r.text)
		if len(padded) >= c.Dimensions[0] {
			padded = truncateWithEllipsis(padded, c.Dimensions[0])
		}
		lines = append(lines, c.applyBgColor(padded, c.Dimensions[0]))
	}

	// Bottom padding
	for range botPad {
		lines = append(lines, c.applyBgColor("", c.Dimensions[0]))
	}

	return lines
}

// SetMetadata updates metadata for template access
func (c *Container) SetMetadata(data Data) *Container {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil {
		c.data = make(map[string]any)
	}
	c.data = data
	return c
}

// UpdateMetadata sets metadata key for template access
func (c *Container) UpdateMetadata(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.data == nil {
		c.data = make(map[string]any)
	}
	c.data[key] = value
}

// Render implements [Renderer]
func (c *Container) Render(data Data, frameIdx int) []byte {
	buf := bytes.Buffer{}
	by := bytes.NewBuffer(buf.Bytes())

	for _, e := range c.elements {
		b := e.Render(data, frameIdx)
		_, err := by.Write(b)
		if err != nil {
			panic(err)
		}
	}

	return by.Bytes()
}

// Count returns the total amount of lines all child elements render.
func (c *Container) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.elements)
}

// Render implements [Renderer]
func (a *App) Render() []byte {
	buf := bytes.Buffer{}
	by := bytes.NewBuffer(buf.Bytes())
	for _, c := range a.containers {
		b := c.Render(a.data, a.frameIdx)
		_, err := by.Write(b)
		if err != nil {
			panic(err)
		}
		by.WriteByte('\n')
	}

	return by.Bytes()
}

func (a *App) renderFrame() {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Move cursor up to the start of the last frame's output
	if a.lastLines > 0 {
		_, _ = fmt.Fprintf(a.Writer, "\033[%dA", a.lastLines)
	}

	// Advance spinner by one tick (safe: only written here, under a.mu)
	a.frameIdx = (a.frameIdx + 1) % len(frames)

	linesThisFrame := 0
	for _, container := range a.containers {
		lines := container.RenderLines(a.data, a.frameIdx)
		for _, line := range lines {
			_, _ = fmt.Fprint(a.Writer, "\033[2K\r") // clear line + return to col 0
			_, _ = fmt.Fprint(a.Writer, line)
			_, _ = fmt.Fprint(a.Writer, "\n")
			linesThisFrame++
		}
	}

	// Erase everything below the last written line. This cleans up any surplus
	// lines from a previous frame that had more lines than this one (e.g. after
	// detail fields collapse on done/failure).
	_, _ = fmt.Fprint(a.Writer, "\033[J")

	a.lastLines = linesThisFrame
}

// Loop renders the app each frame
func (a *App) Loop(ctx context.Context) {
	defer close(a.done)

	// Start with no pre-allocated space. The first renderFrame call will write
	// lines downward from the current cursor position. \033[J in renderFrame
	// cleans up any surplus from previous frames.
	a.lastLines = 0

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Render one last frame before quitting
			a.renderFrame()
			return
		case <-ticker.C:
			a.renderFrame()
		}
	}
}

// Count returns the total amount of lines all child containers render.
func (a *App) Count() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	total := 0
	for _, container := range a.containers {
		total += container.Count()
	}
	return total
}

// Wait blocks until Loop finishes.
func (a *App) Wait() {
	<-a.done
}

// WaitAnd blocks until Loop finishes and then calls fn.
func (a *App) WaitAnd(fn func()) {
	a.Wait()
	fn()
}

func (a *App) WithLevel(l ColorLevel) *App {
	a.level = l
	return a
}

// SetMetadata updates metadata for template access
func (a *App) SetMetadata(md map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.data == nil {
		a.data = make(map[string]any)
	}
	a.data = md
}

// UpdateMetadata sets metadata key for template access
func (a *App) UpdateMetadata(key string, value any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.data == nil {
		a.data = make(map[string]any)
	}
	a.data[key] = value
}

func (a *App) AddContainer(c *Container) *App {
	a.containers = append(a.containers, c)
	return a
}

// NewElement creates a new element with the provided format string.
func NewElement(format string) *Element {
	tmpl, err := template.New("header").Funcs(templateFuncs).Parse(format)
	if err != nil {
		fmt.Printf("error parsing header template: %v\n", err)
		return nil
	}
	return &Element{
		Template: format,
		Parsed:   tmpl,
	}
}

// NewContainer creates a new 1x1 container with the given elements and children.
// func NewContainer(style Style, opts Layout, r ...*Element) *Container {
func NewContainer(data Data, r ...*Element) *Container {
	return &Container{
		elements: r,
		data:     data,
	}
}

func (c *Container) WithStyle(s Style) *Container {
	c.Style = s
	return c
}

func (c *Container) WithLayout(l Layout) *Container {
	c.Layout = l
	return c
}

func (c *Container) Copies(n int) []*Container {
	containers := make([]*Container, n)
	for i := range containers {
		containers[i] = c
	}
	return containers
}

// stripANSI removes ANSI escape sequences from a string.
// This is used to calculate the visible width of strings that contain color codes.
func stripANSI(s string) string {
	// Regex pattern to match ANSI escape sequences
	// Matches: ESC [ <optional params> <command letter>
	// Example: \x1b[38;5;001m or \x1b[0m
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return re.ReplaceAllString(s, "")
}

// truncateWithEllipsis truncates a string to maxWidth, accounting for ANSI codes.
// If truncated, appends an ellipsis within the width limit.
// Strings that already fit are returned unchanged.
// Example: truncateWithEllipsis("very-long-name", 10) => "very-lo…  "
func truncateWithEllipsis(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s // No limit
	}

	if visibleLength(s) <= maxWidth {
		return s // Fits — no padding here; callers handle alignment separately
	}

	// Need to truncate
	if maxWidth <= 3 {
		// Too narrow for ellipsis, just cut
		return truncateToWidth(s, maxWidth)
	}

	// Truncate to (maxWidth - 3) and add ellipsis
	truncated := truncateToWidth(s, maxWidth-3)
	return truncated + "…  "
}

// truncateToWidth truncates a string to exactly width visible characters,
// preserving ANSI codes that appear before the cut point.
func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}

	visibleCount := 0
	inEscape := false
	var result strings.Builder

	for _, r := range s {
		// Detect ANSI escape sequence start
		if r == '\x1b' {
			inEscape = true
		}

		// Always include escape sequence characters
		if inEscape {
			result.WriteRune(r)
			if r == 'm' {
				inEscape = false
			}
			continue
		}

		// Count visible characters
		if visibleCount >= width {
			break
		}

		result.WriteRune(r)
		visibleCount++
	}

	return result.String()
}

// visibleLength returns the visible character count (excluding ANSI codes)
func visibleLength(s string) int {
	return len(stripANSI(s))
}

// padRight pads a string to a fixed width (accounting for ANSI codes)
func padRight(s string, width int) string {
	visible := visibleLength(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// padLeft pads a string to a fixed width on the left (accounting for ANSI codes)
func padLeft(s string, width int) string {
	visible := visibleLength(s)
	if visible >= width {
		return s
	}
	return strings.Repeat(" ", width-visible) + s
}

// Custom template functions for convenience
var templateFuncs = template.FuncMap{
	"duration": func(d time.Duration) string {
		return FormatDuration(d)
	},
	"age": func(t time.Time) string {
		return FormatDuration(time.Since(t))
	},
	"icon": func(done, failed bool) string {
		if !done {
			return "⠿" // Placeholder when not done
		}
		if failed {
			return "✖"
		}
		return "✔"
	},
	"spinner": func() string {
		return "⠋"
	},
	"OCImage": func(s string) string {
		return "" // TODO: Parse OCI registry URL and colorize
	},
	"padLeft":  padLeft,
	"padRight": padRight,

	// Foreground colors (16-color ANSI)
	"FgBlack":     FgBlack,
	"FgRed":       FgRed,
	"FgGreen":     FgGreen,
	"FgYellow":    FgYellow,
	"FgBlue":      FgBlue,
	"FgMagenta":   FgMagenta,
	"FgCyan":      FgCyan,
	"FgWhite":     FgWhite,
	"FgHiBlack":   FgHiBlack,
	"FgHiRed":     FgHiRed,
	"FgHiGreen":   FgHiGreen,
	"FgHiYellow":  FgHiYellow,
	"FgHiBlue":    FgHiBlue,
	"FgHiMagenta": FgHiMagenta,
	"FgHiCyan":    FgHiCyan,
	"FgHiWhite":   FgHiWhite,

	// Background colors (16-color ANSI)
	"BgBlack":     BgBlack,
	"BgRed":       BgRed,
	"BgGreen":     BgGreen,
	"BgYellow":    BgYellow,
	"BgBlue":      BgBlue,
	"BgMagenta":   BgMagenta,
	"BgCyan":      BgCyan,
	"BgWhite":     BgWhite,
	"BgHiBlack":   BgHiBlack,
	"BgHiRed":     BgHiRed,
	"BgHiGreen":   BgHiGreen,
	"BgHiYellow":  BgHiYellow,
	"BgHiBlue":    BgHiBlue,
	"BgHiMagenta": BgHiMagenta,
	"BgHiCyan":    BgHiCyan,
	"BgHiWhite":   BgHiWhite,

	// Text attributes
	"Reset":        Reset,
	"Bold":         Bold,
	"Faint":        Faint,
	"Italic":       Italic,
	"Underline":    Underline,
	"BlinkSlow":    BlinkSlow,
	"BlinkRapid":   BlinkRapid,
	"ReverseVideo": ReverseVideo,
	"Concealed":    Concealed,
	"CrossedOut":   CrossedOut,

	// 256-color palette
	"Fg256": Fg256,
	"Bg256": Bg256,
}

// extractANSIStart gets the opening ANSI code from a colored string
// e.g., color.New(color.BgCyan).Sprint("") -> "\x1b[46m\x1b[0m" -> "\x1b[46m"
func extractANSIStart(coloredEmpty string) string {
	// Match any reset code (starts with ESC[0)
	re := regexp.MustCompile(`\x1b\[0[0-9;]*m$`)
	return re.ReplaceAllString(coloredEmpty, "")
}

// padToWidth pads a string to a specific width, accounting for ANSI codes
func padToWidth(s string, width int) string {
	visibleLen := utf8.RuneCount([]byte(stripANSI(s)))
	if visibleLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visibleLen)
}

// NewApp creates a new App using the given writer and containers and children.
func NewApp(wr io.Writer, data Data, containers ...*Container) *App {
	return &App{
		Writer:     wr,
		data:       data,
		containers: containers,
		done:       make(chan struct{}),
		level:      Level256,
	}
}
