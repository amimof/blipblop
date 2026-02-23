package grpc

import (
	"context"

	"github.com/amimof/voiyd/internal/app"
	"google.golang.org/grpc"

	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
)

var _ leasesv1.LeaseServiceServer = &LeaseService{}

type LeaseService struct {
	leasesv1.UnimplementedLeaseServiceServer
	app *app.LeaseService
}

func (c *LeaseService) Register(server *grpc.Server) {
	leasesv1.RegisterLeaseServiceServer(server, c)
}

func (c *LeaseService) Get(ctx context.Context, req *leasesv1.GetRequest) (*leasesv1.GetResponse, error) {
	lease, err := c.app.Get(ctx, app.ResourceID(req.GetUid()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &leasesv1.GetResponse{Lease: lease}, nil
}

func (c *LeaseService) List(ctx context.Context, req *leasesv1.ListRequest) (*leasesv1.ListResponse, error) {
	leases, err := c.app.List(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	return &leasesv1.ListResponse{Leases: leases}, nil
}

func (c *LeaseService) Acquire(ctx context.Context, req *leasesv1.AcquireRequest) (*leasesv1.AcquireResponse, error) {
	lease, token, err := c.app.Acquire(ctx, app.ResourceID(req.GetTaskId()), app.HolderID(req.GetNodeId()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &leasesv1.AcquireResponse{Lease: lease, Token: token, FencingToken: lease.GetConfig().GetFencingToken()}, nil
}

func (c *LeaseService) Release(ctx context.Context, req *leasesv1.ReleaseRequest) (*leasesv1.ReleaseResponse, error) {
	err := c.app.Release(ctx, app.ResourceID(req.GetTaskId()), app.HolderID(req.GetNodeId()), req.GetToken())
	if err != nil {
		return nil, toStatus(err)
	}
	return &leasesv1.ReleaseResponse{Released: true}, nil
}

func (c *LeaseService) Revoke(ctx context.Context, req *leasesv1.RevokeRequest) (*leasesv1.RevokeResponse, error) {
	err := c.app.Revoke(ctx, app.ResourceID(req.GetTaskId()), app.HolderID(req.GetNodeId()))
	if err != nil {
		return nil, toStatus(err)
	}
	return &leasesv1.RevokeResponse{Released: true}, nil
}

func (c *LeaseService) Renew(ctx context.Context, req *leasesv1.RenewRequest) (*leasesv1.RenewResponse, error) {
	lease, token, err := c.app.Renew(ctx, app.ResourceID(req.GetTaskId()), app.HolderID(req.GetNodeId()), req.GetToken())
	if err != nil {
		return nil, toStatus(err)
	}
	return &leasesv1.RenewResponse{Lease: lease, Token: token, FencingToken: lease.GetConfig().GetFencingToken()}, nil
}

func NewLeaseService(app *app.LeaseService) *LeaseService {
	return &LeaseService{app: app}
}
