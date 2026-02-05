package cmdutil

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

type Color string

type Option func(*Dashboard)

// WithWriter assigns a io.Writer that the Dashboard will render to.
// The default writer is os.Stdout. If the writer literal types can be cast to
// a tabwriter.Writer its Flush() methods will be assigned as the loopFunc. see WithLoopFunc for more info.
// Basically it is set here so the user doesn't have to bother.
func WithWriter(w io.Writer) Option {
	return func(d *Dashboard) {
		d.writer = w
		switch w := w.(type) {
		case *tabwriter.Writer:
			d.flushFunc = func() {
				_ = w.Flush()
			}
		}
	}
}

// WithFlushFunc adds a handler to the dashboard that is executed on each render loop.
// This is useful when for writers that require flushing. Such as the build-in tabwriter pkg writer.
func WithFlushFunc(f func()) Option {
	return func(d *Dashboard) {
		d.flushFunc = f
	}
}

// WithDefaultText sets the text before UpdateText is called
func WithDefaultText(text string) Option {
	return func(d *Dashboard) {
		for _, s := range d.services {
			s.Text = text
		}
	}
}

// WithEmptyText sets the text to display when the list of services is empty
func WithEmptyText(text string) Option {
	return func(d *Dashboard) {
		d.IsDone()
		d.emptyText = text
	}
}

func WithFormat(fmt string) Option {
	return func(d *Dashboard) {
		d.formatStr = fmt
	}
}

// WithHeader sets a header line that will be rendered once at the start
// The header can use template syntax if it contains {{ }}, otherwise it's treated as plain text
func WithHeader(header string) Option {
	return func(d *Dashboard) {
		d.hasHeader = true
		d.headerStr = header
	}
}

// WithMaxServices sets a limit of how many services can be displayed on each render frame.
func WithMaxServices(max int) Option {
	return func(d *Dashboard) {
		d.maxServices = max
	}
}

// ServiceState represents One line in the dashboard
type ServiceState struct {
	Name     string
	Text     string
	Color    Color
	Done     bool
	Failed   bool
	Metadata map[string]any

	spinIdx     int
	Details     []Detail
	failedIcon  string
	successIcon string
}

// Dashboard holds all services + rendering logic
type Dashboard struct {
	Name        string
	mu          sync.Mutex
	services    []*ServiceState
	maxServices int
	writer      io.Writer
	done        chan struct{}
	lastLines   int
	flushFunc   func()
	emptyText   string

	// template  *template.Template
	formatStr string

	// Header management
	headerStr     string // Raw header string (template or plain text)
	hasHeader     bool   // Whether to render a header
	headerWritten bool   // Track if header was writte

	// NEW: Column-based rendering
	headerCols []Column
	columns    []Column // Parsed column specifications
	useColumns bool     // Whether to use column-based rendering
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

func (s *ServiceState) Spinner(frames []rune, frameIdx int) string {
	if !s.Done {
		if len(frames) > 0 {
			return fmt.Sprintf("%c", frames[frameIdx%len(frames)])
		}
		return "⠿"
	}
	if s.Failed {
		return fmt.Sprintf("%s%s", fg("✖").FgRed(), attr("").Reset())
	}
	return fmt.Sprintf("%s%s", fg("✔").FgGreen(), attr("").Reset())
}

// UpdateMetadata updates metadata for template access
func (d *Dashboard) UpdateMetadata(idx int, key, value string) {
	d.Update(idx, func(s *ServiceState) {
		if s.Metadata == nil {
			s.Metadata = make(map[string]any)
		}
		s.Metadata[key] = value
	})
}

// SetMetadata replaces all metadata
func (d *Dashboard) SetMetadata(idx int, metadata map[string]any) {
	d.Update(idx, func(s *ServiceState) {
		s.Metadata = metadata
	})
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

// AddService adds a new service dynamically (returns index)
func (d *Dashboard) AddService(name string) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	s := &ServiceState{
		Name: name,
		Text: "",
		// Color:    FgYellow,
		Metadata: make(map[string]any),
	}

	d.services = append(d.services, s)

	// Apply ring buffer if enabled
	if d.maxServices > 0 && len(d.services) > d.maxServices {
		d.services = d.services[len(d.services)-d.maxServices:]
	}

	return len(d.services) - 1
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
		d.flushFunc()
	}()

	defer close(d.done)

	frames := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

	// Re-render header once
	if d.hasHeader && !d.headerWritten {
		_, _ = fmt.Fprint(d.writer, "\033[2K")         // Clear line
		headerLine, err := d.renderHeaderWithColumns() // NEW: Use helper
		if err != nil {
			_, _ = fmt.Fprintf(d.writer, "Error rendering header: %v", err)
		}
		_, _ = fmt.Fprint(d.writer, headerLine)
		_, _ = fmt.Fprintln(d.writer)
		_, _ = fmt.Fprintln(d.writer)
		d.headerWritten = true
	}

	// Print initial empty lines for each service so we have space to rewrite.
	for range d.services {
		_, _ = fmt.Fprintln(d.writer)
	}

	d.flushFunc()

	// Include header in line count if present
	d.lastLines = len(d.services)
	if d.hasHeader {
		d.lastLines++
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// d.renderFinal()
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
			if d.IsDone() && len(d.services) > 0 {
				fn()
				return
			}
		}
	}()
	d.Wait()
}

// renderHeader renders the header line (template or plain text)
// func (d *Dashboard) renderHeader() string {
// 	if !d.hasHeader {
// 		return ""
// 	}
// 	// If header is a template, execute it
// 	if d.headerTemplate != nil {
// 		var buf bytes.Buffer
// 		// Execute template with nil data (templates use static strings)
// 		err := d.headerTemplate.Execute(&buf, nil)
// 		if err != nil {
// 			// Fall back to plain text on execution error
// 			return d.headerStr
// 		}
// 		return buf.String()
// 	}
// 	// Plain text header
// 	return d.headerStr
// }

// parseColumns parses a format string into column specifications.
// Example: "{{ .Name }}|20|{{ .Text }}|15|{{ .Status }}"
// Returns columns and whether column syntax was detected.
func parseColumns(formatStr string) ([]Column, bool) {
	// Check if format uses column syntax (contains |digit|)
	if !regexp.MustCompile(`\|\d+\|?`).MatchString(formatStr) {
		return nil, false // Not using column syntax
	}

	var columns []Column

	// Split by pipe to find column boundaries
	// Pattern: template|width|template|width|template
	parts := strings.Split(formatStr, "|")

	var currentTemplate strings.Builder

	for i := range parts {
		part := parts[i]

		// Check if this part is a width specification (pure number)
		part = strings.TrimSpace(part)
		if width, err := strconv.Atoi(part); err == nil {
			// This is a width spec, save current column
			if currentTemplate.Len() > 0 {
				columns = append(columns, Column{
					Template: strings.TrimSpace(currentTemplate.String()),
					Width:    width,
				})
				currentTemplate.Reset()
			}
		} else {
			// This is template content
			if currentTemplate.Len() > 0 {
				currentTemplate.WriteString("|") // Restore pipe within template
			}
			currentTemplate.WriteString(part)
		}
	}

	// Handle last column (may not have trailing |width)
	if currentTemplate.Len() > 0 {
		columns = append(columns, Column{
			Template: strings.TrimSpace(currentTemplate.String()),
			Width:    0, // Last column unlimited by default
		})
	}

	return columns, len(columns) > 0
}

// // isTemplate detects if a string contains Go template syntax
// func isTemplate(s string) bool {
// 	return strings.Contains(s, "{{") && strings.Contains(s, "}}")
// }

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

// renderServiceWithColumns renders a ServiceState using column-based formatting.
// Each column is rendered independently, truncated to its max width, and concatenated.
func (d *Dashboard) renderServiceWithColumns(s *ServiceState) (string, error) {
	var result strings.Builder
	for i, col := range d.columns {
		content, err := d.renderCol(col, s)
		if err != nil {
			return "", fmt.Errorf("col %d render error: %w", i, err)
		}
		result.WriteString(content)
	}
	return result.String(), nil
}

func (d *Dashboard) renderHeaderWithColumns() (string, error) {
	var result strings.Builder
	for i, col := range d.headerCols {
		content, err := d.renderCol(col, d)
		if err != nil {
			return "", fmt.Errorf("col %d render error: %w", i, err)
		}
		result.WriteString(content)
	}
	return result.String(), nil
}

func (d *Dashboard) renderCol(c Column, data any) (string, error) {
	var buf bytes.Buffer
	err := c.Parsed.Funcs(templateFuncs).Execute(&buf, data)
	if err != nil {
		return "", fmt.Errorf("column execution error: %w", err)
	}

	content := buf.String()

	// Truncate to column width if specified
	if c.Width > 0 {
		content = truncateWithEllipsis(content, c.Width)
		// Pad to exact width (for alignment)
		content = padRight(content, c.Width)
	}
	return content, nil
}

// renderFrame draws all service lines with spinners.
func (d *Dashboard) renderFrame(frames []rune) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.lastLines > 0 {
		_, _ = fmt.Fprintf(d.writer, "\033[%dA", d.lastLines)
		if len(d.services) == 0 {
			_, _ = fmt.Fprint(d.writer, "\033[2K")
			_, _ = fmt.Fprintf(d.writer, "%s", d.emptyText)
		}
	}

	linesThisFrame := 0

	// Clear each line and redraw via tabwriter
	for _, s := range d.services {

		// Advance spinner if not done
		if !s.Done {
			s.spinIdx = (s.spinIdx + 1) % len(frames)
		}

		// Update spinner function for this service
		templateFuncs["spinner"] = func() string {
			return s.Spinner(frames, s.spinIdx)
		}

		rendered, err := d.renderServiceWithColumns(s)
		if err != nil {
			fmt.Fprintf(d.writer, "Error rendering: %v", err)
		} else {
			_, _ = fmt.Fprint(d.writer, rendered)
		}

		_, _ = fmt.Fprintln(d.writer)
		linesThisFrame++

		// detail lines (indented; no spinner)
		for _, line := range s.Details {
			// key := fmt.Sprintf("%s%s%s", FgGrey245, line.Key, ColorReset)
			// val := fmt.Sprintf("%s%s%s", FgGrey245, line.Value, ColorReset)
			key := fmt.Sprintf("%s%s%s", "", line.Key, "")
			val := fmt.Sprintf("%s%s%s", "", line.Value, "")
			_, _ = fmt.Fprint(d.writer, "\033[2K")

			// formatStrFinal := fmt.Sprintf("%s {{ .Name }}", line)
			//
			// finalTmpl, err := template.New("line").Funcs(templateFuncs).Parse(formatStrFinal)
			// if err != nil {
			// 	panic(err)
			// }

			// err = finalTmpl.Execute(d.writer, s)
			// if err != nil {
			// 	panic(err)
			// }
			_, _ = fmt.Fprintf(
				d.writer,
				"  %s:\t%s\n",
				key,
				val,
			)
			linesThisFrame++
		}
	}

	d.flushFunc()
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

	// NEW: Re-render header if present
	// if d.hasHeader {
	// 	_, _ = fmt.Fprint(d.writer, "\033[2K") // Clear line
	// 	headerLine := d.renderHeader()         // NEW: Use helper
	// 	_, _ = fmt.Fprintln(d.writer, headerLine)
	// 	linesThisFrame++
	// }

	for _, s := range d.services {

		// color := ""
		// color := FgGreen
		// icon := fmt.Sprintf("%s✔%s", color, ColorReset)
		// icon := fmt.Sprintf("%s%s", fg(s.successIcon).FgGreen(), attr("").Reset())

		// if s.Failed {
		// 	// color = FgRed
		// 	// icon = fmt.Sprintf("%s✖%s", color, ColorReset)
		// 	icon = fmt.Sprintf("%s", fg(s.failedIcon).FgRed(), attr("").Reset())
		// }

		// text := fmt.Sprintf("%s%s%s", color, s.Text, ColorReset)
		// _, _ = fmt.Fprint(d.writer, "\033[2K")
		// _, _ = fmt.Fprintf(
		// 	d.writer,
		// 	"%s %s\t%s\n",
		// 	icon,
		// 	s.Name,
		// 	text,
		// )
		// Update icon function for this service
		// templateFuncs["spinner"] = func() string {
		// 	return icon
		// }

		rendered, err := d.renderServiceWithColumns(s)
		if err != nil {
			_, _ = fmt.Fprintf(d.writer, "Error rendering: %v", err)
		} else {
			_, _ = fmt.Fprint(d.writer, rendered)
		}

		_, _ = fmt.Fprintln(d.writer)
		linesThisFrame++

		// detail lines (indented; no spinner)
		for _, line := range s.Details {
			// key := fmt.Sprintf("%s%s%s", FgGrey245, line.Key, ColorReset)
			// val := fmt.Sprintf("%s%s%s", FgGrey245, line.Value, ColorReset)
			key := fmt.Sprintf("%s%s%s", "", line.Key, "")
			val := fmt.Sprintf("%s%s%s", "", line.Value, "")
			_, _ = fmt.Fprint(d.writer, "\033[2K")
			_, _ = fmt.Fprintf(
				d.writer,
				"  %s:\t%s\n",
				key,
				val,
			)
			linesThisFrame++
		}
	}

	d.flushFunc()
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
	"padLeft":         padLeft,
	"padRight":        padRight,
	"FgBlack":         func(s string) string { return fg(s).FgBlack() },
	"FgRed":           func(s string) string { return fg(s).FgRed() },
	"FgGreen":         func(s string) string { return fg(s).FgGreen() },
	"FgYellow":        func(s string) string { return fg(s).FgYellow() },
	"FgBlue":          func(s string) string { return fg(s).FgBlue() },
	"FgMagenta":       func(s string) string { return fg(s).FgMagenta() },
	"FgCyan":          func(s string) string { return fg(s).FgCyan() },
	"FgWhite":         func(s string) string { return fg(s).FgWhite() },
	"FgHiBlack":       func(s string) string { return fg(s).FgHiBlack() },
	"FgHiRed":         func(s string) string { return fg(s).FgHiRed() },
	"FgHiGreen":       func(s string) string { return fg(s).FgHiGreen() },
	"FgHiYellow":      func(s string) string { return fg(s).FgHiYellow() },
	"FgHiBlue":        func(s string) string { return fg(s).FgHiBlue() },
	"FgHiMagenta":     func(s string) string { return fg(s).FgHiMagenta() },
	"FgHiCyan":        func(s string) string { return fg(s).FgHiCyan() },
	"FgHiWhite":       func(s string) string { return fg(s).FgHiWhite() },
	"BgBlack":         func(s string) string { return bg(s).BgBlack() },
	"BgRed":           func(s string) string { return bg(s).BgRed() },
	"BgGreen":         func(s string) string { return bg(s).BgGreen() },
	"BgYellow":        func(s string) string { return bg(s).BgYellow() },
	"BgBlue":          func(s string) string { return bg(s).BgBlue() },
	"BgMagenta":       func(s string) string { return bg(s).BgMagenta() },
	"BgCyan":          func(s string) string { return bg(s).BgCyan() },
	"BgWhite":         func(s string) string { return bg(s).BgWhite() },
	"BgHiBlack":       func(s string) string { return bg(s).BgHiBlack() },
	"BgHiRed":         func(s string) string { return bg(s).BgHiRed() },
	"BgHiGreen":       func(s string) string { return bg(s).BgHiGreen() },
	"BgHiYellow":      func(s string) string { return bg(s).BgHiYellow() },
	"BgHiBlue":        func(s string) string { return bg(s).BgHiBlue() },
	"BgHiMagenta":     func(s string) string { return bg(s).BgHiMagenta() },
	"BgHiCyan":        func(s string) string { return bg(s).BgHiCyan() },
	"BgHiWhite":       func(s string) string { return bg(s).BgHiWhite() },
	"Reset":           func(s string) string { return attr(s).Reset() },
	"Bold":            func(s string) string { return attr(s).Bold() },
	"Faint":           func(s string) string { return attr(s).Faint() },
	"Italic":          func(s string) string { return attr(s).Italic() },
	"Underline":       func(s string) string { return attr(s).Underline() },
	"BlinkSlow":       func(s string) string { return attr(s).BlinkSlow() },
	"BlinkRapid":      func(s string) string { return attr(s).BlinkRapid() },
	"ReverseVideo":    func(s string) string { return attr(s).ReverseVideo() },
	"Concealed":       func(s string) string { return attr(s).Concealed() },
	"CrossedOut":      func(s string) string { return attr(s).CrossedOut() },
	"ResetBold":       func(s string) string { return attr(s).ResetBold() },
	"ResetItalic":     func(s string) string { return attr(s).ResetItalic() },
	"ResetUnderline":  func(s string) string { return attr(s).ResetUnderline() },
	"ResetBlinking":   func(s string) string { return attr(s).ResetBlinking() },
	"ResetReversed":   func(s string) string { return attr(s).ResetReversed() },
	"ResetConcealed":  func(s string) string { return attr(s).ResetConcealed() },
	"ResetCrossedOut": func(s string) string { return attr(s).ResetCrossedOut() },
}

// NewDashboard creates the dashboard with one ServiceState per name.
func NewDashboard(names []string, opts ...Option) *Dashboard {
	svcs := make([]*ServiceState, len(names))
	for i, n := range names {
		svcs[i] = &ServiceState{
			Name:        n,
			Text:        "",
			Metadata:    make(map[string]any),
			failedIcon:  "✖",
			successIcon: "✔",
		}
	}

	d := &Dashboard{
		Name:        "Name",
		services:    svcs,
		done:        make(chan struct{}),
		writer:      os.Stdout,
		flushFunc:   func() {},
		formatStr:   "",
		hasHeader:   false,
		maxServices: 0,
		emptyText:   "Waiting",
	}

	for _, opt := range opts {
		opt(d)
	}

	// Parse service columns template
	columns, useColumns := parseColumns(d.formatStr)
	if useColumns {
		d.columns = columns
		d.useColumns = true

		for i := range d.columns {
			tmpl, err := template.New(fmt.Sprintf("col_%d", i)).Funcs(templateFuncs).Parse(d.columns[i].Template)
			if err != nil {
				fmt.Printf("error parsing column %d template: %v\n", i, err)
				d.useColumns = false
				break
			}
			d.columns[i].Parsed = tmpl
		}
	}

	// Parse header column template
	headerCol, useHeaderCols := parseColumns(d.headerStr)

	if useHeaderCols {
		d.headerCols = headerCol
		d.useColumns = true

		for i := range d.headerCols {
			headerTmpl, err := template.New("header").Funcs(templateFuncs).Parse(d.headerCols[i].Template)
			if err != nil {
				fmt.Printf("error parsing header template: %v\n", err)
				d.useColumns = false
				break
			}
			d.headerCols[i].Parsed = headerTmpl
		}
	}
	return d
}
