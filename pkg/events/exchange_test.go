package events

import (
	"context"
	"sync"
	"testing"
	"time"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	"github.com/amimof/voiyd/pkg/logger"
)

func TestExchange_Subscribe(t *testing.T) {
	e := &Exchange{
		topics: make(map[eventsv1.EventType][]chan *eventsv1.Event),
		mu:     sync.Mutex{},
		logger: logger.ConsoleLogger{},
	}

	ctx := context.Background()
	topic := eventsv1.EventType_ContainerCreate

	ch := e.Subscribe(ctx, topic)
	if ch == nil {
		t.Fatal("Subscribe returned a nil channel")
	}

	// Ensure the topic is added to the map
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.topics[topic]; !ok {
		t.Fatal("Topic was not added to the Exchange")
	}
	if len(e.topics[topic]) != 1 {
		t.Fatalf("Expected 1 subscriber, got %d", len(e.topics[topic]))
	}
}

func TestExchange_Publish(t *testing.T) {
	e := &Exchange{
		topics: make(map[eventsv1.EventType][]chan *eventsv1.Event),
		mu:     sync.Mutex{},
		logger: logger.ConsoleLogger{},
	}

	ctx := context.Background()
	topic := eventsv1.EventType_ContainerCreate

	// Subscribe to the topic
	ch := e.Subscribe(ctx, topic)

	// Publish an event
	event := &eventsv1.Event{Type: eventsv1.EventType_ContainerCreate}
	err := e.Publish(ctx, topic, event)
	if err != nil {
		t.Fatalf("Publish returned an error: %v", err)
	}

	// Verify the event is received
	select {
	case ev := <-ch:
		if ev.GetType() != event.GetType() {
			t.Fatalf("Expected event ID %s, got %s", event.GetType(), ev.GetType())
		}
	case <-time.After(1 * time.Second):

		t.Fatal("Timed out waiting for event")
	}
}

func TestExchange_Handler(t *testing.T) {
	ctx := context.Background()
	topic := eventsv1.EventType_ContainerCreate
	event := &eventsv1.Event{Type: eventsv1.EventType_ContainerCreate}

	i := 0
	e := NewExchange()

	// Attach multiple handlers, each modifying i
	e.On(topic, func(ctx context.Context, e *eventsv1.Event) error {
		i = i + 1
		return nil
	})
	e.On(topic, func(ctx context.Context, e *eventsv1.Event) error {
		i = i + 2
		return nil
	})

	// Publish two events that should call the 2 above handlers
	_ = e.Publish(ctx, topic, event)
	_ = e.Publish(ctx, topic, event)

	if i != 6 {
		t.Fatalf("Expected i to be 6, got %d", i)
	}
}

func TextExchange_FireOnceHandler(t *testing.T) {
	ctx := context.Background()
	topic := eventsv1.EventType_ContainerCreate
	event := &eventsv1.Event{Type: eventsv1.EventType_ContainerCreate}

	i := 0
	e := NewExchange()

	// Handler should be executed only once
	e.Once(topic, func(ctx context.Context, e *eventsv1.Event) error {
		i = i + 1
		return nil
	})

	// Publish three events, only the first event fires the handler
	_ = e.Publish(ctx, topic, event)
	_ = e.Publish(ctx, topic, event)
	_ = e.Publish(ctx, topic, event)

	if i != 1 {
		t.Fatalf("Expected i to be 1, got %d", i)
	}
}

func TestExchange_ThreadSafety(t *testing.T) {
	e := &Exchange{
		topics: make(map[eventsv1.EventType][]chan *eventsv1.Event),
		mu:     sync.Mutex{},
		logger: logger.ConsoleLogger{},
	}

	ctx := context.Background()
	topic := eventsv1.EventType_ContainerCreate

	// Start multiple goroutines to subscribe and publish
	const goroutines = 100
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// Subscribe to the topic
			ch := e.Subscribe(ctx, topic)

			// Publish an event
			event := &eventsv1.Event{Type: eventsv1.EventType_ContainerCreate}
			err := e.Publish(ctx, topic, event)
			if err != nil {
				t.Errorf("Publish returned an error in goroutine %d: %v", i, err)
			}

			// Verify the event is received
			select {
			case <-ch:
				// Success
			case <-time.After(5 * time.Second):
				t.Errorf("Timed out waiting for event in goroutine %d", i)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}
