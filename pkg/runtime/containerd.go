package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/networking"
	"github.com/amimof/blipblop/pkg/util"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	ccontainers "github.com/containerd/containerd/containers"
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

func appendContainerLabels(l labels.Label, container *containers.Container) labels.Label {
	pm := container.GetConfig().GetPortMappings()
	b, _ := json.Marshal(&pm)
	l.Set("blipblop/revision", util.Uint64ToString(container.GetMeta().GetRevision()))
	l.Set("blipblop/created", container.GetMeta().GetCreated().String())
	l.Set("blipblop/updated", container.GetMeta().GetUpdated().String())
	l.Set("blipblop/name", container.GetMeta().GetName())
	l.Set("blipblop/namespace", "blipblop")
	l.Set("blipblop/ports", string(b))
	return l
}

// GC performs any tasks necessary to clean up the environment from danglig configuration. Such as tearing down the network
func (c *ContainerdRuntime) GC(ctx context.Context, ctr *containers.Container) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")

	// Tear down CNI network
	mappings := networking.ParseCNIPortMappings(ctr.GetConfig().GetPortMappings()...)
	cniLabels := labels.New()
	cniLabels.Set("IgnoreUnknown", "1")
	err := networking.DeleteCNINetwork(ctx, c.cni, ctr.GetMeta().GetName(), ctr.GetStatus().GetPid(), gocni.WithLabels(cniLabels), gocni.WithCapabilityPortMap(mappings))
	if err != nil {
		return err
	}

	return c.Delete(ctx, ctr.GetMeta().GetName())
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

		fmt.Println(l)

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

func withPreStopHook(hooks *specs.Hooks) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *ccontainers.Container, s *oci.Spec) error {
		if s.Hooks == nil {
			s.Hooks = &specs.Hooks{}
		}
		s.Hooks.Poststop = hooks.Poststop
		return nil
	}
}

func (c *ContainerdRuntime) Create(ctx context.Context, ctr *containers.Container) error {
	ns := "blipblop"
	ctx = namespaces.WithNamespace(ctx, ns)
	image, err := c.client.Pull(ctx, ctr.Config.Image, containerd.WithPullUnpack)
	if err != nil {
		return err
	}

	// Configure labels
	l := appendContainerLabels(labels.New(), ctr)
	m := buildMounts(ctr.GetConfig().GetMounts())

	// Build OCI specification
	opts := buildSpec(ctr.GetConfig().GetEnvvars(), m, ctr.GetConfig().GetArgs())
	opts = append(opts,
		oci.WithHostname(ctr.GetMeta().GetName()),
		oci.WithImageConfig(image),
		oci.WithEnv(ctr.GetConfig().GetEnvvars()),
	)

	// Create container
	_, err = c.client.NewContainer(
		ctx,
		ctr.GetMeta().GetName(),
		containerd.WithNewSnapshot(fmt.Sprintf("%s-snapshot", ctr.GetMeta().GetName()), image),
		containerd.WithNewSpec(opts...),
		containerd.WithContainerLabels(l),
	)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerdRuntime) Delete(ctx context.Context, key string) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	container, err := c.client.LoadContainer(ctx, key)
	if err != nil {
		return err
	}
	task, _ := container.Task(ctx, nil)
	if task != nil {
		_, err = task.Delete(ctx)
		if err != nil {
			return err
		}
	}
	return container.Delete(ctx)
}

func (c *ContainerdRuntime) Kill(ctx context.Context, ctr *containers.Container) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	container, err := c.client.LoadContainer(ctx, ctr.GetMeta().GetName())
	if err != nil {
		return err
	}
	task, err := container.Task(ctx, nil)
	if errdefs.IsNotFound(err) {
		c.logger.Debug("skip kill, no task found for container", "container", container.ID())
		return nil
	}
	if err != nil {
		return err
	}

	// Tear down network
	mappings := networking.ParseCNIPortMappings(ctr.GetConfig().GetPortMappings()...)
	cniLabels := labels.New()
	cniLabels.Set("IgnoreUnknown", "1")
	err = networking.DeleteCNINetwork(ctx, c.cni, task.ID(), task.Pid(), gocni.WithLabels(cniLabels), gocni.WithCapabilityPortMap(mappings))
	if err != nil {
		return err
	}

	if err = task.Kill(ctx, syscall.SIGINT); err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("failed to send SIGINT: %w", err)
		}
		c.logger.Debug("skip kill, task already stopped", "container", container.ID(), "task", task.ID())
		return nil
	}

	wait, err := task.Wait(ctx)
	if err != nil {
		return err
	}

	select {
	case status := <-wait:
		c.logger.Info("task exited with status", "taskId", task.ID(), "exitCode", status.ExitCode())
		_, _ = task.Delete(ctx)
		return nil
	case <-time.After(time.Second * 60):
		return errors.New("timeout waiting for task to exit gracefully")
	}
}

func (c *ContainerdRuntime) Start(ctx context.Context, ctr *containers.Container) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	var container containerd.Container

	container, err := c.client.LoadContainer(ctx, ctr.GetMeta().GetName())
	if err != nil {
		if errdefs.IsNotFound(err) {
			c.logger.Debug("container not found, creating a new instance in runtime", "container", ctr.GetMeta().GetName())
			if err := c.Create(ctx, ctr); err != nil {
				return err
			}
			container, err = c.client.LoadContainer(ctx, ctr.GetMeta().GetName())
			if err != nil {
				return err
			}
		}
		return err
	}

	task, err := container.NewTask(ctx, cio.NewCreator())
	if errdefs.IsAlreadyExists(err) {
		c.logger.Debug("skip creation, task already exists for container", "container", container.ID())
		return nil
	}
	if err != nil {
		return err
	}

	// Start the task
	err = task.Start(ctx)
	if err != nil {
		return err
	}

	// Tear down network
	mappings := networking.ParseCNIPortMappings(ctr.GetConfig().GetPortMappings()...)
	cniLabels := labels.New()
	cniLabels.Set("IgnoreUnknown", "1")
	result, err := networking.CreateCNINetwork(ctx, c.cni, task.ID(), task.Pid(), gocni.WithLabels(cniLabels), gocni.WithCapabilityPortMap(mappings))
	if err != nil {
		return err
	}

	// Fill container labels with network information
	l := labels.New()
	ip := result.Interfaces["eth1"].IPConfigs[0].IP
	gw := result.Interfaces["eth1"].IPConfigs[0].Gateway
	mac := result.Interfaces["eth1"].Mac
	l.Set("blipblop.io/ip-address", ip.String())
	l.Set("blipblop.io/gateway", gw.String())
	l.Set("blipblop.io/mac", mac)

	_, err = container.SetLabels(ctx, l)
	if err != nil {
		return nil
	}

	// // Testing
	// go func() {
	// 	st, err := task.Wait(ctx)
	// 	if err != nil {
	// 		c.logger.Error("error waiting for task", "container", ctr.GetMeta().GetName(), "error", err)
	// 		return
	// 	}
	// 	status := <-st
	// 	defer func() {
	// 		c.logger.Info("task exited", "taskId", task.ID(), "container", ctr.GetMeta().GetName(), "exitCode", status.ExitCode(), "error", status.Error())
	// 		_, _ = task.Delete(ctx)
	//
	// 		c.logger.Info("tearing down network", "task", task.ID(), "container", ctr.GetMeta().GetName(), "pid", task.Pid())
	// 		err = networking.DeleteCNINetwork(ctx, c.cni, task.ID(), task.Pid(), gocni.WithLabels(cniLabels), gocni.WithCapabilityPortMap(mappings))
	// 		if err != nil {
	// 			c.logger.Error("error tearing down network", "task", task.ID(), "container", ctr.GetMeta().GetName(), "pid", task.Pid(), "error", err)
	// 		}
	// 	}()
	// }()

	return nil
}

func (c *ContainerdRuntime) IsServing(ctx context.Context) (bool, error) {
	return c.client.IsServing(ctx)
}

func NewContainerdRuntimeClient(client *containerd.Client, cni gocni.CNI, opts ...NewContainerdRuntimeOption) *ContainerdRuntime {
	runtime := &ContainerdRuntime{client: client, cni: cni, logger: logger.ConsoleLogger{}}

	for _, opt := range opts {
		opt(runtime)
	}

	return runtime
}
