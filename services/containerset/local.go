package containerset

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/protoutils"
	"github.com/amimof/voiyd/pkg/repository"

	containersetsv1 "github.com/amimof/voiyd/api/services/containersets/v1"
)

type local struct {
	repo     repository.ContainerSetRepository
	mu       sync.Mutex
	exchange *events.Exchange
	logger   logger.Logger
}

var (
	_      containersetsv1.ContainerSetServiceClient = &local{}
	tracer                                           = otel.GetTracerProvider().Tracer("voiyd-server")
)

func (l *local) handleError(err error, msg string, keysAndValues ...any) error {
	def := []any{"error", err.Error()}
	def = append(def, keysAndValues...)
	l.logger.Error(msg, def...)
	if errors.Is(err, repository.ErrNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

func (l *local) Get(ctx context.Context, req *containersetsv1.GetRequest, _ ...grpc.CallOption) (*containersetsv1.GetResponse, error) {
	ctx, span := tracer.Start(ctx, "containerset.Get")
	defer span.End()

	container, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "name", req.GetId())
	}
	return &containersetsv1.GetResponse{
		ContainerSet: container,
	}, nil
}

func (l *local) List(ctx context.Context, req *containersetsv1.ListRequest, _ ...grpc.CallOption) (*containersetsv1.ListResponse, error) {
	ctx, span := tracer.Start(ctx, "containerset.List")
	defer span.End()

	ctrs, err := l.Repo().List(ctx)
	if err != nil {
		return nil, l.handleError(err, "couldn't LIST containers from repo")
	}
	return &containersetsv1.ListResponse{
		ContainerSets: ctrs,
	}, nil
}

func (l *local) Create(ctx context.Context, req *containersetsv1.CreateRequest, _ ...grpc.CallOption) (*containersetsv1.CreateResponse, error) {
	containerSet := req.GetContainerSet()
	containerSetId := containerSet.GetMeta().GetName()

	if existing, _ := l.Repo().Get(ctx, containerSetId); existing != nil {
		return nil, fmt.Errorf("container set %s already exists", containerSet.GetMeta().GetName())
	}

	containerSet.GetMeta().Created = timestamppb.Now()

	err := l.Repo().Create(ctx, containerSet)
	if err != nil {
		return nil, l.handleError(err, "couldn't CREATE container in repo", "name", containerSet.GetMeta().GetName())
	}

	err = l.exchange.Publish(ctx, events.ContainerSetCreate, events.NewEvent(events.ContainerSetCreate, containerSet))
	if err != nil {
		return nil, l.handleError(err, "error publishing CREATE event", "name", containerSet.GetMeta().GetName(), "event", "ContainerCreate")
	}
	return &containersetsv1.CreateResponse{
		ContainerSet: containerSet,
	}, nil
}

func (l *local) Delete(ctx context.Context, req *containersetsv1.DeleteRequest, _ ...grpc.CallOption) (*containersetsv1.DeleteResponse, error) {
	ctx, span := tracer.Start(ctx, "containerset.Delete")
	defer span.End()

	containerSet, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "id", req.GetId())
	}
	containerSetId := containerSet.GetMeta().GetName()
	err = l.Repo().Delete(ctx, containerSetId)
	if err != nil {
		return nil, err
	}
	err = l.exchange.Publish(ctx, events.ContainerSetDelete, events.NewEvent(events.ContainerSetDelete, containerSet))
	if err != nil {
		return nil, l.handleError(err, "error publishing DELETE event", "name", containerSet.GetMeta().GetName(), "event", "ContainerDelete")
	}
	return &containersetsv1.DeleteResponse{
		Id: req.GetId(),
	}, nil
}

func (l *local) Update(ctx context.Context, req *containersetsv1.UpdateRequest, _ ...grpc.CallOption) (*containersetsv1.UpdateResponse, error) {
	ctx, span := tracer.Start(ctx, "containerset.Update")
	defer span.End()

	updateMask := req.GetUpdateMask()
	updateContainerSet := req.GetContainerSet()
	existing, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET container from repo", "name", updateContainerSet.GetMeta().GetName())
	}

	maskedUpdate, err := protoutils.ApplyFieldMaskToNewMessage(updateContainerSet, updateMask)
	if err != nil {
		return nil, err
	}
	proto.Merge(existing, maskedUpdate)

	existing.GetMeta().Updated = timestamppb.Now()

	err = l.Repo().Update(ctx, existing)
	if err != nil {
		return nil, l.handleError(err, "couldn't UPDATE container in repo", "name", existing.GetMeta().GetName())
	}

	containerSet, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	err = l.exchange.Publish(ctx, events.ContainerSetUpdate, events.NewEvent(events.ContainerSetUpdate, containerSet))
	if err != nil {
		return nil, l.handleError(err, "error publishing UPDATE event", "name", existing.GetMeta().GetName(), "event", "ContainerUpdate")
	}
	return &containersetsv1.UpdateResponse{
		ContainerSet: existing,
	}, nil
}

func (l *local) Repo() repository.ContainerSetRepository {
	if l.repo != nil {
		return l.repo
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return repository.NewContainerSetInMemRepo()
}
