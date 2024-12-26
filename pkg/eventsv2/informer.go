package eventsv2

import (
	"sync"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
)

type Informer struct {
	errCh    <-chan error
	ch       <-chan *eventsv1.Event
	handlers map[*eventsv1.EventType][]HandlerFunc
	mu       sync.Mutex
}

// On attaches a handler to the specified event type
func (i *Informer) On(ev *eventsv1.EventType, f HandlerFunc) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.handlers[ev] = append(i.handlers[ev], f)
}

// Run starts receiving events and executes the handlers in sequence
func (i *Informer) Run(eventChan <-chan *eventsv1.Event, errChan chan error) {
	for ev := range eventChan {
		for _, ty := range i.handlers {
			for _, h := range ty {
				if err := h(ev); err != nil {
					errChan <- err
				}
			}
		}
	}
}

func New(ch <-chan *eventsv1.Event) *Informer {
	return &Informer{ch: ch}
}
