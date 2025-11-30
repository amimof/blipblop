package volume

import (
	"context"
	"errors"
	"fmt"
	"sync"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/volumes/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"buf.build/go/protovalidate"
)

var (
	_      volumes.VolumeServiceClient = &local{}
	tracer                             = otel.GetTracerProvider().Tracer("volume-service")
)

type local struct {
	repo     repository.VolumeRepository
	mu       sync.Mutex
	exchange *events.Exchange
	logger   logger.Logger
}

func (l *local) handleError(err error, msg string, keysAndValues ...any) error {
	def := []any{"error", err.Error()}
	def = append(def, keysAndValues...)
	l.logger.Error(msg, def...)
	if errors.Is(err, repository.ErrNotFound) {
		return status.Error(codes.NotFound, fmt.Sprintf("%s: %v", msg, err.Error()))
	}
	return status.Error(codes.Internal, fmt.Sprintf("%s: %v", msg, err.Error()))
}

func applyMaskedUpdate(dst, src *volumes.Status, mask *fieldmaskpb.FieldMask) error {
	if mask == nil || len(mask.Paths) == 0 {
		return status.Error(codes.InvalidArgument, "update_mask is required")
	}

	for _, p := range mask.Paths {
		switch p {
		case "controllers":
			if src.Controllers == nil {
				continue
			}
			dst.Controllers = src.Controllers
		default:
			return fmt.Errorf("unknown mask path %q", p)
		}
	}

	return nil
}

// Patch implements volumes.VolumeServiceClient.
func (l *local) Patch(ctx context.Context, in *volumes.PatchRequest, opts ...grpc.CallOption) (*volumes.PatchResponse, error) {
	panic("unimplemented")
}

// Create implements volumes.VolumeServiceClient.
func (l *local) Create(ctx context.Context, req *volumes.CreateRequest, opts ...grpc.CallOption) (*volumes.CreateResponse, error) {
	ctx, span := tracer.Start(ctx, "volume.Create")
	defer span.End()

	volume := req.GetVolume()
	volume.GetMeta().Created = timestamppb.Now()

	// Deprecated: Validate request
	err := req.ValidateAll()
	if err != nil {
		return nil, err
	}

	// NEW: validate with buf
	if err := protovalidate.Validate(req); err != nil {
		return nil, err
	}

	// Initialize status field if empty
	if volume.GetStatus() == nil {
		volume.Status = &volumes.Status{}
	}

	volumeID := volume.GetMeta().GetName()

	// Check if volume already exists
	if existing, _ := l.Get(ctx, &volumes.GetRequest{Id: volumeID}); existing != nil {
		return nil, fmt.Errorf("volume %s already exists", volume.GetMeta().GetName())
	}

	// Create volume in repo
	err = l.Repo().Create(ctx, volume)
	if err != nil {
		return nil, l.handleError(err, "couldn't CREATE volume in repo", "name", volumeID)
	}

	// Get the created volume from repo
	volume, err = l.Repo().Get(ctx, volumeID)
	if err != nil {
		return nil, err
	}

	// Publish event that volume is created
	err = l.exchange.Publish(ctx, eventsv1.EventType_VolumeCreate, events.NewEvent(eventsv1.EventType_VolumeCreate, volume))
	if err != nil {
		return nil, l.handleError(err, "error publishing CREATE event", "name", volume.GetMeta().GetName(), "event", "VolumeCreate")
	}

	return &volumes.CreateResponse{
		Volume: volume,
	}, nil
}

// Delete implements volumes.VolumeServiceClient.
func (l *local) Delete(ctx context.Context, req *volumes.DeleteRequest, opts ...grpc.CallOption) (*volumes.DeleteResponse, error) {
	ctx, span := tracer.Start(ctx, "volume.Delete")
	defer span.End()

	volume, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, l.handleError(err, "couldn't GET volume from repo", "id", req.GetId())
	}
	err = l.Repo().Delete(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	err = l.exchange.Publish(ctx, eventsv1.EventType_VolumeDelete, events.NewEvent(eventsv1.EventType_VolumeDelete, volume))
	if err != nil {
		return nil, l.handleError(err, "error publishing DELETE event", "name", volume.GetMeta().GetName(), "event", "VolumeDelete")
	}
	return &volumes.DeleteResponse{
		Id: req.GetId(),
	}, nil
}

// Get implements volumes.VolumeServiceClient.
func (l *local) Get(ctx context.Context, req *volumes.GetRequest, opts ...grpc.CallOption) (*volumes.GetResponse, error) {
	ctx, span := tracer.Start(ctx, "volume.Get", trace.WithSpanKind(trace.SpanKindServer))
	span.SetAttributes(
		attribute.String("service", "Volume"),
		attribute.String("volume.id", req.GetId()),
	)
	defer span.End()

	// Validate request
	err := req.Validate()
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	// Get volume from repo
	volume, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		span.RecordError(err)
		return nil, l.handleError(err, "couldn't GET volume from repo", "name", req.GetId())
	}

	span.SetAttributes(attribute.String("volume.name", volume.GetMeta().GetName()))

	return &volumes.GetResponse{
		Volume: volume,
	}, nil
}

// List implements volumes.VolumeServiceClient.
func (l *local) List(ctx context.Context, req *volumes.ListRequest, opts ...grpc.CallOption) (*volumes.ListResponse, error) {
	ctx, span := tracer.Start(ctx, "volume.List")
	defer span.End()

	// Validate request
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	// Get volumes from repo
	ctrs, err := l.Repo().List(ctx, req.GetSelector())
	if err != nil {
		return nil, l.handleError(err, "couldn't LIST volumes from repo")
	}
	return &volumes.ListResponse{
		Volumes: ctrs,
	}, nil
}

// Update implements volumes.VolumeServiceClient.
func (l *local) UpdateStatus(ctx context.Context, req *volumes.UpdateStatusRequest, opts ...grpc.CallOption) (*volumes.UpdateStatusResponse, error) {
	ctx, span := tracer.Start(ctx, "volume.UpdateStatus")
	defer span.End()

	// Get the existing container before updating so we can compare specs
	existingVolume, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Apply mask safely
	base := proto.Clone(existingVolume.GetStatus()).(*volumes.Status)
	if err := applyMaskedUpdate(base, req.Status, req.UpdateMask); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "bad mask: %v", err)
	}

	existingVolume.Status = base
	if err := l.Repo().Update(ctx, existingVolume); err != nil {
		return nil, err
	}

	return &volumes.UpdateStatusResponse{
		Id: existingVolume.GetMeta().GetName(),
	}, nil
}

// UpdateStatus implements volumes.VolumeServiceClient.
func (l *local) Update(ctx context.Context, req *volumes.UpdateRequest, opts ...grpc.CallOption) (*volumes.UpdateResponse, error) {
	ctx, span := tracer.Start(ctx, "volume.Update")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Validate request
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	updateVolume := req.GetVolume()

	// Get the existing volume before updating so we can compare specs
	existingVolume, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Update the volume
	err = l.Repo().Update(ctx, updateVolume)
	if err != nil {
		return nil, l.handleError(err, "couldn't UPDATE volume in repo", "name", updateVolume.GetMeta().GetName())
	}

	// Retreive the volume again so that we can include it in an event
	ctr, err := l.Repo().Get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	// Only publish if spec is updated
	updVal := protoreflect.ValueOfMessage(updateVolume.GetConfig().ProtoReflect())
	newVal := protoreflect.ValueOfMessage(existingVolume.GetConfig().ProtoReflect())
	if !updVal.Equal(newVal) {

		updateVolume.Meta.Revision++

		l.logger.Debug("volume was updated, emitting event to listeners", "event", "VolumeUpdate", "name", ctr.GetMeta().GetName(), "revision", updateVolume.GetMeta().GetRevision())
		err = l.exchange.Publish(ctx, eventsv1.EventType_VolumeUpdate, events.NewEvent(eventsv1.EventType_VolumeUpdate, ctr))
		if err != nil {
			return nil, l.handleError(err, "error publishing UPDATE event", "name", ctr.GetMeta().GetName(), "event", "VolumeUpdate")
		}
	}

	return &volumes.UpdateResponse{
		Volume: ctr,
	}, nil
}

func (l *local) Repo() repository.VolumeRepository {
	if l.repo != nil {
		return l.repo
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return repository.NewVolumeInMemRepo()
}
