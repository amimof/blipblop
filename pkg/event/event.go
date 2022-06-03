package event

import (
	"fmt"
	"sync"
)

var eventSystem *EventSystem

type ListenerFunc func(e *Event) error

func (fn ListenerFunc) Handle(e *Event) error {
	return fn(e)
}

type Listener interface {
	Handle(e *Event) error
}

type ListenerItem struct {
	Name     string
	Listener Listener
}

type EventSystem struct {
	sync.Mutex
	listeners map[string]*ListenerItem
	handlers  []ListenerFunc
}

type Event struct {
	name string
	Data map[string]interface{}
}

func (e Event) Name() string {
	return e.name
}

func (es *EventSystem) On(name string, l Listener) {
	if li, ok := es.listeners[name]; ok {
		li.Listener = l
		return
	}
	fmt.Printf("Added listener %s to eventsystem\n", name)
	es.listeners[name] = &ListenerItem{Name: name, Listener: l}
}

func (es *EventSystem) MustFire(name string) (*Event, error) {
	if !es.HasListener(name) {
		return nil, nil
	}

	e := &Event{name: name, Data: make(map[string]interface{})}
	err := es.FireEvent(e)
	if err != nil {
		return nil, err
	}

	return e, nil
}

func (es *EventSystem) FireEvent(e *Event) error {
	es.Lock()
	defer es.Unlock()
	if li, ok := es.listeners[e.Name()]; ok {
		err := li.Listener.Handle(e)
		if err != nil {
			return err
		}
	}
	return nil
}

func (es *EventSystem) HasListener(name string) bool {
	if _, ok := es.listeners[name]; ok {
		return true
	}
	return false
}

func On(name string, l Listener) {
	NewEventSystem().On(name, l)
}

func MustFire(name string) (*Event, error) {
	return NewEventSystem().MustFire(name)
}

func NewEventSystem() *EventSystem {
	if eventSystem == nil {
		eventSystem = &EventSystem{
			listeners: make(map[string]*ListenerItem),
		}
	}
	return eventSystem
}

func NewEvent(name string, data map[string]interface{}) *Event {
	return &Event{
		name: name,
		Data: data,
	}
}
