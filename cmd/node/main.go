package main

import (
	"log"
	"fmt"
	"os"
	"os/signal"
	"context"
	"golang.org/x/sys/unix"
	"github.com/containerd/console"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/namespaces"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func main() {
	err := runTask()
	if err != nil {
		log.Fatal(err)
	}
}

func runTask() error {
	client, err := containerd.New("/run/containerd/containerd.sock")
	if err != nil {
		return err
	}
	defer client.Close()

	ctx := namespaces.WithNamespace(context.Background(), "opts")
	image, err := client.Pull(ctx, "docker.io/crosbymichael/htop:latest", containerd.WithPullUnpack)
	if err != nil {
		return err
	}
	log.Printf("Successfully pulled image: '%s'", image.Name())

	
	container, err := client.NewContainer(
		ctx,
		"htop",
		containerd.WithNewSpec(oci.WithImageConfig(image)),
		containerd.WithImage(image),
		containerd.WithNewSnapshot("htop-snapshot", image),
	)
	if err != nil {
		return err
	}
	defer container.Delete(ctx, containerd.WithSnapshotCleanup)

	spec, err := oci.GenerateSpec(ctx, client, container, monitor.WithHtop)
	if err != nil {
		return err
	}

	con := console.Current()
	defer con.Reset()
	if err := con.SetRaw(); err != nil {
		return err
	}

	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return err
	}te
	defer task.Delete(ctx, containerd.WithProcessKill)

	exitStatusC := make(chan string, 1)
	go func() {
		status, err := task.Wait(ctx)
		if err != nil {
			fmt.Println(err)
		}
		exitStatusC <- fmt.Sprintf("%d", <-status)
	}()

	go func() {
		resize := func() error {
			size, err := con.Size()
			if err != nil {
				return err
			}
			if err := task.Resize(ctx, uint32(size.Width), uint32(size.Height)); err != nil {
				return err
			}
			return nil
		}
		resize()
		s := make(chan os.Signal, 16)
		signal.Notify(s, unix.SIGWINCH)
		for range s {
			resize()
		}	
	}()
	if err := task.Start(ctx); err != nil {
		return err
	}
	
	<-exitStatusC
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