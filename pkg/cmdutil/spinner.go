package cmdutil

import (
	"fmt"
	"io"
	"os"
	"time"
)

type Spinner struct {
	prefix  string
	text    string
	updates chan formatText
	stop    chan struct{}
	done    chan struct{}
	writer  io.Writer
}

func NewSpinner(opts ...NewSpinnerOpt) *Spinner {
	s := &Spinner{
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		updates: make(chan formatText, 10),
		writer:  os.Stdout,
		prefix:  "",
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func WithWriter(w io.Writer) NewSpinnerOpt {
	return func(s *Spinner) {
		s.writer = &SyncWriter{w: w}
	}
}

func WithPrefix(str string) NewSpinnerOpt {
	return func(s *Spinner) {
		s.prefix = str
	}
}

type formatText struct {
	text string
}

type NewSpinnerOpt func(*Spinner)

func (s *Spinner) NewLine() {
	_, _ = fmt.Fprintf(s.writer, "\n")
}

func (s *Spinner) loop() {
	defer close(s.done)

	// text := ""
	frames := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
	frameIndex := 0

	ticker := time.NewTicker(130 * time.Millisecond)
	defer ticker.Stop()

	_, _ = fmt.Fprint(s.writer, "\033[?25l")
	for {
		select {
		case <-ticker.C:
			frame := frames[frameIndex]
			frameIndex = (frameIndex + 1) % len(frames)
			_, _ = fmt.Fprintf(s.writer, "%s%s %c\r", s.prefix, s.text, frame)
		// case msg := <-s.updates:
		// 	if msg.text != "" {
		// 		text = msg.text
		// 	}
		case <-s.stop:
			// clear line and restore cursor
			_, _ = fmt.Fprint(s.writer, "\r\033[K")
			_, _ = fmt.Fprint(s.writer, "\033[?25h")
			return
		}
	}
}

func (s *Spinner) Update(text string) {
	s.text = text
	// select {
	// case s.updates <- formatText{text: text}:
	// default:
	// }
}

func (s *Spinner) Stop() {
	close(s.stop)
	<-s.done
}

func (s *Spinner) StopMsg(finalText string) {
	s.Stop()
	if finalText != "" {
		_, _ = fmt.Fprintf(s.writer, "%s%s\n", s.prefix, finalText)
		return
	}
	_, _ = fmt.Fprintf(s.writer, "\r\n")
}

func (s *Spinner) Start() {
	go s.loop()
}

// func (s *Spinner) Start() {
// 	chars := []rune{'⣾', '⣽', '⣻', '⢿', '⡿', '⣟', '⣯', '⣷'}
// 	i := 0
// 	_, _ = fmt.Fprint(s.writer, "\033[?25l")
// 	go func() {
// 		for {
// 			select {
// 			case <-s.stop:
// 				// clear line and restore cursor
// 				_, _ = fmt.Fprint(s.writer, "\r\033[K")
// 				_, _ = fmt.Fprint(s.writer, "\033[?25h")
// 				return
// 			default:
// 				_, _ = fmt.Fprintf(s.writer, "%s %c\r", *s.Prefix, chars[i%len(chars)])
// 				time.Sleep(130 * time.Millisecond)
// 				i++
// 			}
// 		}
// 	}()
// }

// func (s *Spinner) Stop() {
// 	s.stop <- struct{}{}
// }
//
// func (s *Spinner) StopWithMessage(msg string) {
// 	s.Stop()
// 	_, _ = fmt.Fprintf(s.writer, "%s\n", msg)
// }
