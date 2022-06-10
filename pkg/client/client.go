package client

import (
	"context"
	"log"
	"errors"
	"bytes"
	"google.golang.org/grpc"
	proto "github.com/amimof/blipblop/proto"
	"github.com/amimof/blipblop/internal/models"
	"io"
	"encoding/gob"
)

type Client struct {
	name string
	conn *grpc.ClientConn
	client proto.NodeServiceClient
}

func NewNodeServiceClient(server string) (*Client, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithBlock(), grpc.WithInsecure())
	conn, err := grpc.Dial(server, opts...)
	if err != nil {
		return nil, err
	}
	c := &Client{
		conn: conn,
		client: proto.NewNodeServiceClient(conn),
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
	return nil
	//go joinNode(ctx, c.client, name)
}

func (c *Client) Subscribe(ctx context.Context) (<-chan *proto.Event, <-chan error) {
	return c.subscribe(ctx, c.client, c.name)
}

func (c *Client) subscribe(ctx context.Context, client proto.NodeServiceClient, name string) (<-chan *proto.Event, <-chan error) {

	evc := make(chan *proto.Event)
	errc := make(chan error)

	stream, err := client.Subscribe(ctx, &proto.Node{Id: name})
	if err != nil {
		log.Fatalf("subscribe error occurred %s", err.Error())
	}
	log.Printf("Node %s joined", name)

	waitc := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				log.Println("Got EOF")
				close(waitc)
				return
			}
			if err != nil {
				//log.Printf("Failed to receive events from joined node: %s", err.Error())
				continue
			}
			log.Printf("Event %s on node %s len: %d", in.Type, in.Node.Id, len(in.Payload))
			var u models.Node
			buf := bytes.NewBuffer(in.Payload)
			dec := gob.NewDecoder(buf)
			err = dec.Decode(&u)
			if err != nil {
				log.Printf("Error decoding payload %s", err.Error())
				continue
			}
			log.Printf("Node name %s", *u.Name)
		}
	}()
	<-waitc
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
