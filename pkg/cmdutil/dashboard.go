package cmdutil

import (
	"context"
	"fmt"
	"os"
	"sync"
	"text/tabwriter"
	"time"
)

type Color string

const (
	ColorReset  Color = "\x1b[0m"
	ColorRed    Color = "\x1b[31m"
	ColorGreen  Color = "\x1b[32m"
	ColorYellow Color = "\x1b[33m"
	ColorCyan   Color = "\x1b[36m"
)

// ServiceState represents One line in the dashboard
type ServiceState struct {
	Name    string
	Text    string
	Color   Color
	Done    bool
	Failed  bool
	spinIdx int
}

// Dashboard holds all services + rendering logic
type Dashboard struct {
	mu       sync.Mutex
	services []*ServiceState
	tw       *tabwriter.Writer
	done     chan struct{}
}

// NewDashboard creates the dashboard with one ServiceState per name.
func NewDashboard(names []string) *Dashboard {
	svcs := make([]*ServiceState, len(names))
	for i, n := range names {
		svcs[i] = &ServiceState{
			Name:  n,
			Text:  "starting…",
			Color: ColorYellow,
		}
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	return &Dashboard{
		services: svcs,
		tw:       tw,
		done:     make(chan struct{}),
	}
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

// Loop runs the renderer until ctx is done.
func (d *Dashboard) Loop(ctx context.Context) {
	defer func() {
		_ = d.tw.Flush()
	}()

	defer close(d.done)

	frames := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

	// Print initial empty lines for each service so we have space to rewrite.
	for range d.services {
		_, _ = fmt.Fprintln(d.tw)
	}
	_ = d.tw.Flush()

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

func (d *Dashboard) DoneMsg(idx int, msg string) {
	d.Update(idx, func(s *ServiceState) {
		s.Done = true
		s.Text = msg
		s.Color = ColorGreen
	})
}

func (d *Dashboard) Done(idx int) {
	d.Update(idx, func(s *ServiceState) {
		s.Done = true
	})
}

func (d *Dashboard) FailMsg(idx int, msg string) {
	d.Update(idx, func(s *ServiceState) {
		s.Done = true
		s.Failed = true
		s.Text = msg
	})
}

func (d *Dashboard) Fail(idx int) {
	d.Update(idx, func(s *ServiceState) {
		s.Done = true
		s.Failed = true
	})
}

func (d *Dashboard) FailAfter(idx int, after time.Duration) {
	go func() {
		time.Sleep(after)
		d.Fail(idx)
	}()
}

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

	// Move cursor up N lines (where N = number of services)
	if len(d.services) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "\033[%dA", len(d.services))
	}

	// Clear each line and redraw via tabwriter
	for _, s := range d.services {

		// advance spinner if not done
		spin := '✔' // no spinner if done
		if !s.Done {
			s.spinIdx = (s.spinIdx + 1) % len(frames)
			spin = frames[s.spinIdx]
		}

		// Update spinner if it is marked as failed
		if s.Failed {
			spin = '✖'
			s.Color = ColorRed
		}

		_, _ = fmt.Fprint(d.tw, "\033[2K") // clear current line
		_, _ = fmt.Fprintf(
			d.tw,
			"  %s\t%s%c %s%s\n",
			s.Name,
			s.Color,
			spin,
			s.Text,
			ColorReset,
		)
	}

	_ = d.tw.Flush()
}

// renderFinal draws a final snapshot (no spinning)
func (d *Dashboard) renderFinal() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.services) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "\033[%dA", len(d.services))
	}

	for _, s := range d.services {
		icon := "✔"
		color := ColorGreen
		if s.Failed {
			icon = "✖"
			color = ColorRed
		}
		_, _ = fmt.Fprint(d.tw, "\033[2K")
		_, _ = fmt.Fprintf(
			d.tw,
			"  %s\t%s%s %s%s\n",
			s.Name,
			color,
			icon,
			s.Text,
			ColorReset,
		)
	}

	_ = d.tw.Flush()
}

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
