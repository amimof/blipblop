package volume

import (
	"context"
	"sync"

	"github.com/amimof/blipblop/api/services/volumes/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/repository"
	"google.golang.org/grpc"
)

var _ volumes.VolumeServiceClient = &local{}

type local struct {
	repo     repository.VolumeRepository
	mu       sync.Mutex
	exchange *events.Exchange
	logger   logger.Logger
}

// Patch implements volumes.VolumeServiceClient.
func (l *local) Patch(ctx context.Context, in *volumes.PatchRequest, opts ...grpc.CallOption) (*volumes.PatchResponse, error) {
	panic("unimplemented")
}

// Create implements volumes.VolumeServiceClient.
func (l *local) Create(ctx context.Context, in *volumes.CreateRequest, opts ...grpc.CallOption) (*volumes.CreateResponse, error) {
	panic("unimplemented")
}

// Delete implements volumes.VolumeServiceClient.
func (l *local) Delete(ctx context.Context, in *volumes.DeleteRequest, opts ...grpc.CallOption) (*volumes.DeleteResponse, error) {
	panic("unimplemented")
}

// Get implements volumes.VolumeServiceClient.
func (l *local) Get(ctx context.Context, in *volumes.GetRequest, opts ...grpc.CallOption) (*volumes.GetResponse, error) {
	panic("unimplemented")
}

// List implements volumes.VolumeServiceClient.
func (l *local) List(ctx context.Context, in *volumes.ListRequest, opts ...grpc.CallOption) (*volumes.ListResponse, error) {
	panic("unimplemented")
}

// Update implements volumes.VolumeServiceClient.
func (l *local) Update(ctx context.Context, in *volumes.UpdateRequest, opts ...grpc.CallOption) (*volumes.UpdateResponse, error) {
	panic("unimplemented")
}

// UpdateStatus implements volumes.VolumeServiceClient.
func (l *local) UpdateStatus(ctx context.Context, in *volumes.UpdateStatusRequest, opts ...grpc.CallOption) (*volumes.UpdateStatusResponse, error) {
	panic("unimplemented")
}
