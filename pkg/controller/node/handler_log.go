package nodecontroller

import (
	"bufio"
	"context"
	"io"
	"strings"
	"time"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	logsv1 "github.com/amimof/voiyd/api/services/logs/v1"
	"github.com/amimof/voiyd/pkg/events"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Log stream timeout - how long to wait for new log data before stopping the stream
	logStreamTimeout = 5 * time.Minute

	// Tail scan interval - polling interval when waiting for new log lines
	tailScanInterval = 300 * time.Millisecond
)

func (c *Controller) onLogStart(ctx context.Context, obj *eventsv1.Event) error {
	s := &logsv1.TailLogRequest{}
	err := obj.GetObject().UnmarshalTo(s)
	if err != nil {
		return err
	}

	node, err := c.getNode(ctx)
	if err != nil {
		return err
	}

	c.logger.Debug("someone requested logs", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId(), "sessionID", s.GetSessionId())

	if s.GetNodeId() != node.GetMeta().GetUid() {
		c.logger.Debug("log not for us", "requested", s.GetNodeId(), "our", node.GetMeta().GetUid())
		return nil
	}

	streamKey := events.LogKey{
		NodeID:    s.GetNodeId(),
		TaskID:    s.GetTaskId(),
		SessionID: s.GetSessionId(),
	}

	c.logStreamsMu.RLock()
	if _, exists := c.activeLogStreams[streamKey]; exists {
		c.logStreamsMu.RUnlock()
		c.logger.Debug("log stream already active", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId(), "sessionID", s.GetSessionId())
		return nil
	}
	c.logStreamsMu.RUnlock()

	taskIO, err := c.runtime.IO(ctx, s.GetTaskId())
	if err != nil {
		c.logger.Error("error getting task logs", "error", err)
		return err
	}

	logStream, err := c.clientset.LogV1().Stream(ctx)
	if err != nil {
		c.logger.Error("error setting up pushlogs", "error", err)
		return err
	}

	streamCtx, cancel := context.WithCancel(ctx)

	c.logStreamsMu.Lock()
	c.activeLogStreams[streamKey] = cancel
	c.logStreamsMu.Unlock()

	c.logger.Info("starting log scanner goroutine", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
	go func() {
		defer func() {
			c.logStreamsMu.Lock()
			delete(c.activeLogStreams, streamKey)
			c.logStreamsMu.Unlock()
			cancel()
			_ = logStream.Close()
			if taskIO.Stdout != nil {
				_ = taskIO.Stdout.Close()
			}
		}()

		// Setup scanner. We use a channel to send each line through
		lines := make(chan string)
		defer close(lines)

		// Goroutine that scans the log file and sends each line on the channel
		go c.tail(streamCtx, taskIO.Stdout, lines)

		// Count the number of lines read
		var seq uint64

		// Send log entry for each line that comes in from the line channel. After x amount of time anf if no lines are
		// received, exit out. This is a blocking operation.
		for {
			select {
			case <-streamCtx.Done():
				c.logger.Debug("log stream cancelled", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
				return
			case <-time.After(logStreamTimeout):
				c.logger.Debug("scanner timeout - no data received", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
				return
			case line, ok := <-lines:

				if !ok {
					c.logger.Debug("log stream completed", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
					return
				}

				// Send the line as log entry to the server
				if err := logStream.Send(&logsv1.LogEntry{
					TaskId:    s.GetTaskId(),
					NodeId:    s.GetNodeId(),
					SessionId: s.GetSessionId(),
					Timestamp: timestamppb.Now(),
					Line:      line,
					Seq:       seq,
				}); err != nil {
					c.logger.Error(
						"error pushing log entry",
						"error", err,
						"taskID", s.GetTaskId(),
						"nodeID", s.GetNodeId(),
						"sessionID", s.GetSessionId(),
						"seq", seq,
					)
					return
				}

				// Increase counter
				seq += 1
			}
		}
	}()

	return nil
}

func (c *Controller) tail(ctx context.Context, reader io.Reader, lines chan string) {
	bufReader := bufio.NewReader(reader)

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Read until newline
		line, err := bufReader.ReadString('\n')

		// Send any data we got (even with EOF)
		if len(line) > 0 {
			// Trim the newline
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r") // Handle CRLF too

			select {
			case <-ctx.Done():
				return
			case lines <- line:
			}
		}

		// Handle errors
		if err != nil {
			if err == io.EOF {
				// Reached end of file, wait before checking for more
				select {
				case <-ctx.Done():
					return
				case <-time.After(tailScanInterval):
					// Loop continues, will try reading again
				}
				continue
			}

			// Real error (not EOF)
			c.logger.Error("error reading from log stream", "error", err)
			return
		}
	}
}

func (c *Controller) onLogStop(ctx context.Context, obj *eventsv1.Event) error {
	s := &logsv1.TailLogRequest{}
	err := obj.GetObject().UnmarshalTo(s)
	if err != nil {
		return err
	}

	node, err := c.getNode(ctx)
	if err != nil {
		return err
	}

	c.logger.Debug("someone requested stop logs", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())

	if s.GetNodeId() != node.GetMeta().GetUid() {
		return nil
	}

	streamKey := events.LogKey{
		NodeID:    s.GetNodeId(),
		TaskID:    s.GetTaskId(),
		SessionID: s.GetSessionId(),
	}

	c.logStreamsMu.RLock()
	cancel, exists := c.activeLogStreams[streamKey]
	c.logStreamsMu.RUnlock()

	if exists {
		cancel()
		c.logger.Debug("cancelled log stream", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
	}

	return nil
}
