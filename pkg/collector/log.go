package collector

import (
	"time"

	logsv1 "github.com/amimof/blipblop/api/services/logs/v1"
)

// type Entry logsv1.LogStreamRequest

type LogCollector interface {
	// Start collecting logs from the specified entry
	Start(chan *logsv1.LogStreamRequest) error

	// Stop collecting logs
	Stop() error

	// Logs retrieves a channel of collected logs for processing
	Logs() <-chan *logsv1.LogStreamRequest

	// SetFilter sets a filter function to process only matching logs
	SetFilter(filterFunc func(*logsv1.LogStreamRequest) bool)

	// SetParser sets a parser for log entry transformation
	SetParser(parserFunc func(rawLog string) (*logsv1.LogStreamRequest, bool))
}

type CioCollector struct {
	stop        chan struct{}
	NodeId      string
	ContainerId string
}

func (c *CioCollector) cleanup() {
}

// Logs implements LogCollector.
func (c *CioCollector) Logs() <-chan *logsv1.LogStreamRequest {
	panic("unimplemented")
}

// SetFilter implements LogCollector.
func (c *CioCollector) SetFilter(filterFunc func(*logsv1.LogStreamRequest) bool) {
	panic("unimplemented")
}

// SetParser implements LogCollector.
func (c *CioCollector) SetParser(parserFunc func(rawLog string) (*logsv1.LogStreamRequest, bool)) {
	panic("unimplemented")
}

// Start implements LogCollector.
func (c *CioCollector) Start(in chan *logsv1.LogStreamRequest) error {
	c.stop = make(chan struct{})

	for {
		select {
		case <-c.stop:
			c.cleanup()
			return nil
		default:
			req := &logsv1.LogStreamRequest{
				NodeId:      c.NodeId,
				ContainerId: c.ContainerId,
				Log: &logsv1.LogItem{
					LogLine:   "%d hello world!",
					Timestamp: time.Now().String(),
				},
			}
			in <- req
			time.Sleep(time.Second * 1)
		}
	}
}

// Stop implements LogCollector.
func (c *CioCollector) Stop() error {
	close(c.stop)
	return nil
}

func NewCioCollector(containerId string) LogCollector {
	return &CioCollector{
		ContainerId: containerId,
		stop:        make(chan struct{}),
	}
}
