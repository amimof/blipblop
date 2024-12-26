package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/amimof/blipblop/api/services/containers/v1"
	eventsv1 "github.com/amimof/blipblop/api/services/events/v1"
	"github.com/amimof/blipblop/pkg/events"
	"github.com/amimof/blipblop/pkg/eventsv2"
)

func main() {
	exchange := eventsv2.NewExchange()

	evType := eventsv1.EventType_ContainerCreate
	ev := events.NewEvent(evType, &containers.Container{})

	// Subscribe
	ch := exchange.Subscribe(context.Background(), evType)

	// Add Handler
	exchange.On(evType, func(e *eventsv1.Event) error {
		log.Println("Processing Data for event")
		time.Sleep(time.Second + 1)
		log.Println("Processing finished")
		return nil
	})

	exit := make(chan os.Signal, 1)

	signal.Notify(exit, os.Interrupt)

	go func() {
		for e := range ch {
			log.Println("Got event", e)
		}
	}()

	exchange.Publish(context.Background(), evType, ev)

	<-exit

	log.Println("Program finished")
}
