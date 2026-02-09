package cmdutil

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"maps"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/fatih/color"
)

// Data holds information used when templating
type Data map[string]any

// Frames for the spinner
var (
	frames   = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
	frameIdx = 0
)

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

func (e *Element) spinner() string {
	return fmt.Sprintf("%c", frames[frameIdx%len(frames)])
}

// Render implements [Renderer]
func (e *Element) Render(d any) []byte {
	buf := bytes.Buffer{}
	b := bytes.NewBuffer(buf.Bytes())

	// Update spinner function for this frame
	templateFuncs["spinner"] = func() string {
		return e.spinner()
	}

	err := e.Parsed.Funcs(templateFuncs).Execute(b, d)
	if err != nil {
		return nil
	}

	// Advance frame tick
	// e.frameIdx = (e.frameIdx + 1) % len(frames)
	return b.Bytes()
}

// Count returns the total amount of lines all child elements render.
// Always returns 1 since elements never span across multiple lines.
func (e *Element) Count() int {
	return 1
}

func (c *Container) contentWidth(data Data) int {
	// Otherwise, calculate based on widest element
	maxWidth := 0
	for _, e := range c.elements {
		b := e.Render(data)
		stripped := stripANSI(string(b))
		width := utf8.RuneCount([]byte(stripped))
		if width > maxWidth {
			maxWidth = width
		}
	}

	return maxWidth
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

func (c *Container) RenderLines(data Data) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	lines := make([]string, 0, len(c.elements))
	c.ContentWidth = c.contentWidth(data)

	// Top padding
	for i := 0; i < c.Padding[0]; i++ {
		line := c.applyBgColor(string([]byte{}), c.Dimensions[0])
		lines = append(lines, line)
	}

	// Content
	for _, e := range c.elements {

		// Make a copy of app data
		d := map[string]any{}
		maps.Copy(d, data)

		d["Container"] = c.data
		b := e.Render(d)
		padded := fmt.Sprintf("  %s  ", string(b))
		if len(padded) >= c.Dimensions[0] {
			padded = truncateWithEllipsis(padded, c.Dimensions[0])
		}

		line := c.applyBgColor(string(padded), c.Dimensions[0])
		lines = append(lines, line)

		// Advance frame tick
		// e.frameIdx = c.frameIdx

	}

	// Bottom padding
	for i := 0; i < c.Padding[2]; i++ {
		line := c.applyBgColor(string([]byte{}), c.Dimensions[0])
		lines = append(lines, line)
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
func (c *Container) Render(data Data) []byte {
	buf := bytes.Buffer{}
	by := bytes.NewBuffer(buf.Bytes())

	for _, e := range c.elements {
		b := e.Render(data)
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
		b := c.Render(a.data)
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
	// Move cursor up to start position (if not first frame)
	if a.lastLines > 0 {
		_, _ = fmt.Fprintf(a.Writer, "\033[%dA", a.lastLines)
	}

	// Advance spinner by one tick
	frameIdx = (frameIdx + 1) % len(frames)

	linesThisFrame := 0
	// Render each container
	for _, container := range a.containers {
		// Get rendered lines from container
		lines := container.RenderLines(a.data)

		// Write each line with proper clearing
		for _, line := range lines {
			// Clear current line + return to column 0
			_, _ = fmt.Fprint(a.Writer, "\033[2K\r")

			// Write line content
			_, _ = fmt.Fprint(a.Writer, line)

			// Move to next line
			_, _ = fmt.Fprint(a.Writer, "\n")

			linesThisFrame++
		}
	}
	// Store line count for next frame
	a.lastLines = linesThisFrame
}

// Loop renders the app each frame
func (a *App) Loop(ctx context.Context) {
	// Calculate total lines needed
	totalLines := a.Count()

	// Pre-allocate space for rendering
	for range totalLines {
		_, _ = fmt.Fprintln(a.Writer)
	}

	// Set initial line count
	a.lastLines = totalLines

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Move cursor past rendered content on exit
			_, _ = fmt.Fprintln(a.Writer)
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

// WaitAnd blocks until Loop finishes and executes the provided function when done
func (a *App) WaitAnd(fn func()) {
	go func() {
		for {
			time.Sleep(200 * time.Millisecond)
			fn()
			return
		}
	}()
	a.Wait()
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
// If truncated, adds "..." at the end (within the width limit).
// Example: truncateWithEllipsis("very-long-name", 10) => "very-lo..."
func truncateWithEllipsis(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s // No limit
	}

	visible := visibleLength(s)
	if visible <= maxWidth {
		return fmt.Sprintf("%s%s", s, strings.Repeat(" ", maxWidth-visible))
		// return s // Fits within limit
	}

	// Need to truncate
	if maxWidth <= 3 {
		// Too narrow for ellipsis, just cut
		return truncateToWidth(s, maxWidth)
	}

	// Truncate to (maxWidth - 3) and add "..."
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
