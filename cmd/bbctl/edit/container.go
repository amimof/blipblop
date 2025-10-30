package edit

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	"github.com/amimof/blipblop/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func NewCmdEditContainer(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "container",
		Short:   "Edit a container",
		Long:    "Edit a container",
		Example: `bbctl edit container NAME`,
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()

			tracer := otel.Tracer("bbctl")
			ctx, span := tracer.Start(ctx, "bbctl.edit.container")
			defer span.End()

			// Setup client
			c, err := client.New(cfg.CurrentServer().Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
				return err
			}
			defer func() {
				if err := c.Close(); err != nil {
					logrus.Fatalf("error closing client connection: %v", err)
				}
			}()

			cname := args[0]

			ctr, err := c.ContainerV1().Get(ctx, cname)
			if err != nil {
				return err
			}

			marshaler := protojson.MarshalOptions{
				EmitUnpopulated: true,
				Indent:          "  ",
			}
			b, err := marshaler.Marshal(ctr)
			if err != nil {
				return err
			}

			// Create temporary file to hold the JSON
			tmpFile, err := os.CreateTemp("", "*.json")
			if err != nil {
				return err
			}
			defer func() {
				if err := tmpFile.Close(); err != nil {
					logrus.Fatalf("error closing tmp file: %v", err)
				}
			}()

			_, err = tmpFile.Write(b)
			if err != nil {
				return err
			}

			// Get the editor from the environment variable, default to Vim
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vim"
			}

			// Open text editor
			editorCmd := exec.Command(editor, tmpFile.Name())
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			if err := editorCmd.Run(); err != nil {
				return err
			}

			// Read modified ctr in file
			ub, err := os.ReadFile(tmpFile.Name())
			if err != nil {
				return err
			}

			var updatedCtr containers.Container
			err = protojson.Unmarshal(ub, &updatedCtr)
			if err != nil {
				return err
			}

			// Exit early if no changes where made
			if proto.Equal(ctr, &updatedCtr) {
				logrus.Info("no changes detected")
				os.Exit(0)
			}

			// Send update to server
			err = c.ContainerV1().Update(ctx, cname, &updatedCtr)
			if err != nil {
				return err
			}

			logrus.Infof("container %s was updated", cname)

			return nil
		},
	}

	return runCmd
}
