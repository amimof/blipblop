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
	Sender      NodeSender
}

func (s *LogService) SendStartLogsCommand(ctx context.Context, req *logsv1.TailLogRequest) error {
	return s.Sender.SendToNode(ctx, req.GetNodeId(), events.NewEvent(events.TailLogsStart, req))
}

func (s *LogService) SendStopLogsCommand(ctx context.Context, req *logsv1.TailLogRequest) error {
	return s.Sender.SendToNode(ctx, req.GetNodeId(), events.NewEvent(events.TailLogsStop, req))
}
