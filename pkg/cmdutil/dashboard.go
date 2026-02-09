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

// // WithDefaultText sets the text before UpdateText is called
// func WithDefaultText(text string) Option {
// 	return func(d *Dashboard) {
// 		for _, s := range d.services {
// 			s.Text = text
// 		}
// 	}
// }

// WithEmptyText sets the text to display when the list of services is empty
func WithEmptyText(text string) Option {
	return func(d *Dashboard) {
		d.IsDone()
		d.emptyText = text
	}
}

// func WithFormat(fmt string) Option {
// 	return func(d *Dashboard) {
// 		d.formatStr = fmt
// 	}
// }

// WithHeader sets a header line that will be rendered once at the start
func WithHeader(header string) Option {
	return func(d *Dashboard) {
		d.app.UpdateMetadata("Prefix", header)
	}
}

// WithMaxServices sets a limit of how many services can be displayed on each render frame.
// func WithMaxServices(max int) Option {
// 	return func(d *Dashboard) {
// 		d.maxServices = max
// 	}
// }

// ServiceState represents One line in the dashboard
type ServiceState struct {
	Done      bool
	DoneMsg   string
	Failed    bool
	FailedMsg string
	task      *tasksv1.Task
	container *Container

	// failedIcon  string
	// successIcon string
}

// Dashboard holds all services + rendering logic
type Dashboard struct {
	Name      string
	mu        sync.Mutex
	services  []*ServiceState
	done      chan struct{}
	flushFunc func()
	emptyText string
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

// UpdateMetadata updates metadata for template access
// func (d *Dashboard) UpdateMetadata(idx int, key, value string) {
// 	d.Update(idx, func(s *ServiceState) {
// 		if s.Metadata == nil {
// 			s.Metadata = make(map[string]any)
// 		}
// 		s.Metadata[key] = value
// 	})
// }

// SetMetadata replaces all metadata
// func (d *Dashboard) SetMetadata(idx int, metadata map[string]any) {
// 	d.Update(idx, func(s *ServiceState) {
// 		s.Metadata = metadata
// 	})
// }

// SetDetails assigns a new slice, overwriting any other Detail sets previously used.
// If you want to update an existing line then use UpdateDetail()
// func (d *Dashboard) SetDetails(idx int, lines []Detail) {
// 	d.Update(idx, func(s *ServiceState) {
// 		s.Details = lines
// 	})
// }

// UpdateDetails inserts a new line. If a line with same key exists then that line is updated.
// So two lines with the same key cannot exist in the slice.
// func (d *Dashboard) UpdateDetails(idx int, key, value string) {
// 	d.Update(idx, func(s *ServiceState) {
// 		for i, d := range s.Details {
// 			if d.Key == key {
// 				s.Details[i] = Detail{Key: key, Value: value}
// 				return
// 			}
// 		}
// 		s.Details = append(s.Details, Detail{Key: key, Value: value})
// 	})
// }

// AddService adds a new service dynamically (returns index)
func (d *Dashboard) AddTask(t *tasksv1.Task) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.services = append(d.services, &ServiceState{task: t})

	return len(d.services) - 1
}

// AddService adds a new service dynamically (returns index)
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

// UpdateText lets workers mutate a single service under lock.
// func (d *Dashboard) UpdateText(idx int, text string) {
// 	d.Update(idx, func(s *ServiceState) {
// 		s.Text = text
// 	})
// }

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
	d.app.Wait()
	// <-d.done
}

// Loop calls loop on the underlying App instance passing the context through to it
func (d *Dashboard) Loop(ctx context.Context) {
	d.app.Loop(ctx)
}

// WaitAnd blocks until Loop finishes and executes the provided function when done
func (d *Dashboard) WaitAnd(fn func()) {
	go func() {
		for {
			time.Sleep(200 * time.Millisecond)
			if d.IsDone() && len(d.services) > 0 {
				fn()
				return
			}
		}
	}()
	d.Wait()
}

// parseColumns parses a format string into column specifications.
// Example: "{{ .Name }}|20|{{ .Text }}|15|{{ .Status }}"
// Returns columns and whether column syntax was detected.
// func parseColumns(formatStr string) ([]Column, bool) {
// 	// Check if format uses column syntax (contains |digit|)
// 	if !regexp.MustCompile(`\|\d+\|?`).MatchString(formatStr) {
// 		return nil, false // Not using column syntax
// 	}
//
// 	var columns []Column
//
// 	// Split by pipe to find column boundaries
// 	// Pattern: template|width|template|width|template
// 	parts := strings.Split(formatStr, "|")
//
// 	var currentTemplate strings.Builder
//
// 	for i := range parts {
// 		part := parts[i]
//
// 		// Check if this part is a width specification (pure number)
// 		part = strings.TrimSpace(part)
// 		if width, err := strconv.Atoi(part); err == nil {
// 			// This is a width spec, save current column
// 			if currentTemplate.Len() > 0 {
// 				columns = append(columns, Column{
// 					Template: strings.TrimSpace(currentTemplate.String()),
// 					Width:    width,
// 				})
// 				currentTemplate.Reset()
// 			}
// 		} else {
// 			// This is template content
// 			if currentTemplate.Len() > 0 {
// 				currentTemplate.WriteString("|") // Restore pipe within template
// 			}
// 			currentTemplate.WriteString(part)
// 		}
// 	}
//
// 	// Handle last column (may not have trailing |width)
// 	if currentTemplate.Len() > 0 {
// 		columns = append(columns, Column{
// 			Template: strings.TrimSpace(currentTemplate.String()),
// 			Width:    0, // Last column unlimited by default
// 		})
// 	}
//
// 	return columns, len(columns) > 0
// }

// renderServiceWithColumns renders a ServiceState using column-based formatting.
// Each column is rendered independently, truncated to its max width, and concatenated.
// func (d *Dashboard) renderServiceWithColumns(s *ServiceState) (string, error) {
// 	var result strings.Builder
// 	for i, col := range d.columns {
// 		content, err := d.renderCol(col, s)
// 		if err != nil {
// 			return "", fmt.Errorf("col %d render error: %w", i, err)
// 		}
// 		result.WriteString(content)
// 	}
// 	return result.String(), nil
// }
//
// func (d *Dashboard) renderHeaderWithColumns() (string, error) {
// 	var result strings.Builder
// 	for i, col := range d.headerCols {
// 		content, err := d.renderCol(col, d)
// 		if err != nil {
// 			return "", fmt.Errorf("col %d render error: %w", i, err)
// 		}
// 		result.WriteString(content)
// 	}
// 	return result.String(), nil
// }
//
// func (d *Dashboard) renderCol(c Column, data any) (string, error) {
// 	var buf bytes.Buffer
// 	err := c.Parsed.Funcs(templateFuncs).Execute(&buf, data)
// 	if err != nil {
// 		return "", fmt.Errorf("column execution error: %w", err)
// 	}
//
// 	content := buf.String()
//
// 	// Truncate to column width if specified
// 	if c.Width > 0 {
// 		content = truncateWithEllipsis(content, c.Width)
// 		// Pad to exact width (for alignment)
// 		content = padRight(content, c.Width)
// 	}
// 	return content, nil
// }

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

	svcs := make([]*ServiceState, len(names))
	for i, n := range names {
		data := map[string]any{
			"Done":      false,
			"Failed":    false,
			"FailedMsg": "",
			"Name":      n,
			"Phase":     "",
			"Node":      "",
			"Pid":       "",
			"ID":        "",
			"Image":     "",
		}
		svc := &ServiceState{
			task: &tasksv1.Task{Meta: &types.Meta{Name: n}},
			container: NewContainer(data,
				NewElement(`{{ if .Container.Failed }}{{"✖" | FgRed }} {{ .Container.FailedMsg | FgRed }}{{else if .Container.Done }}{{ "✔" | FgGreen }} {{ .Container.DoneMsg | FgGreen }}{{else}}{{ spinner | FgYellow }} {{ .Prefix }} {{ .Container.Name | Bold }}{{end}}`),
				NewElement(`  Phase: {{ if eq .Container.Phase "Running" }}{{ .Container.Phase | FgGreen }}{{else}}{{ .Container.Phase | FgYellow }}{{end}}`),
				NewElement(`  Node: {{ .Container.Node }}`),
				NewElement(`  Pid: {{ .Container.Pid }}`),
				NewElement(`  ID: {{ .Container.ID | FgBlue }}`),
				NewElement(`  Image: {{ .Container.Image | FgBlue }}`),
			).WithLayout(Layout{
				Dimensions: [2]int{width, height},
				Padding:    [4]int{0, 1, 0, 1},
			}).WithStyle(Style{
				// Bg: StyleBg256(234),
			}),
		}

		app.AddContainer(svc.container)
		svcs[i] = svc
	}

	d := &Dashboard{
		Name:      "Name",
		services:  svcs,
		done:      make(chan struct{}),
		flushFunc: func() {},
		emptyText: "Waiting",
		app:       app,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d, nil
}
