package middleware

import (
	"context"
)

type Middleware interface {
	Run(context.Context, <-chan struct{})
}

type MiddlewareManager struct {
	middlewares []Middleware
	stop        chan struct{}
}

func (c *MiddlewareManager) Register(m Middleware) {
	if m != nil {
		c.middlewares = append(c.middlewares, m)
	}
}

func (c *MiddlewareManager) Run(ctx context.Context) {
	c.stop = make(chan struct{})
	for _, ctrl := range c.middlewares {
		go ctrl.Run(ctx, c.stop)
	}
}

func (c *MiddlewareManager) Stop() {
	c.stop <- struct{}{}
}

func NewManager(m ...Middleware) *MiddlewareManager {
	cm := &MiddlewareManager{}
	for _, middleware := range m {
		cm.Register(middleware)
	}
	return cm
}
