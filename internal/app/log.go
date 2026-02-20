// Package log implements the log service
package app

import (
	"context"

	logsv1 "github.com/amimof/voiyd/api/services/logs/v1"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
)

type LogService struct {
	Logger      logger.Logger
	Exchange    *events.Exchange
	LogExchange *events.LogExchange
	Manager     SessionManager
}

func (s *LogService) SendStartLogsCommand(ctx context.Context, req *logsv1.TailLogRequest) error {
	err := s.Exchange.Publish(ctx, events.NewEvent(events.TailLogsStart, req))
	if err != nil {
		s.Logger.Error("error publishing TailLogStart event", "nodeID", req.GetNodeId(), "containerID", req.GetTaskId(), "tail?", req.GetWatch())
		return err
	}
	return nil
}

func (s *LogService) SendStopLogsCommand(ctx context.Context, req *logsv1.TailLogRequest) error {
	err := s.Exchange.Publish(ctx, events.NewEvent(events.TailLogsStop, req))
	if err != nil {
		s.Logger.Error("error publishing TailLogStart event", "nodeID", req.GetNodeId(), "containerID", req.GetTaskId(), "tail?", req.GetWatch())
		return err
	}
	return nil
}
