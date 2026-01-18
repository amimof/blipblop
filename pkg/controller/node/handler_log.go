package nodecontroller

import (
	"bufio"
	"context"
	"time"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	logsv1 "github.com/amimof/voiyd/api/services/logs/v1"
	"github.com/amimof/voiyd/pkg/events"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (c *Controller) onLogStart(ctx context.Context, obj *eventsv1.Event) error {
	s := &logsv1.TailLogRequest{}
	err := obj.GetObject().UnmarshalTo(s)
	if err != nil {
		return err
	}
	c.logger.Debug("someone requested logs", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId(), "sessionID", s.GetSessionId())

	if s.GetNodeId() != c.node.GetMeta().GetName() {
		return nil
	}

	streamKey := events.LogKey{
		NodeID:    s.GetNodeId(),
		TaskID:    s.GetTaskId(),
		SessionID: s.GetSessionId(),
	}

	c.logStreamsMu.Lock()
	if _, exists := c.activeLogStreams[streamKey]; exists {
		c.logStreamsMu.Unlock()
		c.logger.Debug("log stream already active", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId(), "sessionID", s.GetSessionId())
		return nil
	}
	c.logStreamsMu.Unlock()

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

	go func() {
		c.logger.Info("starting log scanner goroutine", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())

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
		scanner := bufio.NewScanner(taskIO.Stdout)

		// Goroutine that scans the log file and sends each line on the channel
		go func() {
			defer close(lines)

			for {

				// Exit early on cancel even when at EOF
				select {
				case <-streamCtx.Done():
					c.logger.Debug("exiting log streaming because context was cancelled", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
					return
				default:
				}

				// Scan log file and send each line through the channel
				if scanner.Scan() {
					line := scanner.Text()
					select {
					case <-streamCtx.Done():
						c.logger.Debug("exiting log streaming because context was cancelled", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
						return
					case lines <- line: // Send line through
					}
					continue
				}

				// Scanner returned false. Check for errors and exit out if any
				if err := scanner.Err(); err != nil {
					c.logger.Error(
						"error reading from stdout",
						"error", err,
						"nodeID", s.GetNodeId(),
						"taskID", s.GetTaskId(),
					)
					return
				}

				// No errors, maybe EOF?
				select {
				case <-streamCtx.Done():
					c.logger.Debug("exiting log streaming because context was cancelled", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
					return
				case <-time.After(300 * time.Millisecond): // Wait a bit before iterating again
				}

				// EOF reached, ovewrite the scanner to start reading again
				scanner = bufio.NewScanner(taskIO.Stdout)
			}
		}()

		// Count the number of lines read
		var seq uint64

		// Send log entry for each line that comes in from the line channel. After x amount of time anf if no lines are
		// received, exit out. This is a blocking operation.
		for {
			select {
			case <-streamCtx.Done():
				c.logger.Debug("log stream cancelled", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
				return
			case <-time.After(5 * time.Minute):
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

func (c *Controller) onLogStop(_ context.Context, obj *eventsv1.Event) error {
	s := &logsv1.TailLogRequest{}
	err := obj.GetObject().UnmarshalTo(s)
	if err != nil {
		return err
	}
	c.logger.Debug("someone requested stop logs", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())

	if s.GetNodeId() != c.node.GetMeta().GetName() {
		return nil
	}

	streamKey := events.LogKey{
		NodeID:    s.GetNodeId(),
		TaskID:    s.GetTaskId(),
		SessionID: s.GetSessionId(),
	}

	c.logStreamsMu.Lock()
	cancel, exists := c.activeLogStreams[streamKey]
	c.logStreamsMu.Unlock()

	if exists {
		cancel()
		c.logger.Debug("cancelled log stream", "nodeID", s.GetNodeId(), "taskID", s.GetTaskId())
	}

	return nil
}
