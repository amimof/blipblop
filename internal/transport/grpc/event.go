package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/amimof/voiyd/internal/app"
	"github.com/amimof/voiyd/pkg/client/version"
	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/keys"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	"github.com/amimof/voiyd/api/types/v1"
)

var ErrClientExists = errors.New("client already exists")

type NewServiceOption func(s *EventService)

type EventService struct {
	eventsv1.UnimplementedEventServiceServer
	app *app.EventService
}

func (n *EventService) Register(server *grpc.Server) {
	server.RegisterService(&eventsv1.EventService_ServiceDesc, n)
}

func (n *EventService) Get(ctx context.Context, req *eventsv1.GetRequest) (*eventsv1.GetResponse, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), "")
	if err != nil {
		return nil, err
	}
	event, err := n.app.Get(ctx, uid)
	if err != nil {
		return nil, err
	}
	return &eventsv1.GetResponse{Event: event}, nil
}

func (n *EventService) Create(ctx context.Context, req *eventsv1.CreateRequest) (*eventsv1.CreateResponse, error) {
	event, err := n.app.Create(ctx, req.GetEvent())
	if err != nil {
		return nil, err
	}
	return &eventsv1.CreateResponse{Event: event}, nil
}

func (n *EventService) Delete(ctx context.Context, req *eventsv1.DeleteRequest) (*eventsv1.DeleteResponse, error) {
	uid, err := keys.FromUIDOrName(req.GetUid(), "")
	if err != nil {
		return nil, err
	}

	err = n.app.Delete(ctx, uid)
	if err != nil {
		return nil, err
	}

	return &eventsv1.DeleteResponse{Uid: uid.UUIDStr()}, nil
}

func (n *EventService) List(ctx context.Context, req *eventsv1.ListRequest) (*eventsv1.ListResponse, error) {
	events, err := n.app.List(ctx, req.GetLimit())
	if err != nil {
		return nil, err
	}
	return &eventsv1.ListResponse{Events: events}, nil
}

func (s *EventService) Subscribe(req *eventsv1.SubscribeRequest, stream eventsv1.EventService_SubscribeServer) error {
	var nodeUID string
	var nodeName string
	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		if res, ok := md["x-voiyd-node-uid"]; ok && len(res) > 0 {
			nodeUID = res[0]
		}
	}
	if md, ok := metadata.FromIncomingContext(stream.Context()); ok {
		if res, ok := md["x-voiyd-node-name"]; ok && len(res) > 0 {
			nodeName = res[0]
		}
	}

	sess, err := s.app.Subscribe(stream.Context(), app.NodeConnectInput{
		NodeUID:  nodeUID,
		NodeName: nodeName,
	})
	if err != nil {
		return err
	}

	ctx := stream.Context()

	errCh := make(chan error, 2)
	eventChan := s.app.Exchange.Subscribe(ctx, events.ALL...)

	go func() {
		for {
			select {
			case n := <-eventChan:

				err := stream.Send(n)
				if err != nil {
					errCh <- err
					return
				}
			case <-ctx.Done():

				// Get node name from context
				if md, ok := metadata.FromIncomingContext(ctx); ok {
					if nodeName, ok := md["x-voiyd-node-name"]; ok && len(nodeName) > 0 {
						_, err := s.Publish(ctx, &eventsv1.PublishRequest{Event: events.NewEvent(events.NodeForget, &nodesv1.Node{Version: version.VersionNode, Meta: &types.Meta{Name: nodeName[0]}})})
						if err != nil {
							errCh <- err
						}
					}
				}
				return
			}
		}
	}()

	// Writer: app -> node
	go func() {
		for {
			out, err := sess.Next(stream.Context())
			if err != nil {
				errCh <- err
				return
			}

			if err := stream.Send(out); err != nil {
				errCh <- err
				return
			}
		}
	}()

	<-ctx.Done()
	err = <-errCh
	return err
}

func (s *EventService) Forward(ctx context.Context, event *eventsv1.Event) error {
	_, err := s.app.Create(ctx, event)
	if err != nil {
		return err
	}
	return nil
}

func (s *EventService) Publish(ctx context.Context, req *eventsv1.PublishRequest) (*eventsv1.PublishResponse, error) {
	res, err := s.app.Create(ctx, req.GetEvent())
	if err != nil {
		return nil, err
	}

	ev, err := s.app.Publish(ctx, res)
	if err != nil {
		return nil, err
	}

	return &eventsv1.PublishResponse{Event: ev}, nil
}

func NewEventService(app *app.EventService) *EventService {
	return &EventService{app: app}
}
