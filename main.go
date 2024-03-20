package main

import (
	"os"

	cmd "github.com/amimof/blipblop/cmd/bbctl"
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
	if err := cmd.NewDefaultCommand().Execute(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}
	os.Exit(0)
}
