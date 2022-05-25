package worker

import (
	"log"
	"fmt"
	"context"
	"os"
	"os/signal"
	"syscall"
	//"golang.org/x/sys/unix"
	//"github.com/containerd/console"
	"github.com/amimof/blipblop/pkg/api"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/namespaces"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

const (
	SOCKET = "/run/containerd/containerd.sock"
)

func main() {
	err := runTask()
	if err != nil {
		log.Fatal(err)
	}
}

func Run(work *api.Workload) error {
	// client, err := containerd.New(SOCKET)
	// if err != nil {
	// 	return err
	// }
	// defer client.Close()

	ctx := namespaces.WithNamespace(context.Background(), "blipblop")
	image, err := client.Pull(ctx, work.Image, containerd.WithPullUnpack)
	if err != nil {
		return err
	}
	log.Printf("Successfully pulled image: '%s'", image.Name())
	
	container, err := client.NewContainer(
		ctx,
		work.Name,
		containerd.WithNewSnapshot(fmt.Sprintf("%s-snapshot", work.Name), image),
		containerd.WithNewSpec(
			oci.WithImageConfig(image),
			//oci.WithProcessArgs("sleep", "120m"),
		),
	)
	if err != nil {
		return err
	}
	defer container.Delete(ctx, containerd.WithSnapshotCleanup)
	log.Printf("Created container '%s'", container.ID())

	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return err
	}
	defer task.Delete(ctx, containerd.WithProcessKill)
	log.Printf("Created task '%s' with PID %d", task.ID(), task.Pid())	

	err = task.Start(ctx)
	if err != nil {
		return err
	}

	status, err := task.Wait(ctx)
	if err != nil {
		return nil
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c)
	go func() {
		for sig := range c {
			if sig == syscall.SIGINT {
				log.Printf("Received interrupt signal %s", sig)
				err := task.Kill(ctx, syscall.SIGINT, containerd.WithKillAll)
				if err != nil {
					panic(err)
				}
				return
			}
		}
	}()

	select {
	case code := <-status:
		log.Printf("Exited with code '%d'", code.ExitCode())
		return nil
	}
	
	return nil
}

// WithHtop configures a container to monitor the host system via `htop`
func WithHtop(ctx context.Context, client oci.Client, container *containers.Container, s *specs.Spec) error {
	// make sure we are in the host pid namespace
	if err := oci.WithHostNamespace(specs.PIDNamespace)(ctx, client, container, s); err != nil {
		return err
	}
	// make sure we set htop as our arg
	s.Process.Args = []string{"htop"}
	// make sure we have a tty set for htop
	if err := oci.WithTTY(ctx, client, container, s); err != nil {
		return err
	}
	return nil
}