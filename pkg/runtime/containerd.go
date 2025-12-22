package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/api/types/v1"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/logger"
	"github.com/amimof/blipblop/pkg/networking"
	"github.com/amimof/blipblop/pkg/util"
	"github.com/containerd/containerd/errdefs"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	gocni "github.com/containerd/go-cni"
	"github.com/opencontainers/runtime-spec/specs-go"
	"go.opentelemetry.io/otel"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	labelPrefix = "blipblop"
	logFileName = "stdout.log"
)

var tracer = otel.GetTracerProvider().Tracer("blipblop-node")

type ContainerdRuntime struct {
	client       *containerd.Client
	cni          networking.Manager
	logger       logger.Logger
	ns           string
	mu           sync.Mutex
	containerIOs map[string]*ContainerIO
	logDirFmt    string
}

type NewContainerdRuntimeOption func(c *ContainerdRuntime)

// WithLogDirFmt allows for setting the root directory for container log files (stdout).
// For example /var/lib/blipblop/containers/%s/log which happens to be the default location.
func WithLogDirFmt(rootPath string) NewContainerdRuntimeOption {
	return func(c *ContainerdRuntime) {
		c.logDirFmt = rootPath
	}
}

func WithLogger(l logger.Logger) NewContainerdRuntimeOption {
	return func(c *ContainerdRuntime) {
		c.logger = l
	}
}

// withMounts creates an oci compatible list of volume mounts to be used by the runtime.
// Default mount type is read-write bind mount if not otherwise specified.
func withMounts(m []*containers.Mount) oci.SpecOpts {
	var mounts []specs.Mount
	for _, mount := range m {
		mountType := mount.Type
		mountOptions := mount.Options
		if mountType == "" {
			mountType = "bind"
		}
		if len(mountOptions) == 0 {
			mountOptions = []string{"rbind", "rw"}
		}
		mounts = append(mounts, specs.Mount{
			Destination: mount.Destination,
			Type:        mountType,
			Source:      mount.Source,
			Options:     mountOptions,
		})
	}
	return oci.WithMounts(mounts)
}

func withEnvVars(envs []*containers.EnvVar) oci.SpecOpts {
	envVars := make([]string, len(envs))
	for i, env := range envs {
		envVars[i] = fmt.Sprintf("%s=%s", env.GetName(), env.GetValue())
	}
	return oci.WithEnv(envVars)
}

func WithNamespace(ns string) NewContainerdRuntimeOption {
	return func(c *ContainerdRuntime) {
		c.ns = ns
	}
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

	// Convert to CNI type once here
	cniPorts := networking.ParseCNIPortMappings(pm...)

	// Store CNI mappings, not API mappings
	b, _ := json.Marshal(&cniPorts)

	// Fill label set with values
	l.Set("blipblop/revision", util.Uint64ToString(container.GetMeta().GetRevision()))
	l.Set("blipblop/created", container.GetMeta().GetCreated().String())
	l.Set("blipblop/updated", container.GetMeta().GetUpdated().String())
	l.Set("blipblop/name", container.GetMeta().GetName())
	l.Set("blipblop/namespace", "blipblop")
	l.Set("blipblop/ports", string(b))

	return containerd.WithContainerLabels(l)
}

// Namespace returns the namespace used for runnning workload. If supported by the runtime
func (c *ContainerdRuntime) Namespace() string {
	return c.ns
}

// Version returns the version of the runtime
func (c *ContainerdRuntime) Version(ctx context.Context) (string, error) {
	// c.client.
	ver, err := c.client.Version(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("containerd/%s", ver.Version), nil
}

// Cleanup performs any tasks necessary to clean up the environment from danglig configuration.
// Such as tearing down the network.
func (c *ContainerdRuntime) Cleanup(ctx context.Context, id string) error {
	ctx, span := tracer.Start(ctx, "runtime.containerd.Cleanup")
	defer span.End()

	ctx = namespaces.WithNamespace(ctx, c.ns)
	ctr, err := c.client.LoadContainer(ctx, id)
	if err != nil {
		return err
	}

	t, err := ctr.Task(ctx, nil)
	if err != nil {
		return err
	}

	// Get metadata from labels
	ctrLabels, err := ctr.Labels(ctx)
	if err != nil {
		return err
	}

	// Unmarshal ports label
	b := ctrLabels["blipblop/ports"]
	cniports := []gocni.PortMapping{}
	err = json.Unmarshal([]byte(b), &cniports)
	if err != nil {
		return err
	}

	c.logger.Debug("detaching cni network from container", "container", ctr.ID(), "pid", t.Pid(), "cniPorts", cniports)

	// Delete CNI Network
	err = c.cni.Detach(
		ctx,
		ctr.ID(),
		t.Pid(),
		gocni.WithCapabilityPortMap(cniports),
		gocni.WithArgs("IgnoreUnknown", "true"),
	)
	if err != nil {
		return err
	}

	logFile := path.Join(fmt.Sprintf(c.logDirFmt, ctr.ID()), logFileName)
	c.logger.Debug("cleaning log file", "container", ctr.ID(), "pid", t.Pid(), "logFile", logFile)
	return os.WriteFile(logFile, []byte{}, 0o666)
}

func (c *ContainerdRuntime) List(ctx context.Context) ([]*containers.Container, error) {
	ctx, span := tracer.Start(ctx, "runtime.containerd.List")
	defer span.End()

	ctx = namespaces.WithNamespace(ctx, c.ns)
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
	ctx, span := tracer.Start(ctx, "runtime.containerd.Get")
	defer span.End()

	ctx = namespaces.WithNamespace(ctx, c.ns)
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
	ctx, span := tracer.Start(ctx, "runtime.containerd.Pull")
	defer span.End()

	ctx = namespaces.WithNamespace(ctx, c.ns)
	_, err := c.client.Pull(ctx, ctr.Config.Image, containerd.WithPullUnpack)
	if err != nil {
		return err
	}
	return nil
}

// Delete deletes the container and any tasks associated with it.
// Tasks will be forcefully stopped if running.
func (c *ContainerdRuntime) Delete(ctx context.Context, ctr *containers.Container) error {
	ctx, span := tracer.Start(ctx, "runtime.containerd.Delete")
	defer span.End()

	ctx = namespaces.WithNamespace(ctx, c.ns)

	// Get the container from runtime. If container isn't found, then assume that it's already been deleted
	container, err := c.client.LoadContainer(ctx, ctr.GetMeta().GetName())
	if err != nil {
		if errdefs.IsNotFound(err) {
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
		if err != nil && !errdefs.IsNotFound(err) {
			return err
		}
	}

	// Clean up IO streams
	c.mu.Lock()
	if io, exists := c.containerIOs[ctr.GetMeta().GetName()]; exists {
		if io.Stdout != nil {
			_ = io.Stdout.Close()
		}
		if io.Stderr != nil {
			_ = io.Stderr.Close()
		}
		delete(c.containerIOs, ctr.GetMeta().GetName())
	}
	c.mu.Unlock()

	// Delete the container
	err = container.Delete(ctx, containerd.WithSnapshotCleanup)
	if err != nil {
		return fmt.Errorf("error deleting container and its tasks: %v", err)
	}

	return nil
}

// Stop stops containers associated with the name of provided container instance.
// Stop will attempt to gracefully stop tasks, but will eventually do it forcefully
// if timeout is reached. Stop does not perform any garbage colletion and it is
// up to the caller to call Cleanup() after calling Stop()
func (c *ContainerdRuntime) Stop(ctx context.Context, ctr *containers.Container) error {
	ctx, span := tracer.Start(ctx, "runtime.containerd.Stop")
	defer span.End()

	ctx = namespaces.WithNamespace(ctx, c.ns)
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
		return fmt.Errorf("failed to kill task gracefully %w", err)
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
			return fmt.Errorf("failed to get exit status from task %w", err)
		}
	case err = <-timeoutChan:
		if err != nil {
			return fmt.Errorf("got error waiting for tsk to finish %w", err)
		}
	}

	// Delete the task
	if _, err := task.Delete(ctx); err != nil {
		return err
	}

	return nil
}

// Kill forcefully stops the container and tasks within by sending SIGKILL to the process.
// Like Stop(), Kill() does not perform garbage collection. Use Gleanup() for this.
func (c *ContainerdRuntime) Kill(ctx context.Context, ctr *containers.Container) error {
	ctx, span := tracer.Start(ctx, "runtime.containerd.Kill")
	defer span.End()

	ctx = namespaces.WithNamespace(ctx, c.ns)

	cont, err := c.client.LoadContainer(ctx, ctr.GetMeta().GetName())
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}

	task, err := cont.Task(ctx, cio.Load)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}

	// Attempt to forcefully kill the task
	if err = task.Kill(ctx, syscall.SIGKILL); err != nil {
		if !errdefs.IsNotFound(err) {
			return err
		}
	}

	// Wait
	exitChan, err := task.Wait(ctx)
	if err != nil {
		return err
	}

	// Handle exit status
	select {
	case exitStatus := <-exitChan:
		c.logger.Info("task exited with status", "container", cont.ID(), "task", task.ID(), "exitStatus", exitStatus.ExitCode())
	case <-time.After(10 * time.Second):
		c.logger.Info("deadline exceeded waiting for task to exit", "container", cont.ID(), "task", task.ID())
		return os.ErrDeadlineExceeded
	}

	// Delete the task
	_, err = task.Delete(ctx)
	return err
}

func (c *ContainerdRuntime) Run(ctx context.Context, ctr *containers.Container) error {
	ctx, span := tracer.Start(ctx, "runtime.containerd.Run")
	defer span.End()

	ctx = namespaces.WithNamespace(ctx, c.ns)
	containerID := ctr.GetMeta().GetName()

	// Get the image. Assumes that image has been pulled beforehand
	image, err := c.client.GetImage(ctx, ctr.GetConfig().GetImage())
	if err != nil {
		return err
	}

	// Build OCI specification
	opts := []oci.SpecOpts{
		oci.WithDefaultSpec(),
		oci.WithDefaultUnixDevices,
		oci.WithImageConfig(image),
		oci.WithHostname(containerID),
		oci.WithImageConfig(image),
		withEnvVars(ctr.GetConfig().GetEnvvars()),
		withMounts(ctr.GetConfig().GetMounts()),
	}

	// Add args opts
	if len(ctr.GetConfig().GetArgs()) > 0 {
		opts = append(opts, oci.WithProcessArgs(ctr.GetConfig().GetArgs()...))
	}

	// Create container
	cont, err := c.client.NewContainer(
		ctx,
		containerID,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(fmt.Sprintf("%s-snapshot", containerID), image),
		containerd.WithNewSpec(opts...),
		withContainerLabels(labels.New(), ctr),
	)
	if err != nil {
		return err
	}

	// Pipe stdout and stderr to log file on disk
	logRoot := fmt.Sprintf(c.logDirFmt, containerID)
	stdOut := filepath.Join(logRoot, logFileName)
	ioCreator := cio.LogFile(stdOut)

	// Create task
	task, err := cont.NewTask(ctx, ioCreator)
	if err != nil {
		return err
	}

	// ctr.GetConfig().GetPortMappings()
	pm := networking.ParseCNIPortMappings(ctr.GetConfig().PortMappings...)

	// Setup networking
	attachOpts := []gocni.NamespaceOpts{gocni.WithCapabilityPortMap(pm), gocni.WithArgs("IgnoreUnknown", "true")}
	err = c.cni.Attach(ctx, cont.ID(), task.Pid(), attachOpts...)
	if err != nil {
		return err
	}

	// Start the  task
	err = task.Start(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerdRuntime) Labels(ctx context.Context) (labels.Label, error) {
	return nil, nil
}

func (c *ContainerdRuntime) IO(ctx context.Context, containerID string) (*ContainerIO, error) {
	logRoot := fmt.Sprintf(c.logDirFmt, containerID)
	stdOut := filepath.Join(logRoot, logFileName)

	f, err := os.OpenFile(stdOut, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	return &ContainerIO{Stdout: f}, nil
}

func NewContainerdRuntimeClient(client *containerd.Client, cni networking.Manager, opts ...NewContainerdRuntimeOption) *ContainerdRuntime {
	runtime := &ContainerdRuntime{
		client:       client,
		cni:          cni,
		logger:       logger.ConsoleLogger{},
		ns:           DefaultNamespace,
		containerIOs: map[string]*ContainerIO{},
		logDirFmt:    "/var/lib/blipblop/containers/%s/log",
	}

	for _, opt := range opts {
		opt(runtime)
	}

	return runtime
}
