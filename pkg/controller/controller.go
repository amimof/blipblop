package controller

import (
	"context"
)

type Controller interface {
	Run(context.Context, <-chan struct{})
	Reconcile(context.Context) error
}
