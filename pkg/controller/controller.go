// Package controller provides an interface and various types implementing system specific and agnostic controllers
package controller

import (
	"context"
)

type Controller interface {
	Run(context.Context) error
	Reconcile(context.Context) error
}
