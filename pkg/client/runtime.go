package client

import (
	"context"
	"errors"
	"fmt"
	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/event"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/networking"
	"github.com/amimof/blipblop/pkg/util"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	gocni "github.com/containerd/go-cni"
	"github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/protobuf/types/known/timestamppb"
	"log"
	"syscall"
	"time"
)

const labelPrefix = "blipblop.io/"

type RuntimeClient struct {
	client *containerd.Client
	cni    gocni.CNI
}

func buildMounts(m []*containers.Mount) []specs.Mount {
	var mounts []specs.Mount
	for _, mount := range m {
		mounts = append(mounts, specs.Mount{
			Destination: mount.Destination,
			Type:        mount.Type,
			Source:      mount.Source,
			Options:     mount.Options,
		})
	}
	return mounts
}

func buildSpec(envs []string, mounts []specs.Mount, args []string) []oci.SpecOpts {
	var opts []oci.SpecOpts
	if len(envs) > 0 {
		opts = append(opts, oci.WithEnv(envs))
	}
	if len(mounts) > 0 {
		opts = append(opts, oci.WithMounts(mounts))
	}
	if len(args) > 0 {
		opts = append(opts, oci.WithProcessArgs(args...))
	}
	return opts
}

func parseContainerLabels(ctx context.Context, container containerd.Container) (labels.Label, error) {
	info, err := container.Labels(ctx)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func buildContainerLabels(container *containers.Container) labels.Label {
	l := labels.New()
	l.Set(fmt.Sprintf("%s%s", labelPrefix, "revision"), util.Uint64ToString(container.Revision))
	l.Set(fmt.Sprintf("%s%s", labelPrefix, "created"), container.Created.String())
	l.Set(fmt.Sprintf("%s%s", labelPrefix, "updated"), container.Updated.String())
	l.AppendMap(container.Labels)
	return l
}

func (c *RuntimeClient) List(ctx context.Context) ([]*containers.Container, error) {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	ctrs, err := c.client.Containers(ctx)
	if err != nil {
		return nil, err
	}
	var result = make([]*containers.Container, len(ctrs))
	for i, c := range ctrs {
		l, err := parseContainerLabels(ctx, c)
		if err != nil {
			return nil, err
		}
		info, err := c.Info(ctx, containerd.WithoutRefreshedMetadata)
		if err != nil {
			return nil, err
		}
		// var netstatus models.NetworkStatus
		// var runstatus models.RuntimeStatus
		// if task, _ := c.Task(ctx, nil); task != nil {
		// 	if ip, _ := networking.GetIPAddress(fmt.Sprintf("%s-%d", task.ID(), task.Pid())); ip != nil {
		// 		netstatus.IP = ip
		// 	}
		// 	if status, _ := task.Status(ctx); &status != nil {
		// 		runstatus.Status = status.Status
		// 	}
		// }
		result[i] = &containers.Container{
			Name:     info.ID,
			Revision: util.StringToUint64(*l.Get(fmt.Sprintf("%s%s", labelPrefix, "revision"))),
			Created:  timestamppb.New(util.StringToTimestamp(*l.Get(fmt.Sprintf("%s%s", labelPrefix, "created")))),
			Updated:  timestamppb.New(util.StringToTimestamp(*l.Get(fmt.Sprintf("%s%s", labelPrefix, "updated")))),
			Config: &containers.Config{
				Image: info.Image,
			},
		}
	}
	return result, nil
}

func (c *RuntimeClient) Get(ctx context.Context, key string) (*containers.Container, error) {
	units, err := c.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, unit := range units {
		if key == unit.Name {
			return unit, nil
		}
	}
	return nil, nil
}

func (c *RuntimeClient) Set(ctx context.Context, unit *containers.Container) error {
	ns := "blipblop"
	ctx = namespaces.WithNamespace(ctx, ns)
	image, err := c.client.Pull(ctx, unit.Config.Image, containerd.WithPullUnpack)
	if err != nil {
		return err
	}

	// Configure labels
	l := buildContainerLabels(unit)
	m := buildMounts(unit.Config.Mounts)

	// Build OCI specification
	opts := buildSpec(unit.Config.Envvars, m, unit.Config.Args)
	opts = append(opts, oci.WithHostname(unit.Name))
	opts = append(opts, oci.WithImageConfig(image))

	// Create container
	container, err := c.client.NewContainer(
		ctx,
		unit.Name,
		containerd.WithNewSnapshot(fmt.Sprintf("%s-snapshot", unit.Name), image),
		containerd.WithNewSpec(opts...),
		containerd.WithContainerLabels(l),
	)
	if err != nil {
		return err
	}

	log.Printf("Created container '%s'", container.ID())
	event.MustFire("container-create")
	return nil
}

func (c *RuntimeClient) Delete(ctx context.Context, key string) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	container, err := c.client.LoadContainer(ctx, key)
	if err != nil {
		return err
	}
	task, _ := container.Task(ctx, nil)
	if task != nil {
		_, err = task.Delete(ctx)
		if err != nil {
			log.Printf("Can't delete task for container %s: %s", key, err)
		}
	}
	return container.Delete(ctx)
}

func (c *RuntimeClient) Kill(ctx context.Context, key string) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	container, err := c.client.LoadContainer(ctx, key)
	if err != nil {
		return err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}
	return task.Kill(ctx, syscall.SIGINT)
}

func (c *RuntimeClient) Stop(ctx context.Context, key string) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	container, err := c.client.LoadContainer(ctx, key)
	if err != nil {
		return err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}
	// Tear down network
	cniLabels := labels.New()
	cniLabels.Set("IgnoreUnknown", "1")
	err = networking.DeleteCNINetwork(ctx, c.cni, task, cniLabels)
	if err != nil {
		log.Printf("Unable to tear down network: %s", err)
	}
	err = task.Kill(ctx, syscall.SIGINT)
	if err != nil {
		return err
	}
	wait, err := task.Wait(ctx)
	select {
	case status := <-wait:
		log.Printf("Task %s exited with status %d", task.ID(), status.ExitCode())
		_, err = task.Delete(ctx)
		if err != nil {
			return err
		}
		return nil
	case <-time.After(time.Second * 60):
		return errors.New("Timeout waiting for task to exit gracefully")
	}
}

func (c *RuntimeClient) Start(ctx context.Context, key string) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	container, err := c.client.LoadContainer(ctx, key)
	if err != nil {
		return err
	}

	task, err := container.NewTask(ctx, cio.NewCreator())
	if err != nil {
		return err
	}
	// Setup network for namespace.
	cniLabels := labels.New()
	cniLabels.Set("IgnoreUnknown", "1")
	result, err := networking.CreateCNINetwork(ctx, c.cni, task, cniLabels)
	if err != nil {
		return err
	}

	// Fill container labels with network information
	ip := result.Interfaces["eth1"].IPConfigs[0].IP
	gw := result.Interfaces["eth1"].IPConfigs[0].Gateway
	mac := result.Interfaces["eth1"].Mac
	containerLabels := labels.New()
	containerLabels.Set("blipblop.io/ip-address", ip.String())
	containerLabels.Set("blipblop.io/gateway", gw.String())
	containerLabels.Set("blipblop.io/mac", mac)
	_, err = container.SetLabels(ctx, containerLabels)
	if err != nil {
		return nil
	}

	// Start the task
	err = task.Start(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r *RuntimeClient) ContainerdClient() *containerd.Client {
	return r.client
}

func NewContainerdRuntimeClient(client *containerd.Client, cni gocni.CNI) *RuntimeClient {
	return &RuntimeClient{client, cni}
}
