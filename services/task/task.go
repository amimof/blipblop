// Package task provides the server implemenetation for the task service
package task

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/repository"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

const Version string = "task/v1"

type NewServiceOption func(s *TaskService)

func WithLogger(l logger.Logger) NewServiceOption {
	return func(s *TaskService) {
		s.logger = l
	}
}

func WithExchange(e *events.Exchange) NewServiceOption {
	return func(s *TaskService) {
		s.exchange = e
	}
}

type TaskService struct {
	tasksv1.UnimplementedTaskServiceServer
	local    tasksv1.TaskServiceClient
	logger   logger.Logger
	exchange *events.Exchange
}

func (c *TaskService) Register(server *grpc.Server) error {
	// server.RegisterService(&containers.TaskService_ServiceDesc, c)
	tasksv1.RegisterTaskServiceServer(server, c)
	return nil
}

func (c *TaskService) Get(ctx context.Context, req *tasksv1.GetRequest) (*tasksv1.GetResponse, error) {
	return c.local.Get(ctx, req)
}

func (c *TaskService) List(ctx context.Context, req *tasksv1.ListRequest) (*tasksv1.ListResponse, error) {
	return c.local.List(ctx, req)
}

func (c *TaskService) Create(ctx context.Context, req *tasksv1.CreateRequest) (*tasksv1.CreateResponse, error) {
	return c.local.Create(ctx, req)
}

func (c *TaskService) Delete(ctx context.Context, req *tasksv1.DeleteRequest) (*tasksv1.DeleteResponse, error) {
	return c.local.Delete(ctx, req)
}

func (c *TaskService) Kill(ctx context.Context, req *tasksv1.KillRequest) (*tasksv1.KillResponse, error) {
	return c.local.Kill(ctx, req)
}

func (c *TaskService) Start(ctx context.Context, req *tasksv1.StartRequest) (*tasksv1.StartResponse, error) {
	return c.local.Start(ctx, req)
}

func (c *TaskService) Update(ctx context.Context, req *tasksv1.UpdateRequest) (*tasksv1.UpdateResponse, error) {
	return c.local.Update(ctx, req)
}

func (c *TaskService) Patch(ctx context.Context, req *tasksv1.PatchRequest) (*tasksv1.PatchResponse, error) {
	return c.local.Patch(ctx, req)
}

func (c *TaskService) UpdateStatus(ctx context.Context, req *tasksv1.UpdateStatusRequest) (*tasksv1.UpdateStatusResponse, error) {
	return c.local.UpdateStatus(ctx, req)
}

func (c *TaskService) Condition(ctx context.Context, req *typesv1.ConditionRequest) (*emptypb.Empty, error) {
	return c.local.Condition(ctx, req)
}

func NewService(repo repository.TaskRepository, opts ...NewServiceOption) *TaskService {
	s := &TaskService{
		logger: logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(s)
	}

	s.local = &local{
		repo:     repo,
		exchange: s.exchange,
		logger:   s.logger,
	}

	return s
}
