package cmdutil

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"sync"
)

type Renderer interface {
	// Loop runs the rendering loop until context is cancelled
	Loop(context.Context)

	// Wait blocks until the renderer is done
	Wait()

	// Close cleans up resources
	Close() error
}

type Data map[string]any

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
}

func (e *Element) Render(d any) []byte {
	buf := bytes.Buffer{}
	b := bytes.NewBuffer(buf.Bytes())
	err := e.Parsed.Execute(b, d)
	if err != nil {
		return nil
	}
	return b.Bytes()
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

func NewContainer(r ...*Element) *Container {
	return &Container{
		elements: r,
		Width:    1,
		Height:   1,
		Padding:  [4]int{0, 0, 0, 0},
	}
}

func (b *Container) Render(data Data) []byte {
	buf := bytes.Buffer{}
	by := bytes.NewBuffer(buf.Bytes())

	for _, e := range b.elements {
		b := e.Render(data)
		_, err := by.Write(b)
		if err != nil {
			panic(err)
		}
		by.WriteByte('\n')
	}
	return by.Bytes()
}

type App struct {
	data       Data
	containers []*Container
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

func NewApp(data Data, containers ...*Container) *App {
	return &App{
		data:       data,
		containers: containers,
	}
}
