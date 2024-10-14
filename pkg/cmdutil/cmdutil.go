package cmdutil

import (
	"context"
	"fmt"
	"os"
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
	fmt.Fprint(os.Stdout, "\033[?25l")
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
	fmt.Fprint(os.Stdout, "\033[?25h")
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
