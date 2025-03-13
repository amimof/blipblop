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
		_, err := c.Create(ctx, &containersv1.CreateContainerRequest{Container: ctr})
		if err != nil {
			return err
		}
	}

	return nil
}

type genericContainer struct {
	*containersv1.Container
}

type createOpts func(*containersv1.Container)

func withImage(i string) createOpts {
	return func(c *containersv1.Container) {
		c.Config.Image = i
	}
}

func withEnvVar(ev ...*containersv1.EnvVar) createOpts {
	return func(c *containersv1.Container) {
		c.Config.Envvars = ev
	}
}

func withPortMapping(pm ...*containersv1.PortMapping) createOpts {
	return func(c *containersv1.Container) {
		c.Config.PortMappings = pm
	}
}

func withArgs(args []string) createOpts {
	return func(c *containersv1.Container) {
		c.Config.Args = args
	}
}

func withMounts(mounts ...*containersv1.Mount) createOpts {
	return func(c *containersv1.Container) {
		c.Config.Mounts = mounts
	}
}

func withNodeSelector(key, val string) createOpts {
	return func(c *containersv1.Container) {
		c.Config.NodeSelector = make(map[string]string)
		c.Config.NodeSelector[key] = val
	}
}

// createPatch creates a base update request. Add additional fields using createOpts
func createPatch(name, image string, opts ...createOpts) *containersv1.UpdateContainerRequest {
	ctr := &containersv1.Container{
		Meta: &types.Meta{
			Name: name,
		},
		Config: &containersv1.Config{
			Image: image,
		},
	}

	for _, opt := range opts {
		opt(ctr)
	}

	return &containersv1.UpdateContainerRequest{Container: ctr, Id: name}
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
		name string
		// container string
		expect *containersv1.Container
		input  *containersv1.UpdateContainerRequest
		// patch     []createOpts
		patch *containersv1.Container
	}{
		{
			name: "should only update container image",
			// container: "test-container-1",
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
			},
		},
		{
			name: "should add to environment variables",
			// container: "test-container-2",
			// patch: []createOpts{
			// 	withImage("docker.io/library/nginx:latest"),
			// 	withEnvVar(
			// 		&containersv1.EnvVar{Name: "HTTPS_PROXY", Value: "proxy.a.b:8443"},
			// 		&containersv1.EnvVar{Name: "HTTP_PROXY", Value: "new-proxy.foo.com:8080"},
			// 	),
			// },
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
			},
		},
		{
			name: "should replace volume mounts",
			// container: "test-container-3",
			// patch: []createOpts{
			// 	withImage("docker.io/library/nginx:latest"),
			// 	withMounts(&containersv1.Mount{Name: "temp", Destination: "/etc/nginx/config.d", Source: "/var/lib/nginx/config"}),
			// },
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
			},
		},
		{
			name: "should add label to node selector",
			// container: "test-container-4",
			// patch: []createOpts{
			// 	withImage("docker.io/library/nginx:latest"),
			// 	withNodeSelector("blipblop.io/unschedulable", "true"),
			// },
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
			},
		},
		{
			name: "should replace args",
			// container: "test-container-5",
			// patch: []createOpts{
			// 	withImage("docker.io/library/nginx:latest"),
			// 	withArgs([]string{"--log-level debug", "--insecure-skip-verify"}),
			// },
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
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// req := createPatch(tt.container, "", tt.patch...)
			req := &containersv1.UpdateContainerRequest{Id: tt.patch.Meta.Name, Container: tt.patch}
			res, err := client.Update(ctx, req)
			if err != nil {
				t.Fatal("error updating container", err)
			}

			// exp := createExpectedResponse(tt.container, tt.patch...)
			exp := tt.expect
			// Ignore timestamps
			res.Container.Meta.Created = nil
			if !proto.Equal(exp, res.Container) {
				t.Errorf("\ngot:\n%v\nwant:\n%v", res.Container, exp)
			}
		})
	}
}
