package middleware

type Middleware interface {
	Run(<-chan struct{})
}

type MiddlewareManager struct {
	middlewares []Middleware
}

func (c *MiddlewareManager) Register(m Middleware) {
	if m != nil {
		c.middlewares = append(c.middlewares, m)
	}
}

func (c *MiddlewareManager) SpawnAll() {
	stop := make(chan struct{})
	for _, ctrl := range c.middlewares {
		go ctrl.Run(stop)
	}
}

func NewManager(m ...Middleware) *MiddlewareManager {
	cm := &MiddlewareManager{}
	for _, middleware := range m {
		cm.Register(middleware)
	}
	return cm
}
