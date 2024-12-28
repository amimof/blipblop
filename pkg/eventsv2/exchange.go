package eventsv2

import (
	"context"
	"log"
	"sync"

	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/logger"
)

var (
	_ Publisher  = &Exchange{}
	_ Forwarder  = &Exchange{}
	_ Subscriber = &Exchange{}
)

type HandlerFunc func(context.Context, *eventsv1.Event) error

type Exchange struct {
	topics             map[eventsv1.EventType][]chan *eventsv1.Event
	persistentHandlers map[eventsv1.EventType][]HandlerFunc
	fireOnceHandlers   map[eventsv1.EventType][]HandlerFunc
	errChan            chan error
	mu                 sync.Mutex
	logger             logger.Logger
	publishers         []Publisher
}

// AddPublisher adds a forwarder to this Exchange
func (e *Exchange) AddPublisher(forwarder Publisher) {
	e.publishers = append(e.publishers, forwarder)
}

// On registers a handler func for a certain event type
func (e *Exchange) On(ev eventsv1.EventType, f HandlerFunc) {
	log.Println("On", ev.String())
	e.mu.Lock()
	defer e.mu.Unlock()
	e.persistentHandlers[ev] = append(e.persistentHandlers[ev], f)
}

// Once attaches a handler to the specified event type. The handler func is only executed once
func (e *Exchange) Once(ev eventsv1.EventType, f HandlerFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.fireOnceHandlers[ev] = append(e.fireOnceHandlers[ev], f)
}

// Unsubscribe implements Subscriber.
func (e *Exchange) Unsubscribe(context.Context, eventsv1.EventType) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return nil
}

// Forward publishes the event using the publishers added to this exchange. Implements Forwarder.
func (e *Exchange) Forward(ctx context.Context, t eventsv1.EventType, ev *eventsv1.Event) error {
	for _, forwarder := range e.publishers {
		forwarder.Publish(ctx, t, ev)
	}
	return nil
}

// Subscribe subscribes to events of a certain event type
func (e *Exchange) Subscribe(ctx context.Context, t ...eventsv1.EventType) chan *eventsv1.Event {
	ch := make(chan *eventsv1.Event, 10)
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, evType := range t {
		e.topics[evType] = append(e.topics[evType], ch)
	}
	return ch
}

// Publish publishes an event of a certain type
func (e *Exchange) Publish(ctx context.Context, t eventsv1.EventType, ev *eventsv1.Event) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Publish to subscribers on the topic
	if evChans, ok := e.topics[t]; ok {
		for _, evChan := range evChans {
			go func(ch chan *eventsv1.Event) {
				ch <- ev
			}(evChan)
		}
	}

	// Run persistent handler funcs
	if handlers, ok := e.persistentHandlers[t]; ok {
		for _, handler := range handlers {
			log.Println("Running handler", ev.String())
			if err := handler(ctx, ev); err != nil {
				log.Println("error running handler", ev.String(), err)
				e.errChan <- err
			}
		}
	}
	// Run oneoff handler funcs
	if handlers, ok := e.fireOnceHandlers[t]; ok {
		for i, handler := range handlers {
			if err := handler(ctx, ev); err != nil {
				e.errChan <- err
			}
			handlers = remove(handlers, i)
		}
	}
	return nil
}

func remove(s []HandlerFunc, i int) []HandlerFunc {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func NewExchange() *Exchange {
	return &Exchange{
		topics:             make(map[eventsv1.EventType][]chan *eventsv1.Event),
		persistentHandlers: make(map[eventsv1.EventType][]HandlerFunc),
		errChan:            make(chan error),
	}
}
