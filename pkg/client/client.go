package client

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"github.com/amimof/blipblop/internal/models"
	proto "github.com/amimof/blipblop/proto"
	"google.golang.org/grpc"
	"io"
	"log"
)

type Client struct {
	name   string
	conn   *grpc.ClientConn
	client proto.NodeServiceClient
	//nodeService *services.NodeService
}

func NewNodeServiceClient(server string) (*Client, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithBlock(), grpc.WithInsecure())
	conn, err := grpc.Dial(server, opts...)
	if err != nil {
		return nil, err
	}
	c := &Client{
		conn:   conn,
		client: proto.NewNodeServiceClient(conn),
		//nodeService: services.NewNodeService(),
	}
	return c, nil
}

func (c *Client) Close() {
	c.conn.Close()
}

func (c *Client) JoinNode(ctx context.Context, name string) error {
	c.name = name
	n := &proto.JoinRequest{
		Node: &proto.Node{
			Id: c.name,
		},
	}
	res, err := c.client.Join(context.Background(), n)
	if err != nil {
		return err
	}
	if res.Status == proto.Status_JoinFail {
		return errors.New("unable to join node")
	}
	log.Printf("Node %s joined", name)
	return nil
}

func (c *Client) Subscribe(ctx context.Context) (<-chan *proto.Event, <-chan error) {
	return c.subscribe(ctx, c.name)
}

func (c *Client) subscribe(ctx context.Context, name string) (<-chan *proto.Event, <-chan error) {

	evc := make(chan *proto.Event)
	errc := make(chan error)

	stream, err := c.client.Subscribe(ctx, &proto.Node{Id: name})
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
			var u models.Node
			buf := bytes.NewBuffer(in.Payload)
			dec := gob.NewDecoder(buf)
			err = dec.Decode(&u)
			if err != nil {
				//log.Printf("Error decoding payload %s", err.Error())
				errc <- err
				//continue
			}
			//log.Printf("Node name %s", *u.Name)
			evc <- in
		}
	}()
	//<-waitc
	return evc, errc
}

// func sendEvent(ctx context.Context, client proto.NodeServiceClient, event, name string) {
// 	stream, err := client.FireEvent(ctx)
// 	if err != nil {
// 		log.Printf("Cannot fire event: %s", err.Error())
// 	}

// 	ev := proto.Event{
// 		Node: &proto.Node{
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
