// Package cmdutil provides helper utilities and interfaces for working with command line tools
package cmdutil

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/fatih/color"
)

type Spinner struct {
	stop   bool
	Prefix *string
	Suffix string
}

type NewSpinnerOpt func(*Spinner)

func NewSpinner(opts ...NewSpinnerOpt) *Spinner {
	s := &Spinner{
		stop: false,
	}

	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithPrefix(str *string) NewSpinnerOpt {
	return func(s *Spinner) {
		s.Prefix = str
	}
}

func (s *Spinner) Start() {
	chars := []rune{'⣾', '⣽', '⣻', '⢿', '⡿', '⣟', '⣯', '⣷'}
	i := 0
	_, _ = fmt.Fprint(os.Stdout, "\033[?25l")
	go func() {
		for {
			fmt.Printf("%s %c\r", *s.Prefix, chars[i%len(chars)])
			time.Sleep(130 * time.Millisecond)
			i = i + 1
			if s.stop {
				return
			}
		}
	}()
}

func (s *Spinner) Stop() {
	s.stop = true
	_, _ = fmt.Fprint(os.Stdout, "\033[?25h")
}

func FormatPhase(phase string) string {
	p := strings.ToUpper(phase)
	switch p {
	case "UNKNOWN":
		return color.YellowString(p)
	case "RUNNING":
		return color.GreenString(p)
	case "ERROR":
		return color.RedString(p)
	default:
		return color.CyanString(p)
	}
}

type (
	StopFunc  func()
	WatchFunc func(StopFunc) error
)

func Watch(ctx context.Context, id string, wf WatchFunc) error {
	s := false
	stop := func() {
		s = true
	}
	for {
		err := wf(stop)
		if err != nil {
			return err
		}
		if s {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	return nil
}

func FormatDuration(d time.Duration) string {
	// Convert the duration to whole days, hours, minutes, and seconds
	days := d / (24 * time.Hour)
	d -= days * 24 * time.Hour
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second

	if days > 0 {
		return fmt.Sprintf("%dd", days)
	} else if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

type OutputFormat string

var (
	OutputFormatJSON  OutputFormat = "json"
	OutputFormatYAML  OutputFormat = "yaml"
	OutputFormatTable OutputFormat = "table"
)

func validateOutputFormat(s string) (OutputFormat, error) {
	allowedOutputs := []OutputFormat{
		OutputFormatJSON,
		OutputFormatYAML,
		OutputFormatTable,
	}

	o := strings.ToLower(s)
	if ok := slices.Contains(allowedOutputs, OutputFormat(o)); !ok {
		return "", fmt.Errorf("expected output to be one of %v", allowedOutputs)
	}

	return OutputFormat(o), nil
}

func SerializerFor(s string) (Serializer, error) {
	o, err := validateOutputFormat(s)
	if err != nil {
		return nil, err
	}

	switch o {
	case OutputFormatJSON:
		return &JSONSerializer{}, nil
	case OutputFormatYAML:
		return &YamlSerializer{}, nil
	case OutputFormatTable:
		return &TableSerializer{}, nil
	default:
		return &JSONSerializer{}, nil
	}
}

func DeserializerFor(s string) (Deserializer, error) {
	o, err := validateOutputFormat(s)
	if err != nil {
		return nil, err
	}

	switch o {
	case OutputFormatJSON:
		return &JSONDeserializer{}, nil
	case OutputFormatYAML:
		return &YamlDeserializer{}, nil
	default:
		return &JSONDeserializer{}, nil
	}
}

func CodecFor(s string) (Codec, error) {
	o, err := validateOutputFormat(s)
	if err != nil {
		return nil, err
	}

	switch o {
	case OutputFormatJSON:
		return NewJSONCodec(), nil
	case OutputFormatYAML:
		return NewYamlCodec(), nil
	default:
		return NewJSONCodec(), nil
	}
}
