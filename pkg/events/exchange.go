package events

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/amimof/voiyd/pkg/logger"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	leasesv1 "github.com/amimof/voiyd/api/services/leases/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
	volumesv1 "github.com/amimof/voiyd/api/services/volumes/v1"
)

var (
	_      Publisher  = &Exchange{}
	_      Forwarder  = &Exchange{}
	_      Subscriber = &Exchange{}
	tracer            = otel.Tracer("exchange")
)

type (
	HandlerFunc       func(context.Context, *eventsv1.Event) error
	TaskHandlerFunc   func(context.Context, *tasksv1.Task) error
	VolumeHandlerFunc func(context.Context, *volumesv1.Volume) error
	NodeHandlerFunc   func(context.Context, *nodesv1.Node) error
	LeaseHandlerFunc  func(context.Context, *leasesv1.Lease) error

	NewExchangeOption func(*Exchange)
)

type Exchange struct {
	topics             map[eventsv1.EventType][]chan *eventsv1.Event
	persistentHandlers map[eventsv1.EventType][]HandlerFunc
	fireOnceHandlers   map[eventsv1.EventType][]HandlerFunc
	errChan            chan error
	mu                 sync.Mutex
	logger             logger.Logger
	forwarders         []Forwarder
}

func WithExchangeLogger(l logger.Logger) NewExchangeOption {
	return func(e *Exchange) {
		e.logger = l
	}
}

// AddForwarder adds a forwarder to this Exchange, forwarding any message on Publish()
func (e *Exchange) AddForwarder(forwarder Forwarder) {
	e.forwarders = append(e.forwarders, forwarder)
}

// On registers a handler func for a certain event type
func (e *Exchange) On(ev eventsv1.EventType, f HandlerFunc) {
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

// Forward publishes the event using the publishers added to this exchange. Implements Forwarder.
func (e *Exchange) Forward(ctx context.Context, ev *eventsv1.Event) error {
	return e.publish(ctx, ev, true)
}

// Publish publishes an event of a certain type
func (e *Exchange) Publish(ctx context.Context, ev *eventsv1.Event) error {
	return e.publish(ctx, ev, false)
}

// Publish publishes an event of a certain type
func (e *Exchange) publish(ctx context.Context, ev *eventsv1.Event, persist bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	t := ev.GetType()

	ctx, span := tracer.Start(ctx, "exchange.Publish")
	span.SetAttributes(
		attribute.String("event.client.id", ev.GetClientId()),
		attribute.String("event.object.id", ev.GetObjectId()),
		attribute.String("event.type", t.String()),
	)
	defer span.End()

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
			if err := handler(ctx, ev); err != nil {
				go func(err error) {
					e.errChan <- err
				}(err)
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

	// Call forwarders if persistent
	if persist {
		for _, forwarder := range e.forwarders {
			if err := forwarder.Forward(ctx, ev); err != nil {
				return err
			}
		}
	}

	return nil
}

func remove(s []HandlerFunc, i int) []HandlerFunc {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func NewExchange(opts ...NewExchangeOption) *Exchange {
	e := &Exchange{
		topics:             make(map[eventsv1.EventType][]chan *eventsv1.Event),
		persistentHandlers: make(map[eventsv1.EventType][]HandlerFunc),
		errChan:            make(chan error),
		logger:             logger.ConsoleLogger{},
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}
