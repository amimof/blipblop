package controller

import (
	"context"

	logsv1 "github.com/amimof/blipblop/api/services/logs/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/amimof/blipblop/pkg/collector"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/runtime"
	"github.com/google/uuid"
)

type LogController struct {
	runtime    runtime.Runtime
	logger     logger.Logger
	clientset  *client.ClientSet
	collectors map[string]collector.LogCollector
	nodeName   string
}

type NewLogControllerOption func(c *LogController)

func WithLogControllerNodeName(nodeName string) NewLogControllerOption {
	return func(c *LogController) {
		c.nodeName = nodeName
	}
}

func (c *LogController) getOrCreateCollectorForContainer(containerId string) collector.LogCollector {
	_, ok := c.collectors[containerId]
	if !ok {
		c.collectors[containerId] = collector.NewCioCollector(containerId)
	}
	return c.collectors[containerId]
}

func (c *LogController) Run(ctx context.Context) {
	// Setup channels
	reqChan := make(chan *logsv1.LogStreamRequest, 10)
	resChan := make(chan *logsv1.LogStreamResponse, 1)
	errChan := make(chan error, 1)

	// Control logging flow using responses from the server.
	// The server might ask us to send logs for the same container (exact same log)
	// since multiple clients should be able to stream logs simultaneously.
	// Below goroutine makes sure to only send logs once per container to the server.
	// It is then up to the server to broadcast the logs to it's clients.
	go func() {
		for e := range resChan {

			// Create a collector and start logging only if there are no collectors already
			// present for the specific container
			if _, ok := c.collectors[e.GetContainerId()]; !ok {
				col := c.getOrCreateCollectorForContainer(e.GetContainerId())
				go col.Start(reqChan)
			}

			// If we get a signal to stop logging, then tell the collector to stop
			// only if there is a collector present for the specific container
			if !e.GetStart() {
				col := c.getOrCreateCollectorForContainer(e.GetContainerId())
				col.Stop()
				delete(c.collectors, e.GetContainerId())
			}

		}
	}()

	// Connect to log service
	go func() {
		err := c.clientset.LogV1().LogStream(ctx, c.nodeName, "nginx2", reqChan, errChan, resChan)
		if err != nil {
			c.logger.Error("error connecting to log collector service", "error", err)
		}
	}()

	// Handle messages
	for {
		select {
		case err := <-errChan:
			c.logger.Error("received stream error", "error", err)
		case <-ctx.Done():
			c.logger.Info("context canceled, shutting down controller")
			return
		}
	}
}

// func (c *LogController) collectLogs(reqChan chan *logsv1.LogStreamRequest, res *logsv1.LogStreamResponse) {
// 	var i int
// 	for res.GetStart() {
// 		if !res.GetStart() {
// 			log.Println("Exiting, no logs")
// 		}
// 		req := &logsv1.LogStreamRequest{
// 			NodeId:      res.GetNodeId(),
// 			ContainerId: res.GetContainerId(),
// 			Log: &logsv1.LogItem{
// 				LogLine:   fmt.Sprintf("%d hello world!", i),
// 				Timestamp: time.Now().String(),
// 			},
// 		}
// 		reqChan <- req
// 		log.Println("Log collected!", res.GetStart())
// 		time.Sleep(time.Second * 1)
// 		i = i + 1
// 	}
// 	log.Println("Done collecting ")
// }

func NewLogController(cs *client.ClientSet, rt runtime.Runtime, opts ...NewLogControllerOption) (*LogController, error) {
	c := &LogController{
		clientset:  cs,
		runtime:    rt,
		logger:     logger.ConsoleLogger{},
		nodeName:   uuid.New().String(),
		collectors: make(map[string]collector.LogCollector),
	}
	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}
