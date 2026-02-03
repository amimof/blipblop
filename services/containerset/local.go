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
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/repository"

	containersetsv1 "github.com/amimof/voiyd/api/services/containersets/v1"
)

type local struct {
	repo     *repository.Repo[*containersetsv1.ContainerSet]
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

	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, err
	}

	containerSet, err := l.repo.Get(ctx, uid)
	if err != nil {
		return nil, l.handleError(err, "error getting containerset", "name", containerSet.GetMeta().GetName())
	}

	return &containersetsv1.GetResponse{
		ContainerSet: containerSet,
	}, nil
}

func (l *local) List(ctx context.Context, req *containersetsv1.ListRequest, _ ...grpc.CallOption) (*containersetsv1.ListResponse, error) {
	ctx, span := tracer.Start(ctx, "containerset.List")
	defer span.End()

	sets, err := l.repo.List(ctx, int(req.GetLimit()))
	if err != nil {
		return nil, l.handleError(err, "couldn't LIST containers from repo")
	}

	return &containersetsv1.ListResponse{
		ContainerSets: sets,
	}, nil
}

func (l *local) Create(ctx context.Context, req *containersetsv1.CreateRequest, _ ...grpc.CallOption) (*containersetsv1.CreateResponse, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	containerSet := req.GetContainerSet()
	containerSetId := containerSet.GetMeta().GetName()

	if existing, _ := l.Get(ctx, &containersetsv1.GetRequest{Name: containerSetId}); existing != nil {
		return nil, fmt.Errorf("containerset %s already exists", containerSet.GetMeta().GetName())
	}

	containerSet.GetMeta().ResourceVersion = 1
	containerSet.GetMeta().Generation = 1

	newSet, err := l.repo.Create(ctx, containerSet)
	if err != nil {
		return nil, l.handleError(err, "error creating containerset", "name", newSet.GetMeta().GetName())
	}

	err = l.exchange.Publish(ctx, events.NewEvent(events.ContainerSetCreate, newSet))
	if err != nil {
		return nil, l.handleError(err, "error publishing containerset create event", "name", newSet.GetMeta().GetName(), "event", "ContainerCreate")
	}
	return &containersetsv1.CreateResponse{
		ContainerSet: newSet,
	}, nil
}

func (l *local) Delete(ctx context.Context, req *containersetsv1.DeleteRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	ctx, span := tracer.Start(ctx, "containerset.Delete")
	defer span.End()

	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, err
	}

	containerSet, err := l.repo.Get(ctx, uid)
	if err != nil {
		return nil, l.handleError(err, "error getting containerset", "id", containerSet.GetMeta().GetName())
	}

	err = l.repo.Delete(ctx, uid)
	if err != nil {
		return nil, err
	}
	err = l.exchange.Publish(ctx, events.NewEvent(events.ContainerSetDelete, containerSet))
	if err != nil {
		return nil, l.handleError(err, "error publishing containerset delete event", "name", containerSet.GetMeta().GetName(), "event", "ContainerDelete")
	}

	return &emptypb.Empty{}, nil
}

func (l *local) Update(ctx context.Context, req *containersetsv1.UpdateRequest, _ ...grpc.CallOption) (*containersetsv1.UpdateResponse, error) {
	ctx, span := tracer.Start(ctx, "containerset.Update")
	defer span.End()

	updateContainerSet := req.GetContainerSet()

	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, err
	}

	existing, err := l.repo.Get(ctx, uid)
	if err != nil {
		return nil, l.handleError(err, "error getting containerset", "name", updateContainerSet.GetMeta().GetName())
	}

	// Ignore fields
	updateContainerSet.GetMeta().ResourceVersion++
	updateContainerSet.GetMeta().Updated = existing.Meta.Updated
	updateContainerSet.GetMeta().Created = existing.Meta.Created

	updVal := protoreflect.ValueOfMessage(updateContainerSet.ProtoReflect())
	newVal := protoreflect.ValueOfMessage(existing.ProtoReflect())

	// Only update metadata fields if spec is updated
	if !updVal.Equal(newVal) {
		updateContainerSet.Meta.Generation++
	}

	// Update the set
	err = l.repo.Update(ctx, uid, updateContainerSet)
	if err != nil {
		return nil, l.handleError(err, "error updating containerset", "name", updateContainerSet.GetMeta().GetName())
	}

	// Retreive the set again so that we can include it in an event
	containerSet, err := l.repo.Get(ctx, uid)
	if err != nil {
		return nil, err
	}

	// Only publish if spec is updated
	if !updVal.Equal(newVal) {

		// Decorate label with some labels
		eventLabels := labels.New()
		eventLabels.Set(labels.LabelPrefix("object-id").String(), containerSet.GetMeta().GetName())
		eventLabels.Set(labels.LabelPrefix("object-version").String(), containerSet.GetVersion())

		err = l.exchange.Forward(ctx, events.NewEvent(events.TaskUpdate, containerSet, eventLabels))
		if err != nil {
			return nil, l.handleError(err, "error publishing containerset update event", "name", containerSet.GetMeta().GetName(), "event", "TaskUpdate")
		}
	}

	return &containersetsv1.UpdateResponse{
		ContainerSet: containerSet,
	}, nil
}
