package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/networking"
	"github.com/amimof/blipblop/pkg/util"
	"github.com/containerd/console"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	gocni "github.com/containerd/go-cni"
	"github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const labelPrefix = "blipblop"

type ContainerdRuntime struct {
	client *containerd.Client
	cni    gocni.CNI
	logger logger.Logger
}

type NewContainerdRuntimeOption func(c *ContainerdRuntime)

func WithLogger(l logger.Logger) NewContainerdRuntimeOption {
	return func(c *ContainerdRuntime) {
		c.logger = l
	}
}

func withMounts(m []*containers.Mount) oci.SpecOpts {
	var mounts []specs.Mount
	for _, mount := range m {
		mounts = append(mounts, specs.Mount{
			Destination: mount.Destination,
			Type:        mount.Type,
			Source:      mount.Source,
			Options:     mount.Options,
		})
	}
	return oci.WithMounts(mounts)
}

func parseContainerLabels(ctx context.Context, container containerd.Container) (labels.Label, error) {
	info, err := container.Labels(ctx)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func withContainerLabels(l labels.Label, container *containers.Container) containerd.NewContainerOpts {
	pm := container.GetConfig().GetPortMappings()
	b, _ := json.Marshal(&pm)
	l.Set("blipblop/revision", util.Uint64ToString(container.GetMeta().GetRevision()))
	l.Set("blipblop/created", container.GetMeta().GetCreated().String())
	l.Set("blipblop/updated", container.GetMeta().GetUpdated().String())
	l.Set("blipblop/name", container.GetMeta().GetName())
	l.Set("blipblop/namespace", "blipblop")
	l.Set("blipblop/ports", string(b))
	return containerd.WithContainerLabels(l)
}

// GC performs any tasks necessary to clean up the environment from danglig configuration. Such as tearing down the network
func (c *ContainerdRuntime) Cleanup(ctx context.Context, ctr *containers.Container) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")

	// Tear down CNI network
	mappings := networking.ParseCNIPortMappings(ctr.GetConfig().GetPortMappings()...)
	cniLabels := labels.New()
	cniLabels.Set("IgnoreUnknown", "1")
	err := networking.DeleteCNINetwork(ctx, c.cni, ctr.GetMeta().GetName(), ctr.GetStatus().GetPid(), gocni.WithLabels(cniLabels), gocni.WithCapabilityPortMap(mappings))
	if err != nil {
		return err
	}

	return c.Delete(ctx, ctr)
}

func (c *ContainerdRuntime) List(ctx context.Context) ([]*containers.Container, error) {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	ctrs, err := c.client.Containers(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*containers.Container, len(ctrs))
	for i, c := range ctrs {
		l, err := parseContainerLabels(ctx, c)
		if err != nil {
			return nil, err
		}
		info, err := c.Info(ctx, containerd.WithoutRefreshedMetadata)
		if err != nil {
			return nil, err
		}

		result[i] = &containers.Container{
			Meta: &types.Meta{
				Name:     info.ID,
				Revision: util.StringToUint64(l.Get(fmt.Sprintf("%s/%s", labelPrefix, "revision"))),
				Created:  timestamppb.New(util.StringToTimestamp(l.Get(fmt.Sprintf("%s/%s", labelPrefix, "created")))),
				Updated:  timestamppb.New(util.StringToTimestamp(l.Get(fmt.Sprintf("%s/%s", labelPrefix, "updated")))),
			},
			Config: &containers.Config{
				Image: info.Image,
			},
		}
	}
	return result, nil
}

func (c *ContainerdRuntime) Get(ctx context.Context, key string) (*containers.Container, error) {
	ctrs, err := c.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, ctr := range ctrs {
		if key == ctr.GetMeta().GetName() {
			return ctr, nil
		}
	}
	return nil, nil
}

func (c *ContainerdRuntime) Pull(ctx context.Context, ctr *containers.Container) error {
	ns := "blipblop"
	ctx = namespaces.WithNamespace(ctx, ns)
	_, err := c.client.Pull(ctx, ctr.Config.Image, containerd.WithPullUnpack)
	if err != nil {
		return err
	}
	return nil
}

// Delete deletes the container and any tasks associated with it.
// Tasks will be forcefully stopped if running.
func (c *ContainerdRuntime) Delete(ctx context.Context, ctr *containers.Container) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")

	// Get the container from runtime. If container isn't found, then assume that it's already been deleted
	container, err := c.client.LoadContainer(ctx, ctr.GetMeta().GetName())
	if err != nil {
		if errors.Is(err, errdefs.ErrNotFound) {
			return nil
		}
		return err
	}

	// Return any error that isn't NotFound
	task, err := container.Task(ctx, nil)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return err
		}
	}

	// Make sure to stop tasks before deleting container. Using Kill here which is a forceful operation
	if task != nil {
		err = c.Kill(ctx, ctr)
		if err != nil && !errors.Is(err, errdefs.ErrNotFound) {
			return err
		}
	}
	return container.Delete(ctx, containerd.WithSnapshotCleanup)
}

func (c *ContainerdRuntime) Stop(ctx context.Context, ctr *containers.Container) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	cont, err := c.client.LoadContainer(ctx, ctr.GetMeta().GetName())
	if err != nil {
		return err
	}

	task, err := cont.Task(ctx, cio.Load)
	if err != nil {
		return err
	}

	// Wait
	waitChan, err := task.Wait(ctx)
	if err != nil {
		return err
	}

	// Attempt gracefull shutdown
	if err = task.Kill(ctx, syscall.SIGTERM); err != nil {
		return err
	}

	timeoutChan := make(chan error)
	timer := time.AfterFunc(time.Second*30, func() {
		timeoutChan <- task.Kill(ctx, syscall.SIGQUIT)
	})

	// Wait for task to stop. Stop forcefully if timeout occurs
	select {
	case exitStatus := <-waitChan:
		timer.Stop()
		err = exitStatus.Error()
		if err != nil {
			return err
		}
	case err = <-timeoutChan:
		if err != nil {
			return err
		}
	}

	// Delete the task
	if _, err := task.Delete(ctx); err != nil {
		return err
	}

	return nil
}

func (c *ContainerdRuntime) Kill(ctx context.Context, ctr *containers.Container) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	cont, err := c.client.LoadContainer(ctx, ctr.GetMeta().GetName())
	if err != nil {
		if errors.Is(err, errdefs.ErrNotFound) {
			return nil
		}
		return err
	}

	task, err := cont.Task(ctx, cio.Load)
	if err != nil {
		return err
	}

	// Wait
	waitChan, err := task.Wait(ctx)
	if err != nil {
		return err
	}

	// Attempt to forcefully kill the task
	if err = task.Kill(ctx, syscall.SIGQUIT); err != nil {
		return err
	}

	<-waitChan

	// Delete the task
	_, err = task.Delete(ctx)
	return err
}

func (c *ContainerdRuntime) Run(ctx context.Context, ctr *containers.Container) error {
	ns := "blipblop"
	ctx = namespaces.WithNamespace(ctx, ns)

	// Get the image. Assumes that image has been pulled beforehand
	image, err := c.client.GetImage(ctx, ctr.GetConfig().GetImage())
	if err != nil {
		return err
	}

	// Build OCI specification
	opts := []oci.SpecOpts{
		oci.WithDefaultSpec(),
		oci.WithDefaultUnixDevices,
		oci.WithTTY,
		oci.WithImageConfig(image),
		oci.WithHostname(ctr.GetMeta().GetName()),
		oci.WithImageConfig(image),
		oci.WithEnv(ctr.GetConfig().GetEnvvars()),
		withMounts(ctr.GetConfig().GetMounts()),
	}

	// Add args opts
	if len(ctr.GetConfig().GetArgs()) > 0 {
		opts = append(opts, oci.WithProcessArgs(ctr.GetConfig().GetArgs()...))
	}

	// Create container
	cont, err := c.client.NewContainer(
		ctx,
		ctr.GetMeta().GetName(),
		containerd.WithImage(image),
		containerd.WithNewSnapshot(fmt.Sprintf("%s-snapshot", ctr.GetMeta().GetName()), image),
		containerd.WithNewSpec(opts...),
		withContainerLabels(labels.New(), ctr),
	)
	if err != nil {
		return err
	}

	con, _, err := console.NewPty()
	if err != nil {
		return err
	}
	defer con.Close()

	dummyReader, _, err := os.Pipe()
	if err != nil {
		return err
	}
	ioCreator := cio.NewCreator(cio.WithTerminal, cio.WithStreams(dummyReader, con, con))

	// Create the task
	task, err := cont.NewTask(ctx, ioCreator)
	if err != nil {
		return err
	}

	// Start the  task
	return task.Start(ctx)
}

func (c *ContainerdRuntime) Labels(ctx context.Context) (labels.Label, error) {
	return nil, nil
}

func NewContainerdRuntimeClient(client *containerd.Client, cni gocni.CNI, opts ...NewContainerdRuntimeOption) *ContainerdRuntime {
	runtime := &ContainerdRuntime{client: client, cni: cni, logger: logger.ConsoleLogger{}}

	for _, opt := range opts {
		opt(runtime)
	}

	return runtime
}
