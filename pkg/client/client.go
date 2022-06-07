package client

import (
	"context"
	"log"
	"bytes"
	"google.golang.org/grpc"
	proto "github.com/amimof/blipblop/proto"
	"github.com/amimof/blipblop/internal/models"
	"io"
	"encoding/gob"
)

type Client struct {
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

func (c *Client) JoinNode(ctx context.Context, name string) {
	go joinNode(ctx, c.client, name)
}

func joinNode(ctx context.Context, client proto.NodeServiceClient, name string) {
	node := proto.Node{Id: name}
	stream, err := client.JoinNode(ctx, &node)
	if err != nil {
		log.Fatalf("join node error %s", err.Error())
	}

	log.Printf("Node %s joined", node.Id)

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
			var u models.Unit
			buf := bytes.NewBuffer(in.Payload)
			dec := gob.NewDecoder(buf)
			err = dec.Decode(&u)
			if err != nil {
				log.Printf("Error decoding payload %s", err.Error())
				continue
			}
			log.Printf("Unit name %s, image %s", *u.Name, *u.Image)
		}
	}()
	<-waitc
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
