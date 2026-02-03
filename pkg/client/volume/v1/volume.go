package v1

import (
	"context"

	"github.com/amimof/voiyd/api/services/volumes/v1"
	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/labels"
	"github.com/amimof/voiyd/pkg/util"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

var _ ClientV1 = &clientV1{}

type CreateOption func(c *clientV1)

func WithEmitLabels(l labels.Label) CreateOption {
	return func(c *clientV1) {
		c.emitLabels = l
	}
}

func WithClient(client volumes.VolumeServiceClient) CreateOption {
	return func(c *clientV1) {
		c.Client = client
	}
}

type ClientV1 interface {
	Status() StatusClientV1
	Create(context.Context, *volumes.Volume, ...CreateOption) error
	Update(context.Context, string, *volumes.Volume) error
	Patch(context.Context, string, *volumes.Volume) error
	Get(context.Context, string) (*volumes.Volume, error)
	Delete(context.Context, string) error
	List(context.Context, ...labels.Label) ([]*volumes.Volume, error)
}

type StatusClientV1 interface {
	Update(context.Context, string, *volumes.Status, ...string) error
}

type clientV1 struct {
	Client     volumes.VolumeServiceClient
	emitLabels labels.Label
	id         string
}

type statusV1 struct {
	client volumes.VolumeServiceClient
}

func (c *clientV1) Status() StatusClientV1 {
	return &statusV1{
		client: c.Client,
	}
}

func (c *statusV1) Update(ctx context.Context, id string, status *volumes.Status, path ...string) error {
	// Construct field mask
	mask := &fieldmaskpb.FieldMask{
		Paths: path,
	}

	uid, err := keys.ParseStr(id)
	if err != nil {
		return err
	}

	req := &volumes.UpdateStatusRequest{
		Name:       uid.NameStr(),
		Uid:        uid.UUIDStr(),
		UpdateMask: mask,
		Status:     status,
	}

	_, err = c.client.UpdateStatus(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (c *clientV1) Create(ctx context.Context, ctr *volumes.Volume, opts ...CreateOption) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.volume.Update")
	defer span.End()

	for _, opt := range opts {
		opt(c)
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err := c.Client.Create(ctx, &volumes.CreateRequest{Volume: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Update(ctx context.Context, name string, ctr *volumes.Volume) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.volume.Update")
	defer span.End()

	uid, err := keys.ParseStr(name)
	if err != nil {
		return err
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err = c.Client.Update(ctx, &volumes.UpdateRequest{Uid: uid.UUIDStr(), Name: uid.NameStr(), Volume: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Patch(ctx context.Context, name string, ctr *volumes.Volume) error {
	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.volume.Patch")
	defer span.End()

	uid, err := keys.ParseStr(name)
	if err != nil {
		return err
	}

	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)
	_, err = c.Client.Patch(ctx, &volumes.PatchRequest{Uid: uid.UUIDStr(), Name: uid.NameStr(), Volume: ctr})
	if err != nil {
		return err
	}
	return nil
}

func (c *clientV1) Get(ctx context.Context, id string) (*volumes.Volume, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)

	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.volume.Get")
	defer span.End()

	uid, err := keys.ParseStr(id)
	if err != nil {
		return nil, err
	}

	res, err := c.Client.Get(ctx, &volumes.GetRequest{Name: uid.NameStr(), Uid: uid.UUIDStr()})
	if err != nil {
		return nil, err
	}
	return res.GetVolume(), nil
}

func (c *clientV1) List(ctx context.Context, l ...labels.Label) ([]*volumes.Volume, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)

	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.volume.List")
	defer span.End()

	mergedLabels := util.MergeLabels(l...)
	res, err := c.Client.List(ctx, &volumes.ListRequest{Selector: mergedLabels})
	if err != nil {
		return nil, err
	}
	return res.Volumes, nil
}

func (c *clientV1) Delete(ctx context.Context, id string) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "voiyd_client_id", c.id)

	tracer := otel.Tracer("client-v1")
	ctx, span := tracer.Start(ctx, "client.volume.Delete")
	defer span.End()

	uid, err := keys.ParseStr(id)
	if err != nil {
		return err
	}

	_, err = c.Client.Delete(ctx, &volumes.DeleteRequest{Name: uid.NameStr(), Uid: uid.UUIDStr()})
	if err != nil {
		return err
	}
	return nil
}

func NewClientV1(opts ...CreateOption) ClientV1 {
	c := &clientV1{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func NewClientV1WithConn(conn *grpc.ClientConn, clientID string, opts ...CreateOption) ClientV1 {
	c := &clientV1{
		Client: volumes.NewVolumeServiceClient(conn),
		id:     clientID,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}
