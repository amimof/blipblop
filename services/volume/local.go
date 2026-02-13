package volume

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/repository"

	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
)

var (
	_      volumesv1.VolumeServiceClient = &local{}
	tracer                               = otel.GetTracerProvider().Tracer("volume-service")
)

type local struct {
	repo     *repository.Repo[*volumesv1.Volume]
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

func applyMaskedUpdate(dst, src *volumesv1.Status, mask *fieldmaskpb.FieldMask) error {
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
func (l *local) Patch(ctx context.Context, in *volumesv1.PatchRequest, opts ...grpc.CallOption) (*volumesv1.PatchResponse, error) {
	panic("unimplemented")
}

// Create implements volumes.VolumeServiceClient.
func (l *local) Create(ctx context.Context, req *volumesv1.CreateRequest, opts ...grpc.CallOption) (*volumesv1.CreateResponse, error) {
	ctx, span := tracer.Start(ctx, "volume.Create")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	volume := req.GetVolume()
	volumeName := volume.GetMeta().GetName()

	// Check if volume already exists
	if existing, _ := l.Get(ctx, &volumesv1.GetRequest{Name: volumeName}); existing != nil {
		return nil, fmt.Errorf("volume %s already exists", volume.GetMeta().GetName())
	}

	volume.GetMeta().ResourceVersion = 1
	volume.GetMeta().Generation = 1

	// Initialize status field if empty
	if volume.GetStatus() == nil {
		volume.Status = &volumesv1.Status{}
	}

	// Create volume in repo
	volume, err := l.repo.Create(ctx, volume)
	if err != nil {
		return nil, l.handleError(err, "error creating volume", "name", volumeName)
	}

	// Decorate label with some labels
	eventLabels := labels.New()
	eventLabels.Set(labels.LabelPrefix("object-id").String(), volume.GetMeta().GetName())
	eventLabels.Set(labels.LabelPrefix("object-version").String(), volume.GetVersion())

	// Publish event that volume is created
	err = l.exchange.Forward(ctx, events.NewEvent(events.VolumeCreate, volume, eventLabels))
	if err != nil {
		return nil, l.handleError(err, "error publishing volume create event", "name", volume.GetMeta().GetName(), "event", "VolumeCreate")
	}

	return &volumesv1.CreateResponse{
		Volume: volume,
	}, nil
}

// Delete implements volumes.VolumeServiceClient.
func (l *local) Delete(ctx context.Context, req *volumesv1.DeleteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	ctx, span := tracer.Start(ctx, "volume.Delete")
	defer span.End()

	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, l.handleError(err, "couldn't parse uid")
	}

	volume, err := l.repo.Get(ctx, uid)
	if err != nil {
		return nil, l.handleError(err, "error deleting repo", "id", req.GetUid())
	}

	err = l.repo.Delete(ctx, uid)
	if err != nil {
		return nil, err
	}

	// Decorate label with some labels
	eventLabels := labels.New()
	eventLabels.Set(labels.LabelPrefix("object-id").String(), volume.GetMeta().GetName())
	eventLabels.Set(labels.LabelPrefix("object-version").String(), volume.GetVersion())

	err = l.exchange.Forward(ctx, events.NewEvent(events.VolumeDelete, volume, eventLabels))
	if err != nil {
		return nil, l.handleError(err, "error publishing volume delete event", "name", volume.GetMeta().GetName(), "event", "VolumeDelete")
	}
	return &emptypb.Empty{}, nil
}

// Get implements volumes.VolumeServiceClient.
func (l *local) Get(ctx context.Context, req *volumesv1.GetRequest, opts ...grpc.CallOption) (*volumesv1.GetResponse, error) {
	ctx, span := tracer.Start(ctx, "volume.Get", trace.WithSpanKind(trace.SpanKindServer))
	span.SetAttributes(
		attribute.String("service", "Volume"),
		attribute.String("volume.id", req.GetUid()),
	)
	defer span.End()

	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, l.handleError(err, "couldn't parse uid")
	}

	// Get volume from repo
	volume, err := l.repo.Get(ctx, uid)
	if err != nil {
		span.RecordError(err)
		return nil, l.handleError(err, "error getting volume", "name", uid.String())
	}

	span.SetAttributes(attribute.String("volume.name", volume.GetMeta().GetName()))

	return &volumesv1.GetResponse{
		Volume: volume,
	}, nil
}

// List implements volumes.VolumeServiceClient.
func (l *local) List(ctx context.Context, req *volumesv1.ListRequest, opts ...grpc.CallOption) (*volumesv1.ListResponse, error) {
	ctx, span := tracer.Start(ctx, "volume.List")
	defer span.End()

	// Get volumes from repo
	ctrs, err := l.repo.List(ctx, int(req.GetLimit()))
	if err != nil {
		return nil, l.handleError(err, "error listing volumes")
	}
	return &volumesv1.ListResponse{
		Volumes: ctrs,
	}, nil
}

// Update implements volumes.VolumeServiceClient.
func (l *local) UpdateStatus(ctx context.Context, req *volumesv1.UpdateStatusRequest, opts ...grpc.CallOption) (*volumesv1.UpdateStatusResponse, error) {
	ctx, span := tracer.Start(ctx, "volume.UpdateStatus")
	defer span.End()

	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, l.handleError(err, "couldn't parse uid")
	}

	// Get the existing container before updating so we can compare specs
	existingVolume, err := l.repo.Get(ctx, uid)
	if err != nil {
		return nil, err
	}

	// Apply mask safely
	base := proto.Clone(existingVolume.GetStatus()).(*volumesv1.Status)
	if err := applyMaskedUpdate(base, req.Status, req.UpdateMask); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "bad mask: %v", err)
	}

	existingVolume.GetMeta().ResourceVersion++
	existingVolume.Status = base

	if err := l.repo.Update(ctx, uid, existingVolume); err != nil {
		return nil, err
	}

	return &volumesv1.UpdateStatusResponse{
		Id: existingVolume.GetMeta().GetName(),
	}, nil
}

// UpdateStatus implements volumes.VolumeServiceClient.
func (l *local) Update(ctx context.Context, req *volumesv1.UpdateRequest, opts ...grpc.CallOption) (*volumesv1.UpdateResponse, error) {
	ctx, span := tracer.Start(ctx, "volume.Update")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	uid, err := keys.FromUIDOrName(req.GetUid(), req.GetName())
	if err != nil {
		return nil, l.handleError(err, "couldn't parse uid")
	}

	updateVolume := req.GetVolume()

	// Get the existing volume before updating so we can compare specs
	existingVolume, err := l.repo.Get(ctx, uid)
	if err != nil {
		return nil, err
	}

	// Ignore fields
	updateVolume.GetMeta().ResourceVersion++
	updateVolume.Status = existingVolume.Status
	updateVolume.GetMeta().Updated = existingVolume.Meta.Updated
	updateVolume.GetMeta().Created = existingVolume.Meta.Created
	updateVolume.GetMeta().ResourceVersion = existingVolume.Meta.ResourceVersion

	updVal := protoreflect.ValueOfMessage(updateVolume.GetConfig().ProtoReflect())
	newVal := protoreflect.ValueOfMessage(existingVolume.GetConfig().ProtoReflect())

	// Only update metadata fields if spec is updated
	if !updVal.Equal(newVal) {
		updateVolume.Meta.Generation++
		updateVolume.Meta.Updated = timestamppb.Now()
	}

	// Update the volume
	err = l.repo.Update(ctx, uid, updateVolume)
	if err != nil {
		return nil, l.handleError(err, "error updating volume", "name", updateVolume.GetMeta().GetName())
	}

	// Retreive the volume again so that we can include it in an event
	volume, err := l.repo.Get(ctx, uid)
	if err != nil {
		return nil, err
	}

	// Only publish if spec is updated
	if !updVal.Equal(newVal) {

		// Decorate label with some labels
		eventLabels := labels.New()
		eventLabels.Set(labels.LabelPrefix("object-id").String(), volume.GetMeta().GetName())
		eventLabels.Set(labels.LabelPrefix("object-version").String(), volume.GetVersion())

		err = l.exchange.Forward(ctx, events.NewEvent(events.VolumeUpdate, volume, eventLabels))
		if err != nil {
			return nil, l.handleError(err, "error publishing UPDATE event", "name", volume.GetMeta().GetName(), "event", "VolumeUpdate")
		}
	}

	return &volumesv1.UpdateResponse{
		Volume: volume,
	}, nil
}
