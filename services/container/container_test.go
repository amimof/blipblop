package container

import (
	"context"
	"fmt"
	"log"
	"net"
	"testing"

	containersv1 "github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/repository"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
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

func initDB(ctx context.Context, c containersv1.ContainerServiceClient) error {
	baseContainer := containersv1.Container{
		Meta: &types.Meta{
			Name: "test-container",
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
					Name:          "http",
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
				"--config /mnt/cfg/config.yaml",
			},
			Mounts: []*containersv1.Mount{
				{
					Name:        "temp",
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
	}

	// Create 5 copies of baseContainer but with different names
	for i := 1; i < 10; i++ {
		ctr := &baseContainer
		ctr.Meta.Name = fmt.Sprintf("test-container-%d", i)
		_, err := c.Create(ctx, &containersv1.CreateRequest{Container: ctr})
		if err != nil {
			return err
		}
	}

	bareBoneContainer := &containersv1.Container{
		Meta: &types.Meta{
			Name: "bare-container",
		},
		Config: &containersv1.Config{
			Image: "docker.io/library/nginx:latest",
		},
	}

	// Create bare bone container so test nilness
	_, err := c.Create(ctx, &containersv1.CreateRequest{Container: bareBoneContainer})
	if err != nil {
		return err
	}

	return nil
}

// type createOpts func(*containersv1.Container)
//
// func withImage(i string) createOpts {
// 	return func(c *containersv1.Container) {
// 		c.Config.Image = i
// 	}
// }
//
// func withEnvVar(ev ...*containersv1.EnvVar) createOpts {
// 	return func(c *containersv1.Container) {
// 		c.Config.Envvars = ev
// 	}
// }
//
// func withPortMapping(pm ...*containersv1.PortMapping) createOpts {
// 	return func(c *containersv1.Container) {
// 		c.Config.PortMappings = pm
// 	}
// }
//
// func withArgs(args []string) createOpts {
// 	return func(c *containersv1.Container) {
// 		c.Config.Args = args
// 	}
// }
//
// func withMounts(mounts ...*containersv1.Mount) createOpts {
// 	return func(c *containersv1.Container) {
// 		c.Config.Mounts = mounts
// 	}
// }
//
// func withNodeSelector(key, val string) createOpts {
// 	return func(c *containersv1.Container) {
// 		c.Config.NodeSelector = make(map[string]string)
// 		c.Config.NodeSelector[key] = val
// 	}
// }
//
// // createPatch creates a base update request. Add additional fields using createOpts
// func createPatch(name, image string, opts ...createOpts) *containersv1.UpdateContainerRequest {
// 	ctr := &containersv1.Container{
// 		Meta: &types.Meta{
// 			Name: name,
// 		},
// 		Config: &containersv1.Config{
// 			Image: image,
// 		},
// 	}
//
// 	for _, opt := range opts {
// 		opt(ctr)
// 	}
//
// 	return &containersv1.UpdateContainerRequest{Container: ctr, Id: name}
// }

func Test_ContainerService_Status(t *testing.T) {
	server, _ := initTestServer()
	defer server.Stop()

	ctx := context.Background()

	//nolint:staticcheck
	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("error closing connection: %v", err)
		}
	}()

	client := containersv1.NewContainerServiceClient(conn)
	if err := initDB(ctx, client); err != nil {
		log.Fatalf("error initializing DB: %v", err)
	}

	testCases := []struct {
		name        string
		expect      *containersv1.Status
		patch       *containersv1.Status
		containerID string
		mask        string
	}{
		{
			name:        "should be equal",
			containerID: "test-container-1",
			mask:        "phase",
			patch: &containersv1.Status{
				Phase: wrapperspb.String("creating"),
			},
			expect: &containersv1.Status{
				Phase: wrapperspb.String("creating"),
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := &containersv1.UpdateStatusRequest{Id: tt.containerID, Status: tt.patch, UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{tt.mask}}}
			_, err := client.UpdateStatus(ctx, req)
			if err != nil {
				t.Fatal("error updating container", err)
			}

			expectedVal := protoreflect.ValueOfMessage(tt.expect.ProtoReflect())

			updated, err := client.Get(ctx, &containersv1.GetRequest{Id: tt.containerID})
			if err != nil {
				t.Fatal("error getting container", err)
			}

			resultVal := protoreflect.ValueOfMessage(updated.GetContainer().GetStatus().ProtoReflect())

			if !resultVal.Equal(expectedVal) {
				t.Errorf("\ngot:\n%v\nwant:\n%v", resultVal, expectedVal)
			}
		})
	}
}

func Test_ContainerService_Equal(t *testing.T) {
	server, _ := initTestServer()
	defer server.Stop()

	ctx := context.Background()

	//nolint:staticcheck
	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("error closing connection: %v", err)
		}
	}()

	client := containersv1.NewContainerServiceClient(conn)
	if err := initDB(ctx, client); err != nil {
		log.Fatalf("error initializing DB: %v", err)
	}

	testCases := []struct {
		name   string
		expect *containersv1.Container
		patch  *containersv1.Container
	}{
		{
			name: "should be equal",
			patch: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-1",
				},
				Config: &containersv1.Config{
					// Updates image tag
					Image: "docker.io/library/nginx:v1.27.3",
				},
			},
			expect: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container",
					Labels: map[string]string{
						"team":        "backend",
						"environment": "production",
						"role":        "root",
					},
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/nginx:v1.27.3",
					PortMappings: []*containersv1.PortMapping{
						{
							Name:          "http",
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
						"--config /mnt/cfg/config.yaml",
					},
					Mounts: []*containersv1.Mount{
						{
							Name:        "temp",
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
				Status: &containersv1.Status{},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := &containersv1.PatchRequest{Id: tt.patch.Meta.Name, Container: tt.patch}
			res, err := client.Patch(ctx, req)
			if err != nil {
				t.Fatal("error updating container", err)
			}

			expectedVal := protoreflect.ValueOfMessage(tt.expect.GetConfig().ProtoReflect())
			resultVal := protoreflect.ValueOfMessage(res.GetContainer().GetConfig().ProtoReflect())

			if !resultVal.Equal(expectedVal) {
				t.Errorf("\ngot:\n%v\nwant:\n%v", resultVal, expectedVal)
			}
		})
	}
}

func Test_ContainerService_Patch(t *testing.T) {
	server, _ := initTestServer()
	defer server.Stop()

	ctx := context.Background()

	//nolint:staticcheck
	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(bufDialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("error closing connection: %v", err)
		}
	}()

	client := containersv1.NewContainerServiceClient(conn)
	if err := initDB(ctx, client); err != nil {
		log.Fatalf("error initializing DB: %v", err)
	}

	testCases := []struct {
		name   string
		expect *containersv1.Container
		patch  *containersv1.Container
	}{
		{
			name: "should only update container image",
			patch: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-1",
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/nginx:v1.27.2",
				},
			},
			expect: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-1",
					Labels: map[string]string{
						"team":        "backend",
						"environment": "production",
						"role":        "root",
					},
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/nginx:v1.27.2",
					PortMappings: []*containersv1.PortMapping{
						{
							Name:          "http",
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
						"--config /mnt/cfg/config.yaml",
					},
					Mounts: []*containersv1.Mount{
						{
							Name:        "temp",
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
				Status: &containersv1.Status{},
			},
		},
		{
			name: "should add to environment variables",
			patch: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-2",
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/nginx:latest",
					Envvars: []*containersv1.EnvVar{
						{
							Name:  "HTTPS_PROXY",
							Value: "proxy.a.b:8443",
						},
						{
							Name:  "HTTP_PROXY",
							Value: "new-proxy.foo.com:8080",
						},
					},
				},
			},
			expect: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-2",
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
							Name:          "http",
							HostPort:      8080,
							ContainerPort: 80,
							Protocol:      "TCP",
						},
					},
					Envvars: []*containersv1.EnvVar{
						{
							Name:  "HTTP_PROXY",
							Value: "new-proxy.foo.com:8080",
						},
						{
							Name:  "HTTPS_PROXY",
							Value: "proxy.a.b:8443",
						},
					},
					Args: []string{
						"--config /mnt/cfg/config.yaml",
					},
					Mounts: []*containersv1.Mount{
						{
							Name:        "temp",
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
				Status: &containersv1.Status{},
			},
		},
		{
			name: "should replace volume mounts",
			patch: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-3",
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/nginx:latest",
					Mounts: []*containersv1.Mount{
						{
							Name:        "temp",
							Source:      "/var/lib/nginx/config",
							Destination: "/etc/nginx/config.d",
						},
					},
				},
			},
			expect: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-3",
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
							Name:          "http",
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
						"--config /mnt/cfg/config.yaml",
					},
					Mounts: []*containersv1.Mount{
						{
							Name:        "temp",
							Source:      "/var/lib/nginx/config",
							Destination: "/etc/nginx/config.d",
						},
					},
					NodeSelector: map[string]string{
						"blipblop.io/arch": "amd64",
						"blipblop.io/os":   "linux",
					},
				},
				Status: &containersv1.Status{},
			},
		},
		{
			name: "should add label to node selector",
			patch: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-4",
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/nginx:latest",
					NodeSelector: map[string]string{
						"blipblop.io/unschedulable": "true",
					},
				},
			},
			expect: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-4",
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
							Name:          "http",
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
						"--config /mnt/cfg/config.yaml",
					},
					Mounts: []*containersv1.Mount{
						{
							Name:        "temp",
							Source:      "/tmp",
							Destination: "/mnt/tmp",
							Type:        "bind",
						},
					},
					NodeSelector: map[string]string{
						"blipblop.io/arch":          "amd64",
						"blipblop.io/os":            "linux",
						"blipblop.io/unschedulable": "true",
					},
				},
				Status: &containersv1.Status{},
			},
		},
		{
			name: "should replace args",
			patch: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-5",
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/nginx:latest",
					Args:  []string{"--config /mnt/cfg/config.yaml", "--log-level debug", "--insecure-skip-verify"},
				},
			},
			expect: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-5",
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
							Name:          "http",
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
						"--config /mnt/cfg/config.yaml",
						"--log-level debug",
						"--insecure-skip-verify",
					},
					Mounts: []*containersv1.Mount{
						{
							Name:        "temp",
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
				Status: &containersv1.Status{},
			},
		},
		{
			name: "should add to port mappings",
			patch: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-6",
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/nginx:latest",
					PortMappings: []*containersv1.PortMapping{
						{
							Name:          "metrics",
							ContainerPort: 9000,
						},
						{
							Name:          "http",
							HostPort:      8088,
							ContainerPort: 80,
							Protocol:      "TCP",
						},
						{
							Name:          "https",
							ContainerPort: 8443,
						},
					},
				},
			},
			expect: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-6",
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
							Name:          "http",
							HostPort:      8088,
							ContainerPort: 80,
							Protocol:      "TCP",
						},
						{
							Name:          "metrics",
							ContainerPort: 9000,
						},
						{
							Name:          "https",
							ContainerPort: 8443,
						},
					},
					Envvars: []*containersv1.EnvVar{
						{
							Name:  "HTTP_PROXY",
							Value: "proxy.foo.com",
						},
					},
					Args: []string{
						"--config /mnt/cfg/config.yaml",
					},
					Mounts: []*containersv1.Mount{
						{
							Name:        "temp",
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
				Status: &containersv1.Status{},
			},
		},
		{
			name: "should add to empty args field",
			patch: &containersv1.Container{
				Meta: &types.Meta{
					Name: "bare-container",
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/nginx:latest",
				},
			},
			expect: &containersv1.Container{
				Meta: &types.Meta{
					Name: "bare-container",
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/nginx:latest",
				},
				Status: &containersv1.Status{},
			},
		},
		{
			name: "should only update status",
			patch: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-7",
				},
				Config: &containersv1.Config{
					Image: "docker.io/library/nginx:latest",
				},
				Status: &containersv1.Status{
					Phase: wrapperspb.String("RUNNING"),
				},
			},
			expect: &containersv1.Container{
				Meta: &types.Meta{
					Name: "test-container-7",
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
							Name:          "http",
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
						"--config /mnt/cfg/config.yaml",
					},
					Mounts: []*containersv1.Mount{
						{
							Name:        "temp",
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
				Status: &containersv1.Status{
					Phase: wrapperspb.String("RUNNING"),
				},
			},
		},
	}

	// Run tests
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			req := &containersv1.PatchRequest{Id: tt.patch.Meta.Name, Container: tt.patch}
			res, err := client.Patch(ctx, req)
			if err != nil {
				t.Fatal("error updating container", err)
			}

			exp := tt.expect
			// Ignore timestamps
			res.Container.Meta.Created = nil
			if !proto.Equal(exp, res.Container) {
				t.Errorf("\ngot:\n%v\nwant:\n%v", res.Container, exp)
			}
		})
	}
}
