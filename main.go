package main

import (
	"context"
	"os"

	cmd "github.com/amimof/blipblop/cmd/bbctl"
	"github.com/amimof/blipblop/pkg/trace"
	"github.com/sirupsen/logrus"
)

var (
	// VERSION of the app. Is set when project is built and should never be set manually
	VERSION string
	// COMMIT is the Git commit currently used when compiling. Is set when project is built and should never be set manually
	COMMIT string
	// BRANCH is the Git branch currently used when compiling. Is set when project is built and should never be set manually
	BRANCH string
	// GOVERSION used to compile. Is set when project is built and should never be set manually
	GOVERSION string
	// DATE used to compile. Is set when project is built and should never be set manually
	DATE string
)

func main() {
	cmd.SetVersionInfo(VERSION, COMMIT, DATE, BRANCH, GOVERSION)

	ctx := context.Background()
	shutdownTraceProvider, err := trace.InitTracing(ctx, "bbctl", VERSION, "192.168.5.15:4317")
	if err != nil {
		logrus.Fatalf("error setting up tracing: %v", err)
	}
	defer shutdownTraceProvider(ctx)

	if err := cmd.NewDefaultCommand().Execute(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	os.Exit(0)
}
