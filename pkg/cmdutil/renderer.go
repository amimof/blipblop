package cmdutil

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"maps"
	"strings"
	"sync"
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
	mu         sync.Mutex
	writer     io.Writer
}

// Container renders elements on the screen. Contaner holds layout information such
// as dimensions and padding.
type Container struct {
	Layout
	Style
	data Data

	mu       sync.Mutex
	elements []*Element
}

type Element struct {
	Template string             // Raw template string for this column
	Width    int                // Max width (0 = unlimited)
	Parsed   *template.Template // Compiled template (set during initialization)

	frameIdx int
}

type Layout struct {
	Dimensions [2]int // Width, Height
	Padding    [4]int // Top, right, bottom, left
}

type Style struct {
	Bg   Attribute
	Fg   Attribute
	Attr Attribute
}

func (e *Element) spinner() string {
	return fmt.Sprintf("%c", frames[e.frameIdx%len(frames)])
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
	// TODO: What to do with frame here?
	e.frameIdx = (e.frameIdx + 1) % len(frames)
	return b.Bytes()
}

// Count returns the total amount of lines all child elements render.
// Always returns 1 since elements never span across multiple lines.
func (e *Element) Count() int {
	return 1
}

func (c *Container) contentWidth(data Data) int {
	// if c.Dimensions[0] > 1 {
	// 	return c.Dimensions[0]
	// }

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

var resetCode = "\x1b[0m"

// extractANSIStart gets the opening ANSI code from a colored string
// e.g., color.New(color.BgCyan).Sprint("") -> "\x1b[46m\x1b[0m" -> "\x1b[46m"
func extractANSIStart(coloredEmpty string) string {
	// Remove the trailing reset code
	return strings.TrimSuffix(coloredEmpty, resetCode)
}

// padToWidth pads a string to a specific width, accounting for ANSI codes
func padToWidth(s string, width int) string {
	visibleLen := utf8.RuneCount([]byte(stripANSI(s)))
	if visibleLen >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visibleLen)
}

// applyBgColor wraps a string with background color ANSI codes
func (c *Container) applyBgColor(s string, max int) string {
	bgAttr := c.Bg

	// No background color set
	if bgAttr == 0 {
		return padToWidth(s, max)
	}

	if max < c.Dimensions[0] {
		max = c.Dimensions[0]
	}

	// Get the background ANSI start code
	bgCode := extractANSIStart(color.New(color.Attribute(bgAttr)).Sprint(""))
	if bgCode == "" {
		return padToWidth(s, max)
	}

	// Strategy: Replace all reset codes with reset+background
	// This maintains background even after foreground color resets
	result := bgCode + strings.ReplaceAll(s, resetCode, resetCode+bgCode)

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
	width := c.contentWidth(data)

	// Top padding
	for i := 0; i < c.Padding[0]; i++ {
		line := c.applyBgColor(string([]byte{}), width)
		lines = append(lines, line)
	}

	// Content
	for _, e := range c.elements {

		// Make a copy of app data
		d := map[string]any{}
		maps.Copy(d, data)

		d["Container"] = c.data
		b := e.Render(d)
		// padded := fmt.Sprintf("  %s  ", string(b))
		// if len(padded) >= width {
		// 	padded = truncateWithEllipsis(padded, c.Dimensions[0])
		// }

		line := c.applyBgColor(string(b), width)
		lines = append(lines, line)

		// Advance frame tick
		e.frameIdx = (e.frameIdx + 1) % len(frames)

	}

	// Bottom padding
	for i := 0; i < c.Padding[2]; i++ {
		line := c.applyBgColor(string([]byte{}), width)
		lines = append(lines, line)
	}

	return lines
}

func (c *Container) SetMetadata(data Data) *Container {
	c.data = data
	return c
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
		_, _ = fmt.Fprintf(a.writer, "\033[%dA", a.lastLines)
	}
	linesThisFrame := 0
	// Render each container
	for _, container := range a.containers {
		// Get rendered lines from container
		lines := container.RenderLines(a.data)

		// Write each line with proper clearing
		for _, line := range lines {
			// Clear current line + return to column 0
			_, _ = fmt.Fprint(a.writer, "\033[2K\r")

			// Write line content
			_, _ = fmt.Fprint(a.writer, line)

			// Move to next line
			_, _ = fmt.Fprint(a.writer, "\n")

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
		_, _ = fmt.Fprintln(a.writer)
	}

	// Set initial line count
	a.lastLines = totalLines

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Move cursor past rendered content on exit
			_, _ = fmt.Fprintln(a.writer)
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

// func (a *App) AddContainer(c *Container) int {
// 	a.containers = append(a.containers, c)
// 	return len(a.containers) - 1
// }
//
// func (a *App) UpdateContainer(idx int, c *Container) {
// }

func (a *App) SetMetadata(md map[string]any) {
	a.data = md
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

func NewContainers(num int, data Data, layout Layout, style Style, e ...*Element) []*Container {
	containers := make([]*Container, num)
	for i := range containers {
		containers[i] = &Container{
			elements: e,
			data:     data,
			Layout:   layout,
			Style:    style,
		}
	}
	return containers
}

func (c *Container) WithStyle(s Style) *Container {
	c.Style = s
	return c
}

func (c *Container) WithLayout(l Layout) *Container {
	c.Layout = l
	return c
}

// NewApp creates a new App using the given writer and containers and children.
func NewApp(wr io.Writer, data Data, containers ...*Container) *App {
	return &App{
		writer:     wr,
		data:       data,
		containers: containers,
		done:       make(chan struct{}),
	}
}
