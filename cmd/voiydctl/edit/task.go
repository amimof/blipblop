package edit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"google.golang.org/protobuf/proto"

	"github.com/amimof/voiyd/pkg/client"
	"github.com/amimof/voiyd/pkg/cmdutil"

	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

func NewCmdEditContainer(cfg *client.Config) *cobra.Command {
	runCmd := &cobra.Command{
		Use:     "task NAME",
		Short:   "Edit a task",
		Long:    "Edit a task",
		Example: `voiydctl edit task NAME`,
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := viper.BindPFlags(cmd.Flags()); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			baseCtx := cmd.Context()

			tracer := otel.Tracer("voiydctl")
			baseCtx, span := tracer.Start(baseCtx, "voiydctl.edit.task")
			defer span.End()

			// Setup client
			currentSrv, err := cfg.CurrentServer()
			if err != nil {
				logrus.Fatal(err)
			}
			c, err := client.New(currentSrv.Address, client.WithTLSConfigFromCfg(cfg))
			if err != nil {
				logrus.Fatalf("error setting up client: %v", err)
				return err
			}
			defer func() {
				if err := c.Close(); err != nil {
					logrus.Fatalf("error closing client connection: %v", err)
				}
			}()

			tname := args[0]

			getCtx, cancel := context.WithTimeout(baseCtx, time.Second*30)
			defer cancel()
			task, err := c.TaskV1().Get(getCtx, tname)
			if err != nil {
				return err
			}

			codec, err := cmdutil.CodecFor(output)
			if err != nil {
				logrus.Fatalf("error creating serializer: %v", err)
			}

			b, err := codec.Serialize(task)
			if err != nil {
				logrus.Fatalf("error serializing: %v", err)
			}

			// Create temporary file to hold the JSON
			tmpFile, err := os.CreateTemp("", fmt.Sprintf("*.%s", output))
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

			var updatedTask tasksv1.Task
			err = codec.Deserialize(ub, &updatedTask)
			if err != nil {
				return err
			}

			// Exit early if no changes where made
			if proto.Equal(task, &updatedTask) {
				logrus.Info("no changes detected")
				os.Exit(0)
			}

			// Send update to server
			updateCtx, cancel := context.WithTimeout(baseCtx, time.Second*30)
			defer cancel()
			err = c.TaskV1().Update(updateCtx, tname, &updatedTask)
			if err != nil {
				return err
			}

			logrus.Infof("task %s was updated", tname)

			return nil
		},
	}

	return runCmd
}
