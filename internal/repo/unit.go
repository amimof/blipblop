package repo

import (
	"context"
	"errors"
	"fmt"
	"github.com/amimof/blipblop/internal/models"
	"github.com/amimof/blipblop/pkg/cache"
	"github.com/amimof/blipblop/pkg/event"
	"github.com/amimof/blipblop/pkg/labels"
	"github.com/amimof/blipblop/pkg/networking"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	gocni "github.com/containerd/go-cni"
	"github.com/google/uuid"
	"github.com/opencontainers/runtime-spec/specs-go"
	"log"
	"syscall"
	"time"
)

type UnitRepo interface {
	GetAll(ctx context.Context) ([]*models.Unit, error)
	Get(ctx context.Context, key string) (*models.Unit, error)
	Set(ctx context.Context, unit *models.Unit) error
	Delete(ctx context.Context, key string) error
	Start(ctx context.Context, key string) error
	Stop(ctx context.Context, key string) error
	Kill(ctx context.Context, key string) error
}

// containerdRepo is a live containerd environment. Data is stored and fetched in and from the contaienrd runtime
type containerdRepo struct {
	client *containerd.Client
	cni    gocni.CNI
}

type inmemUnitRepo struct {
	cache *cache.Cache
}

func (i *inmemUnitRepo) GetAll(ctx context.Context) ([]*models.Unit, error) {
	return nil, nil
}
func (i *inmemUnitRepo) Get(ctx context.Context, key string) (*models.Unit, error) {
	return nil, nil
}
func (i *inmemUnitRepo) Set(ctx context.Context, unit *models.Unit) error {
	return nil
}
func (i *inmemUnitRepo) Delete(ctx context.Context, key string) error {
	return nil
}
func (i *inmemUnitRepo) Start(ctx context.Context, key string) error {
	return nil
}
func (i *inmemUnitRepo) Stop(ctx context.Context, key string) error {
	return nil
}
func (i *inmemUnitRepo) Kill(ctx context.Context, key string) error {
	return nil
}

func (u *containerdRepo) GetAll(ctx context.Context) ([]*models.Unit, error) {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	containers, err := u.client.Containers(ctx)
	if err != nil {
		return nil, err
	}
	var result = make([]*models.Unit, len(containers))
	for i, c := range containers {
		l, err := parseContainerLabels(ctx, c)
		if err != nil {
			return nil, err
		}
		info, err := c.Info(ctx, containerd.WithoutRefreshedMetadata)
		if err != nil {
			return nil, err
		}
		var netstatus models.NetworkStatus
		var runstatus models.RuntimeStatus
		if task, _ := c.Task(ctx, nil); task != nil {
			if ip, _ := networking.GetIPAddress(fmt.Sprintf("%s-%d", task.ID(), task.Pid())); ip != nil {
				netstatus.IP = ip
			}
			if status, _ := task.Status(ctx); &status != nil {
				runstatus.Status = status.Status
			}
		}
		result[i] = &models.Unit{
			Name:   &info.ID,
			Image:  &info.Image,
			Labels: l,
			Status: &models.Status{
				Network: &netstatus,
				Runtime: &runstatus,
			},
		}
	}
	return result, nil
}

func parseContainerLabels(ctx context.Context, container containerd.Container) (labels.Label, error) {
	info, err := container.Labels(ctx)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (u *containerdRepo) Get(ctx context.Context, key string) (*models.Unit, error) {
	units, err := u.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	for _, unit := range units {
		if key == *unit.Name {
			return unit, nil
		}
	}
	return nil, nil
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

func (u *containerdRepo) Set(ctx context.Context, unit *models.Unit) error {
	ns := "blipblop"
	ctx = namespaces.WithNamespace(ctx, ns)
	image, err := u.client.Pull(ctx, *unit.Image, containerd.WithPullUnpack)
	if err != nil {
		return err
	}

	// Configure labels
	labels := labels.New()
	labels.Set("blipblop.io/uuid", uuid.New().String())

	// Build OCI specification
	opts := buildSpec(unit.EnvVars, unit.Mounts, unit.Args)
	opts = append(opts, oci.WithHostname(*unit.Name))
	opts = append(opts, oci.WithImageConfig(image))

	// Create container
	container, err := u.client.NewContainer(
		ctx,
		*unit.Name,
		containerd.WithNewSnapshot(fmt.Sprintf("%s-snapshot", *unit.Name), image),
		containerd.WithNewSpec(opts...),
		containerd.WithContainerLabels(labels),
	)
	if err != nil {
		return err
	}

	log.Printf("Created container '%s'", container.ID())
	event.MustFire("container-create")
	return nil
}

func (u *containerdRepo) Delete(ctx context.Context, key string) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	container, err := u.client.LoadContainer(ctx, key)
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

func (u *containerdRepo) Kill(ctx context.Context, key string) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	container, err := u.client.LoadContainer(ctx, key)
	if err != nil {
		return err
	}
	task, err := container.Task(ctx, nil)
	if err != nil {
		return err
	}
	return task.Kill(ctx, syscall.SIGINT)
}

func (u *containerdRepo) Stop(ctx context.Context, key string) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	container, err := u.client.LoadContainer(ctx, key)
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
	err = networking.DeleteCNINetwork(ctx, u.cni, task, cniLabels)
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

func (u *containerdRepo) Start(ctx context.Context, key string) error {
	ctx = namespaces.WithNamespace(ctx, "blipblop")
	container, err := u.client.LoadContainer(ctx, key)
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
	result, err := networking.CreateCNINetwork(ctx, u.cni, task, cniLabels)
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

func NewUnitRepo(client *containerd.Client, cni gocni.CNI) UnitRepo {
	return &containerdRepo{client, cni}
}

func NewInMemUnitRepo() UnitRepo {
	c := cache.New()
	c.TTL = time.Hour * 24
	return &inmemUnitRepo{
		cache: c,
	}
}
