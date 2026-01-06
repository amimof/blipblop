package cmdutil

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/juju/ansiterm"
)

const (
	ColorReset Color = "\x1b[0m"

	FgRed     Color = "\x1b[31m"
	FgGreen   Color = "\x1b[32m"
	FgYellow  Color = "\x1b[33m"
	FgCyan    Color = "\x1b[36m"
	FgGrey245 Color = "\x1b[38;5;245m"
	FgGrey240 Color = "\x1b[38;5;240m"
	FgGrey238 Color = "\x1b[38;5;238m"
	FgGrey237 Color = "\x1b[38;5;237m"
	FgGrey236 Color = "\x1b[38;5;236m"

	Fg23 Color = "\x1b[38;5;23m"
	Fg53 Color = "\x1b[38;5;54m"
	Fg92 Color = "\x1b[38;5;92m"

	BgGrey242 Color = "\x1b[48;5;242m"
	BgGrey238 Color = "\x1b[48;5;238m"
	BgGrey236 Color = "\x1b[48;5;236m"
	BgGrey235 Color = "\x1b[48;5;235m"
	BgGrey233 Color = "\x1b[48;5;233m"
)

type Color string

type Option func(*Dashboard)

var (
	DefaultColoredTabWriter = ansiterm.NewTabWriter(os.Stdout, 0, 8, 2, ' ', 0)
	DefaultTabWriter        = tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
)

// WithWriter assigns a io.Writer that the Dashboard will render to.
// The default writer is os.Stdout. If the writer literal types can be cast to
// either a tabwriter.Writer or ansiterm.TabWriter (3rd party lib) then their Flush()
// methods will be assigned as the loopFunc. see WithLoopFunc for more info.
// Basically it is set here so the user doesnt have to bother.
func WithWriter(w io.Writer) Option {
	return func(d *Dashboard) {
		d.writer = w
		switch w := w.(type) {
		case *tabwriter.Writer:
			d.loopFunc = func() {
				_ = w.Flush()
			}
		case *ansiterm.TabWriter:
			d.loopFunc = func() {
				_ = w.Flush()
			}
		}
	}
}

// WithLoopFunc adds a function to the dashboard that is executed on each render loop.
// This is useful when using writers that requires flushing. Such as the build-in tabwriter pkg writer.
func WithLoopFunc(f func()) Option {
	return func(d *Dashboard) {
		d.loopFunc = f
	}
}

// ServiceState represents One line in the dashboard
type ServiceState struct {
	Name    string
	Text    string
	Color   Color
	Done    bool
	Failed  bool
	spinIdx int
	Details []Detail
}

// Dashboard holds all services + rendering logic
type Dashboard struct {
	mu        sync.Mutex
	services  []*ServiceState
	writer    io.Writer
	done      chan struct{}
	lastLines int
	loopFunc  func()
}

// Detail represents a line in the details view of a ServiceState.
// It's pretty much just a key-value pair
type Detail struct {
	Key   string
	Value string
}

// SetDetails assigns a new slice, overwriting any other Detail sets previously used.
// If you want to update an existing line then use UpdateDetail()
func (d *Dashboard) SetDetails(idx int, lines []Detail) {
	d.Update(idx, func(s *ServiceState) {
		s.Details = lines
	})
}

// UpdateDetails inserts a new line. If a line with same key exists then that line is updated.
// So two lines with the same key cannot exist in the slice.
func (d *Dashboard) UpdateDetails(idx int, key, value string) {
	d.Update(idx, func(s *ServiceState) {
		for i, d := range s.Details {
			if d.Key == key {
				s.Details[i] = Detail{Key: key, Value: value}
				return
			}
		}
		s.Details = append(s.Details, Detail{Key: key, Value: value})
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
func (d *Dashboard) UpdateText(idx int, text string) {
	d.Update(idx, func(s *ServiceState) {
		s.Text = text
	})
}

// Loop runs the renderer until ctx is done.
func (d *Dashboard) Loop(ctx context.Context) {
	defer func() {
		// _ = d.tw.Flush()
		d.loopFunc()
	}()

	defer close(d.done)

	frames := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

	// Print initial empty lines for each service so we have space to rewrite.
	for range d.services {
		// _, _ = fmt.Fprintln(d.tw)
		_, _ = fmt.Fprintln(d.writer)
	}
	// _ = d.tw.Flush()
	d.loopFunc()
	d.lastLines = len(d.services)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.renderFinal()
			return
		case <-ticker.C:
			d.renderFrame(frames)
		}
	}
}

// DoneMsg sets the provided message when the Dashboard is done
func (d *Dashboard) DoneMsg(idx int, msg string) {
	d.Update(idx, func(s *ServiceState) {
		s.Done = true
		s.Text = msg
		s.Color = FgGreen
	})
}

// Done marks the service entry at idx as done
func (d *Dashboard) Done(idx int) {
	d.Update(idx, func(s *ServiceState) {
		s.Done = true
	})
}

// FailMsg sets the provided message and marks the service as failed
func (d *Dashboard) FailMsg(idx int, msg string) {
	d.Update(idx, func(s *ServiceState) {
		s.Done = true
		s.Failed = true
		s.Text = msg
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
	<-d.done
}

// WaitAnd blocks until Loop finishes and executes the provided function when done
func (d *Dashboard) WaitAnd(fn func()) {
	go func() {
		for {
			time.Sleep(200 * time.Millisecond)
			if d.IsDone() {
				fn()
				return
			}
		}
	}()
	d.Wait()
}

// renderFrame draws all service lines with spinners.
func (d *Dashboard) renderFrame(frames []rune) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.lastLines > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "\033[%dA", d.lastLines)
	}

	linesThisFrame := 0

	// Clear each line and redraw via tabwriter
	for _, s := range d.services {

		// advance spinner if not done
		spin := "✔" // no spinner if done
		if !s.Done {
			s.spinIdx = (s.spinIdx + 1) % len(frames)
			spin = fmt.Sprintf("%s%c%s", Fg92, frames[s.spinIdx], ColorReset)
		}

		// Update spinner if it is marked as failed
		if s.Failed {
			spin = "✖"
			s.Color = FgRed
		}

		// _, _ = fmt.Fprint(d.tw, "\033[2K") // clear current line
		_, _ = fmt.Fprint(d.writer, "\033[2K") // clear current line
		_, _ = fmt.Fprintf(
			// d.tw,
			d.writer,
			" %s %s\t%s%s%s\n",
			spin,
			s.Name,
			s.Color,
			s.Text,
			ColorReset,
		)
		linesThisFrame++

		// detail lines (indented; no spinner)
		for _, line := range s.Details {
			// _, _ = fmt.Fprint(d.tw, "\033[2K")
			_, _ = fmt.Fprint(d.writer, "\033[2K")
			_, _ = fmt.Fprintf(
				// d.tw,
				d.writer,
				"   %s%s:\t%s%s\n",
				FgGrey245,
				line.Key,
				line.Value,
				ColorReset,
			)
			linesThisFrame++
		}
	}

	// _ = d.tw.Flush()
	d.loopFunc()
	d.lastLines = linesThisFrame
}

// renderFinal draws a final snapshot (no spinning)
func (d *Dashboard) renderFinal() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.lastLines > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "\033[%dA", d.lastLines)
	}

	linesThisFrame := 0

	for _, s := range d.services {

		color := FgGreen
		icon := fmt.Sprintf("%s✔%s", color, ColorReset)

		if s.Failed {
			color = FgRed
			icon = fmt.Sprintf("%s✖%s", color, ColorReset)
		}
		// _, _ = fmt.Fprint(d.tw, "\033[2K")
		_, _ = fmt.Fprint(d.writer, "\033[2K")
		_, _ = fmt.Fprintf(
			// d.tw,
			d.writer,
			" %s %s\t%s%s%s\n",
			icon,
			s.Name,
			color,
			s.Text,
			ColorReset,
		)

		linesThisFrame++

		// detail lines (indented; no spinner)
		for _, line := range s.Details {
			// _, _ = fmt.Fprint(d.tw, "\033[2K")
			_, _ = fmt.Fprint(d.writer, "\033[2K")
			_, _ = fmt.Fprintf(
				// d.tw,
				d.writer,
				"   %s%s:\t%s%s\n",
				FgGrey245,
				line.Key,
				line.Value,
				ColorReset,
			)
			linesThisFrame++
		}
	}

	// _ = d.tw.Flush()
	d.loopFunc()
	d.lastLines = linesThisFrame
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
func NewDashboard(names []string, opts ...Option) *Dashboard {
	svcs := make([]*ServiceState, len(names))
	for i, n := range names {
		svcs[i] = &ServiceState{
			Name:  n,
			Text:  "starting…",
			Color: FgYellow,
		}
	}

	d := &Dashboard{
		services: svcs,
		done:     make(chan struct{}),
		writer:   os.Stdout,
		loopFunc: func() {},
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}
