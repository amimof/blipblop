package client

import (
	//"bytes"
	"context"
	//"encoding/gob"
	"errors"
	//"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/api/services/nodes/v1"
	"github.com/amimof/blipblop/api/services/containers/v1"
	"google.golang.org/grpc"
	"io"
	"log"
)

type RESTClient struct {

}

type Client struct {
	name         string
	conn         *grpc.ClientConn
	nodeService  nodes.NodeServiceClient
	eventService events.EventServiceClient
	containerService containers.ContainerServiceClient
}

func New(server string) (*Client, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithBlock(), grpc.WithInsecure())
	conn, err := grpc.Dial(server, opts...)
	if err != nil {
		return nil, err
	}
	c := &Client{
		conn:         conn,
		nodeService:  nodes.NewNodeServiceClient(conn),
		eventService: events.NewEventServiceClient(conn),
		containerService: containers.NewContainerServiceClient(conn), 
	}
	return c, nil
}

func (c *Client) Close() {
	c.conn.Close()
}

func (c *Client) JoinNode(ctx context.Context, name string) error {
	c.name = name
	n := &nodes.JoinRequest{
		Node: &nodes.Node{
			Id: c.name,
		},
	}
	res, err := c.nodeService.Join(context.Background(), n)
	if err != nil {
		return err
	}
	if res.Status == nodes.Status_JoinFail {
		return errors.New("unable to join node")
	}
	log.Printf("Node %s joined", name)
	return nil
}

func (c *Client) GetContainer(ctx context.Context, id string) (*containers.Container, error) {
	res, err := c.containerService.GetTest(ctx, &containers.GetContainerRequest{Id: id})
	if err != nil {
		return nil, err
	}
	return res.Container, nil
}

func (c *Client) Subscribe(ctx context.Context) (<-chan *events.Event, <-chan error) {
	return c.subscribe(ctx, c.name)
}

func (c *Client) subscribe(ctx context.Context, name string) (<-chan *events.Event, <-chan error) {

	evc := make(chan *events.Event)
	errc := make(chan error)

	stream, err := c.eventService.Subscribe(ctx, &events.SubscribeRequest{Id: name})
	if err != nil {
		log.Fatalf("subscribe error occurred %s", err.Error())
	}

	//waitc := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				errc <- err
				//close(waitc)
				//return
			}
			if err != nil {
				//log.Printf("Failed to receive events from joined node: %s", err.Error())
				errc <- err
				//continue
			}
			//log.Printf("Event %s on node %s len: %d", in.Type, in.Node.Id, len(in.Payload))
			// var u models.Node
			// buf := bytes.NewBuffer(in.Payload)
			// dec := gob.NewDecoder(buf)
			// err = dec.Decode(&u)
			// if err != nil {
			// 	//log.Printf("Error decoding payload %s", err.Error())
			// 	errc <- err
			// 	//continue
			// }
			//log.Printf("Node name %s", *u.Name)
			evc <- in
		}
	}()
	//<-waitc
	return evc, errc
}

// func sendEvent(ctx context.Context, client nodes.NodeServiceClient, event, name string) {
// 	stream, err := client.FireEvent(ctx)
// 	if err != nil {
// 		log.Printf("Cannot fire event: %s", err.Error())
// 	}

// 	ev := events.Event{
// 		Node: &nodes.Node{
// 			Id: name,
// 		},
// 		Name: event,
// 	}

// 	stream.Send(&ev)

// 	ack, err := stream.CloseAndRecv()
// 	if err != nil {
// 		log.Fatalf("Cannot send event: %s", err.Error())
// 	}
// 	log.Printf("Event sent: %v", ack)
// }
