package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/logger"
	"github.com/amimof/voiyd/pkg/protoutils"
	"github.com/amimof/voiyd/pkg/repository"

	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

type VolumeService struct {
	Repo     *repository.Repo[*volumesv1.Volume]
	mu       sync.Mutex
	Exchange *events.Exchange
	Logger   logger.Logger
}

func (l *VolumeService) handleError(err error, msg string, keysAndValues ...any) error {
	def := []any{"error", err.Error()}
	def = append(def, keysAndValues...)
	l.Logger.Error(msg, def...)
	if errors.Is(err, repository.ErrNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}

func applyMaskedUpdateVolume(dst, src *volumesv1.Status, mask *fieldmaskpb.FieldMask) error {
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

func (l *VolumeService) Get(ctx context.Context, id keys.ID) (*volumesv1.Volume, error) {
	ctx, span := tracer.Start(ctx, "volume.Get", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()

	return l.Repo.Get(ctx, id)
}

func (l *VolumeService) List(ctx context.Context, limit int32) ([]*volumesv1.Volume, error) {
	ctx, span := tracer.Start(ctx, "volume.List")
	defer span.End()

	// Get volumes from repo
	return l.Repo.List(ctx, limit)
}

func (l *VolumeService) Create(ctx context.Context, volume *volumesv1.Volume) (*volumesv1.Volume, error) {
	ctx, span := tracer.Start(ctx, "volume.Create")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Create volume in repo
	newVolume, err := l.Repo.Create(ctx, volume)
	if err != nil {
		return nil, l.handleError(err, "error creating volume", "name", newVolume.GetMeta().GetName())
	}

	// Publish event that volume is created
	err = l.Exchange.Forward(ctx, events.NewEvent(events.VolumeCreate, volume))
	if err != nil {
		return nil, l.handleError(err, "error publishing volume create event", "name", newVolume.GetMeta().GetName())
	}

	return newVolume, nil
}

// Delete publishes a delete request and the subscribers are responsible for deleting resources.
// Once they do, they will update there resource with the status Deleted
func (l *VolumeService) Delete(ctx context.Context, id keys.ID) error {
	ctx, span := tracer.Start(ctx, "volume.Delete")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	volume, err := l.Repo.Get(ctx, id)
	if err != nil {
		return err
	}

	err = l.Repo.Delete(ctx, id)
	if err != nil {
		return err
	}

	err = l.Exchange.Forward(ctx, events.NewEvent(events.VolumeDelete, volume))
	if err != nil {
		return l.handleError(err, "error publishing volume delete event", "name", volume.GetMeta().GetName())
	}

	return nil
}

func (l *VolumeService) Patch(ctx context.Context, id keys.ID, patch *volumesv1.Volume) error {
	ctx, span := tracer.Start(ctx, "volume.Patch")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Get existing volume from repo
	existing, err := l.Repo.Get(ctx, id)
	if err != nil {
		return l.handleError(err, "error getting volume", "name", patch.GetMeta().GetName())
	}

	// Generate field mask
	genFieldMask, err := protoutils.GenerateFieldMask(existing, patch)
	if err != nil {
		return err
	}

	// Handle partial update
	maskedUpdate, err := protoutils.ApplyFieldMaskToNewMessage(patch, genFieldMask)
	if err != nil {
		return err
	}

	updated := maskedUpdate.(*volumesv1.Volume)
	existing = protoutils.StrategicMerge(existing, updated)

	// Update the volume
	volume, err := l.Repo.Update(ctx, id, existing)
	if err != nil {
		return l.handleError(err, "error updating volume", "name", existing.GetMeta().GetName())
	}

	changed, err := protoutils.SpecEqual(existing.GetConfig(), volume.GetConfig())
	if err != nil {
		return err
	}

	// Only publish if spec is updated
	if changed {
		err = l.Exchange.Forward(ctx, events.NewEvent(events.VolumePatch, volume))
		if err != nil {
			return l.handleError(err, "error publishing volume patch event", "name", existing.GetMeta().GetName())
		}
	}

	return nil
}

// UpdateStatus implements volumes.VolumeServieClient.
func (l *VolumeService) UpdateStatus(ctx context.Context, id keys.ID, st *volumesv1.Status, mask ...string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	ctx, span := tracer.Start(ctx, "volume.UpdateStatus")
	defer span.End()

	// Get the existing volume before updating so we can compare specs
	existingVolume, err := l.Repo.Get(ctx, id)
	if err != nil {
		return err
	}

	// Apply mask safely
	base := proto.Clone(existingVolume.Status).(*volumesv1.Status)
	if err := applyMaskedUpdateVolume(base, st, &fieldmaskpb.FieldMask{Paths: mask}); err != nil {
		return status.Errorf(codes.InvalidArgument, "bad mask: %v", err)
	}

	existingVolume.Status = base

	if _, err := l.Repo.Update(ctx, id, existingVolume); err != nil {
		return err
	}

	return nil
}

func (l *VolumeService) Update(ctx context.Context, id keys.ID, volume *volumesv1.Volume) error {
	ctx, span := tracer.Start(ctx, "volume.Update")
	defer span.End()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Get the existing volume before updating so we can compare specs
	existingVolume, err := l.Repo.Get(ctx, id)
	if err != nil {
		return err
	}

	// Update the volume
	updated, err := l.Repo.Update(ctx, id, volume)
	if err != nil {
		return l.handleError(err, "error updating volume", "name", volume.GetMeta().GetName())
	}

	changed, err := protoutils.SpecEqual(existingVolume.GetConfig(), updated.GetConfig())
	if err != nil {
		return err
	}

	// Only publish if spec is updated
	if changed {
		l.Logger.Debug("volume was updated, emitting event to listeners", "event", "VolumeUpdate", "name", updated.GetMeta().GetName())
		err = l.Exchange.Forward(ctx, events.NewEvent(events.VolumeUpdate, updated))
		if err != nil {
			return l.handleError(err, "error publishing volume update event", "name", updated.GetMeta().GetName())
		}
	}

	return nil
}

func (l *VolumeService) Condition(ctx context.Context, id keys.ID, req *typesv1.ConditionRequest) error {
	st := &volumesv1.Status{
		Conditions: req.GetConditions(),
	}

	err := l.UpdateStatus(ctx, id, st, "conditions")
	if err != nil {
		return err
	}

	err = l.Exchange.Publish(ctx, events.NewEvent(events.ConditionReported, req))
	if err != nil {
		return err
	}
	return nil
}
