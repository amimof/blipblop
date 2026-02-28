package cmdutil

import (
	"context"
	"html/template"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	"github.com/amimof/voiyd/api/types/v1"
)

type Color string

// Field identifies a detail line that can be shown per task entry in the dashboard.
type Field int

const (
	FieldPhase Field = iota
	FieldNode
	FieldPid
	FieldID
	FieldImage
)

// fieldTemplate maps each Field to its Go template string.
var fieldTemplate = map[Field]string{
	FieldPhase: `  Phase: {{ if eq .Container.Phase "Running" }}{{ .Container.Phase | FgGreen }}{{else}}{{ .Container.Phase | FgYellow }}{{end}}`,
	FieldNode:  `  Node: {{ .Container.Node }}`,
	FieldPid:   `  Pid: {{ .Container.Pid }}`,
	FieldID:    `  ID: {{ .Container.ID | FgBlue }}`,
	FieldImage: `  Image: {{ .Container.Image | FgBlue }}`,
}

// defaultFields is the ordered set of fields shown when WithFields is not called.
var defaultFields = []Field{FieldPhase, FieldNode, FieldPid, FieldID, FieldImage}

type Option func(*Dashboard)

// WithWriter assigns a io.Writer that the Dashboard will render to.
// The default writer is os.Stdout. If the writer literal types can be cast to
// a tabwriter.Writer its Flush() methods will be assigned as the loopFunc. see WithLoopFunc for more info.
// Basically it is set here so the user doesn't have to bother.
func WithWriter(w io.Writer) Option {
	return func(d *Dashboard) {
		d.app.Writer = w
	}
}

// WithFlushFunc adds a handler to the dashboard that is executed on each render loop.
// This is useful when for writers that require flushing. Such as the build-in tabwriter pkg writer.
func WithFlushFunc(f func()) Option {
	return func(d *Dashboard) {
		d.flushFunc = f
	}
}

// WithEmptyText sets the text to display when the list of services is empty
func WithEmptyText(text string) Option {
	return func(d *Dashboard) {
		d.IsDone()
		d.emptyText = text
	}
}

// WithHeader sets a header line that will be rendered once at the start
func WithHeader(header string) Option {
	return func(d *Dashboard) {
		d.app.UpdateMetadata("Prefix", header)
	}
}

// WithFields sets which detail fields are displayed beneath each task entry.
// Fields are rendered in the order they appear in defaultFields regardless of
// the order they are passed here.
// If WithFields is not called, all fields are shown (equivalent to passing all
// Field constants).
func WithFields(fields ...Field) Option {
	return func(d *Dashboard) {
		d.fields = fields
	}
}

// ServiceState represents One line in the dashboard
type ServiceState struct {
	Done      bool
	DoneMsg   string
	Failed    bool
	FailedMsg string
	task      *tasksv1.Task
	container *Container
}

// Dashboard holds all services + rendering logic
type Dashboard struct {
	Name      string
	mu        sync.Mutex
	services  []*ServiceState
	done      chan struct{}
	flushFunc func()
	emptyText string
	fields    []Field // detail fields to display per task; nil means all fields
	app       *App
}

// Column defines a single column in the dashboard output
type Column struct {
	Template string             // Raw template string for this column
	Width    int                // Max width (0 = unlimited)
	Parsed   *template.Template // Compiled template (set during initialization)
}

// Detail represents a line in the details view of a ServiceState.
// It's pretty much just a key-value pair
type Detail struct {
	Key   string
	Value string
}

// AddTask adds a new task dynamically to dashboard (returns index)
func (d *Dashboard) AddTask(t *tasksv1.Task) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.services = append(d.services, &ServiceState{task: t})

	return len(d.services) - 1
}

// SetTask overwrites existing task at idx a new instance dynamically
func (d *Dashboard) SetTask(idx int, t *tasksv1.Task) {
	d.Update(idx, func(s *ServiceState) {
		s.task = t
		s.container.SetMetadata(map[string]any{
			"Name":  t.GetMeta().GetName(),
			"Phase": t.GetStatus().GetPhase().GetValue(),
			"Node":  t.GetStatus().GetNode().GetValue(),
			"Pid":   t.GetStatus().GetPid().GetValue(),
			"ID":    t.GetStatus().GetId().GetValue(),
			"Image": t.GetConfig().GetImage(),
		})
	})
}

// Update lets workers mutate a single service under lock.
func (d *Dashboard) Update(idx int, fn func(s *ServiceState)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.services[idx].Done {
		return
	}
	fn(d.services[idx])
}

// DoneMsg sets the provided message when the Dashboard is done
func (d *Dashboard) DoneMsg(idx int, msg string) {
	d.Update(idx, func(s *ServiceState) {
		s.container.UpdateMetadata("Done", true)
		s.container.UpdateMetadata("DoneMsg", msg)
		s.Done = true
		s.DoneMsg = msg
	})
}

// Done marks the service entry at idx as done
func (d *Dashboard) Done(idx int) {
	d.Update(idx, func(s *ServiceState) {
		s.container.UpdateMetadata("Done", true)
		s.container.UpdateMetadata("DoneMsg", "")
		s.Done = true
	})
}

// FailMsg sets the provided message and marks the service as failed
func (d *Dashboard) FailMsg(idx int, msg string) {
	d.Update(idx, func(s *ServiceState) {
		s.container.UpdateMetadata("Failed", true)
		s.container.UpdateMetadata("FailedMsg", msg)
		s.Done = true
		s.Failed = true
		s.FailedMsg = msg
	})
}

// Fail marks the service as failed
func (d *Dashboard) Fail(idx int) {
	d.Update(idx, func(s *ServiceState) {
		s.Done = true
		s.Failed = true
	})
}

// FailAfter marks the service as faild when x amount of time as elapsed
func (d *Dashboard) FailAfter(idx int, after time.Duration) {
	go func() {
		time.Sleep(after)
		d.Fail(idx)
	}()
}

// FailAfterMsg sets the provided message marks the service as faild when x amount of time as elapsed
func (d *Dashboard) FailAfterMsg(idx int, after time.Duration, msg string) {
	go func() {
		time.Sleep(after)
		d.FailMsg(idx, msg)
	}()
}

// Wait blocks until Loop finishes.
func (d *Dashboard) Wait() {
	go func() {
		for {
			time.Sleep(200 * time.Millisecond)
			if d.IsDone() && len(d.services) > 0 {
				d.done <- struct{}{}
			}
		}
	}()
	<-d.done
}

// WaitAnd blocks until Loop finishes and executes the provided function when done
func (d *Dashboard) WaitAnd(fn func()) {
	defer fn()
	d.Wait()
}

// Loop calls loop on the underlying App instance passing the context through to it
func (d *Dashboard) Loop(ctx context.Context) {
	defer close(d.done)

	d.app.Loop(ctx)
}

// IsDone return true if all services in the Dashboard is marked as done
func (d *Dashboard) IsDone() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.services {
		if !s.Done && !s.Failed {
			return false
		}
	}
	return true
}

// NewDashboard creates the dashboard with one ServiceState per name.
func NewDashboard(names []string, opts ...Option) (*Dashboard, error) {
	var width, height int
	// Get width of terminal
	if term.IsTerminal(0) {
		w, h, err := term.GetSize(0)
		if err != nil {
			return nil, err
		}
		width = w
		height = h
	}

	app := NewApp(os.Stdout,
		map[string]any{},
	)

	// Build a partial Dashboard and apply options first so that d.fields is
	// known before we construct the per-task containers.
	d := &Dashboard{
		Name:      "Name",
		done:      make(chan struct{}),
		flushFunc: func() {},
		emptyText: "Waiting",
		app:       app,
	}

	for _, opt := range opts {
		opt(d)
	}

	// Resolve effective field set: nil means "all fields" (default behaviour).
	activeFields := d.fields
	if activeFields == nil {
		activeFields = defaultFields
	}

	// Build a set for O(1) membership checks, preserving defaultFields order.
	fieldSet := make(map[Field]struct{}, len(activeFields))
	for _, f := range activeFields {
		fieldSet[f] = struct{}{}
	}

	svcs := make([]*ServiceState, len(names))
	for i, n := range names {
		data := map[string]any{
			"Done":      false,
			"DoneMsg":   "",
			"Failed":    false,
			"FailedMsg": "",
			"Name":      n,
			"Phase":     "",
			"Node":      "",
			"Pid":       "",
			"ID":        "",
			"Image":     "",
		}

		// Status line is always present.
		elements := []*Element{
			NewElement(`{{ if .Container.Failed }}{{"✖" | FgRed }} {{ .Container.FailedMsg | FgRed }}{{else if .Container.Done }}{{ "✔" | FgGreen }} {{ .Container.DoneMsg | FgGreen }}{{else}}{{ spinner | FgYellow }} {{ .Prefix }} {{ .Container.Name | Bold }}{{end}}`),
		}

		// Append one element per active field, in defaultFields order.
		for _, f := range defaultFields {
			if _, ok := fieldSet[f]; ok {
				elements = append(elements, NewElement(fieldTemplate[f]))
			}
		}

		svc := &ServiceState{
			task: &tasksv1.Task{Meta: &types.Meta{Name: n}},
			container: NewContainer(data, elements...).WithLayout(Layout{
				Dimensions: [2]int{width, height},
				Padding:    [4]int{0, 1, 0, 1},
			}).WithStyle(Style{
				// Bg: StyleBg256(234),
			}),
		}

		app.AddContainer(svc.container)
		svcs[i] = svc
	}

	d.services = svcs

	return d, nil
}
