package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
	"github.com/amimof/voiyd/internal/app"
	"github.com/amimof/voiyd/pkg/keys"
)

type TaskService struct {
	tasksv1.UnimplementedTaskServiceServer
	app *app.TaskService
}

func (c *TaskService) Register(server *grpc.Server) {
	tasksv1.RegisterTaskServiceServer(server, c)
}

func (c *TaskService) Get(ctx context.Context, req *tasksv1.GetRequest) (*tasksv1.GetResponse, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}
	task, err := c.app.Get(ctx, uid)
	if err != nil {
		return nil, toStatus(err)
	}
	return &tasksv1.GetResponse{Task: task}, nil
}

func (c *TaskService) Create(ctx context.Context, req *tasksv1.CreateRequest) (*tasksv1.CreateResponse, error) {
	task, err := c.app.Create(ctx, req.GetTask())
	if err != nil {
		return nil, toStatus(err)
	}
	return &tasksv1.CreateResponse{Task: task}, nil
}

func (c *TaskService) Delete(ctx context.Context, req *tasksv1.DeleteRequest) (*emptypb.Empty, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}

	err = c.app.Delete(ctx, uid)
	if err != nil {
		return nil, toStatus(err)
	}

	return &emptypb.Empty{}, nil
}

func (c *TaskService) List(ctx context.Context, req *tasksv1.ListRequest) (*tasksv1.ListResponse, error) {
	tasks, err := c.app.List(ctx, req.GetLimit())
	if err != nil {
		return nil, toStatus(err)
	}
	return &tasksv1.ListResponse{Tasks: tasks}, nil
}

func (c *TaskService) Update(ctx context.Context, req *tasksv1.UpdateRequest) (*tasksv1.UpdateResponse, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}

	err = c.app.Update(ctx, uid, req.GetTask())
	if err != nil {
		return nil, toStatus(err)
	}

	task, err := c.app.Get(ctx, uid)
	if err != nil {
		return nil, toStatus(err)
	}

	return &tasksv1.UpdateResponse{Task: task}, nil
}

func (c *TaskService) Patch(ctx context.Context, req *tasksv1.PatchRequest) (*tasksv1.PatchResponse, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}

	err = c.app.Patch(ctx, uid, req.GetTask())
	if err != nil {
		return nil, toStatus(err)
	}

	task, err := c.app.Get(ctx, uid)
	if err != nil {
		return nil, toStatus(err)
	}

	return &tasksv1.PatchResponse{Task: task}, nil
}

func (c *TaskService) UpdateStatus(ctx context.Context, req *tasksv1.UpdateStatusRequest) (*tasksv1.UpdateStatusResponse, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}

	err = c.app.UpdateStatus(ctx, uid, req.GetStatus(), req.GetUpdateMask().GetPaths()...)
	if err != nil {
		return nil, toStatus(err)
	}

	return &tasksv1.UpdateStatusResponse{Id: uid.UUIDStr()}, nil
}

func (c *TaskService) Condition(ctx context.Context, req *typesv1.ConditionRequest) (*emptypb.Empty, error) {
	uid, err := keys.ParseStr(req.GetTaskId())
	if err != nil {
		return nil, toStatus(err)
	}
	err = c.app.Condition(ctx, uid, req)
	if err != nil {
		return nil, toStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (c *TaskService) Kill(ctx context.Context, req *tasksv1.KillRequest) (*emptypb.Empty, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}
	if req.GetForceKill() {
		err = c.app.Kill(ctx, uid)
		if err != nil {
			return nil, toStatus(err)
		}
	} else {
		err = c.app.Stop(ctx, uid)
		if err != nil {
			return nil, toStatus(err)
		}
	}
	return &emptypb.Empty{}, toStatus(err)
}

func (c *TaskService) Start(ctx context.Context, req *tasksv1.StartRequest) (*emptypb.Empty, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}
	err = c.app.Start(ctx, uid)
	if err != nil {
		return nil, toStatus(err)
	}
	return &emptypb.Empty{}, toStatus(err)
}

func (c *TaskService) Stop(ctx context.Context, req *tasksv1.StartRequest) (*emptypb.Empty, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, toStatus(err)
	}
	err = c.app.Stop(ctx, uid)
	if err != nil {
		return nil, toStatus(err)
	}
	return &emptypb.Empty{}, toStatus(err)
}

func NewTaskService(app *app.TaskService) *TaskService {
	return &TaskService{app: app}
}
