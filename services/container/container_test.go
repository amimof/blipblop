package container

import (
	"context"
	"log"
	"net"
	"testing"

	"github.com/amimof/blipblop/api/services/containers/v1"
	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/repository"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

func initTestServer() (*grpc.Server, *bufconn.Listener) {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	svc := NewService(repository.NewContainerInMemRepo(), WithExchange(events.NewExchange()))
	containersv1.RegisterContainerServiceServer(s, svc)
	go func() {
		if err := s.Serve(lis); err != nil {
			panic(err)
		}
	}()
	return s, lis
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func initDB(c containersv1.ContainerServiceClient) error {
	ctx := context.Background()
	containers := []*containersv1.Container{
		{
			Meta: &types.Meta{
				Name: "test-container-1",
				Labels: map[string]string{
					"team":        "backend",
					"environment": "production",
					"role":        "root",
				},
			},
			Config: &containersv1.Config{
				Image: "docker.io/library/nginx:latest",
				PortMappings: []*containersv1.PortMapping{
					{
						HostPort:      8080,
						ContainerPort: 80,
						Protocol:      "TCP",
					},
				},
				Envvars: []*containersv1.EnvVar{
					{
						Name:  "HTTP_PROXY",
						Value: "proxy.foo.com",
					},
				},
				Args: []string{
					"--config",
					"/mnt/cfg/config.yaml",
				},
				Mounts: []*containersv1.Mount{
					{
						Source:      "/tmp",
						Destination: "/mnt/tmp",
						Type:        "bind",
					},
				},
				NodeSelector: map[string]string{
					"blipblop.io/arch": "amd64",
					"blipblop.io/os":   "linux",
				},
			},
		},
	}

	for _, ctr := range containers {
		_, err := c.Create(ctx, &containersv1.CreateContainerRequest{Container: ctr})
		if err != nil {
			return err
		}
	}

	return nil
}

func TestContainerService_Update(t *testing.T) {
	server, _ := initTestServer()
	defer server.Stop()

	ctx := context.Background()
	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := containersv1.NewContainerServiceClient(conn)
	if err := initDB(client); err != nil {
		log.Fatalf("error initializing DB: %v", err)
	}

	testCases := []struct {
		name   string
		expect *containers.UpdateContainerResponse
		input  *containers.UpdateContainerRequest
	}{
		{
			name: "should only update container image",
			expect: &containersv1.UpdateContainerResponse{
				Container: &containersv1.Container{
					Meta: &types.Meta{
						Name: "test-container-1",
					},
					Config: &containersv1.Config{
						Image: "docker.io/library/nginx:v1.27.1",
					},
				},
			},
			input: &containersv1.UpdateContainerRequest{
				Id: "test-container-1",
				Container: &containersv1.Container{
					Meta: &types.Meta{
						Name: "test-container-1",
					},
					Config: &containersv1.Config{
						Image: "docker.io/library/nginx:v1.27.1",
					},
				},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			res, err := client.Update(ctx, tt.input)
			if err != nil {
				t.Fatal("error updating container", err)
			}
			// Ignore timestamps
			res.Container.Meta.Created = nil
			if !proto.Equal(tt.expect, res) {
				t.Errorf("got %v, want %v", tt.input, res)
			}
		})
	}
}
