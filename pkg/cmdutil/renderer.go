package cmdutil

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"sync"
	"time"
)

var frames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

type Renderer interface {
	// Loop runs the rendering loop until context is cancelled
	Loop(context.Context)

	// Wait blocks until the renderer is done
	Wait()

	// Close cleans up resources
	Close() error
}

type Data map[string]any

type App struct {
	data       Data
	containers []*Container
	done       chan struct{}
	lastLines  int
	mu         sync.Mutex
	writer     io.Writer
}

type Container struct {
	Width   int
	Height  int
	Padding [4]int

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
	Padding [4]int //  top, right, bottom, left
}

func (e *Element) spinner() string {
	return fmt.Sprintf("%c", frames[e.frameIdx%len(frames)])
}

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

func (e *Element) Count() int {
	return 1
}

func (c *Container) RenderLines(data Data) []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	lines := make([]string, 0, len(c.elements))

	for i := 0; i < c.Padding[0]; i++ {
		lines = append(lines, string([]byte{}))
	}

	for _, e := range c.elements {
		b := e.Render(data)
		lines = append(lines, string(b))

		// Advance frame tick
		e.frameIdx = (e.frameIdx + 1) % len(frames)
	}

	for i := 0; i < c.Padding[2]; i++ {
		lines = append(lines, string([]byte{}))
	}

	return lines
}

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

func (c *Container) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.elements)
}

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
		fmt.Fprintf(a.writer, "\033[%dA", a.lastLines)
	}
	linesThisFrame := 0
	// Render each container
	for _, container := range a.containers {
		// Get rendered lines from container
		lines := container.RenderLines(a.data)

		// Write each line with proper clearing
		for _, line := range lines {
			// Clear current line + return to column 0
			fmt.Fprint(a.writer, "\033[2K\r")

			// Write line content
			fmt.Fprint(a.writer, line)

			// Move to next line
			fmt.Fprint(a.writer, "\n")

			linesThisFrame++
		}
	}
	// Store line count for next frame
	a.lastLines = linesThisFrame
}

func (a *App) Loop(ctx context.Context) {
	// Calculate total lines needed
	totalLines := a.LineCount()

	// Pre-allocate space for rendering
	for i := 0; i < totalLines; i++ {
		fmt.Fprintln(a.writer)
	}

	// Set initial line count
	a.lastLines = totalLines

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Move cursor past rendered content on exit
			fmt.Fprintln(a.writer)
			return
		case <-ticker.C:
			a.renderFrame()
		}
	}
}

func (a *App) LineCount() int {
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

func NewContainer(opts Layout, r ...*Element) *Container {
	return &Container{
		elements: r,
		Width:    1,
		Height:   1,
		Padding:  opts.Padding,
	}
}

func NewApp(wr io.Writer, data Data, containers ...*Container) *App {
	return &App{
		writer:     wr,
		data:       data,
		containers: containers,
		done:       make(chan struct{}),
	}
}
