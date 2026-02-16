package condition

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	typesv1 "github.com/amimof/voiyd/api/types/v1"
)

type Publisher interface {
	Run(context.Context)
	Report(string, bool, *typesv1.ConditionReport)
}

type ConditionClient interface {
	Condition(context.Context, string, ...*typesv1.Condition) error
}

type publisher struct {
	mu            sync.Mutex
	subPublishers map[string]*ConditionPublisher
	ch            chan *ConditionPublisher
	client        ConditionClient
	done          chan string
}

type ConditionPublisher struct {
	mu     sync.Mutex
	state  []*typesv1.Condition
	condCh chan *typesv1.Condition
	client ConditionClient
	id     string
	done   chan string
}

func (p *publisher) addSubPublisher(sub *ConditionPublisher) {
	resourceID := sub.id
	p.subPublishers[resourceID] = sub
	go func() {
		p.ch <- sub
	}()
}

// Report implements [Publisher].
func (p *publisher) Report(resourceID string, status bool, report *typesv1.ConditionReport) {
	p.mu.Lock()
	defer p.mu.Unlock()

	cond := &typesv1.Condition{
		Type:               wrapperspb.String(string(report.Type)),
		Status:             wrapperspb.Bool(status),
		Reason:             wrapperspb.String(report.Reason),
		Msg:                wrapperspb.String(report.Msg),
		LastTransitionTime: timestamppb.Now(),
	}

	if _, ok := p.subPublishers[resourceID]; !ok {
		sub := &ConditionPublisher{
			state:  make([]*typesv1.Condition, 0, 8),
			condCh: make(chan *typesv1.Condition, 256),
			client: p.client,
			id:     resourceID,
			done:   p.done,
		}
		p.addSubPublisher(sub)
		// Send condition and return - SubPublisher goroutine will be started by Run()
		sub.condCh <- cond
		return
	}

	sub := p.subPublishers[resourceID]
	sub.condCh <- cond
}

func (s *ConditionPublisher) runForTask(ctx context.Context) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	idleTimer := time.NewTimer(10 * time.Second)
	defer idleTimer.Stop()

	flush := func() error {
		s.mu.Lock()
		defer s.mu.Unlock()
		if len(s.state) == 0 {
			return nil
		}

		// Send to server
		err := s.client.Condition(ctx, s.id, s.state...)

		// Clear state after successful send
		if err == nil {
			s.state = s.state[:0]
		}

		return err
	}
	for {
		select {
		case <-ctx.Done():
			// TODO: use logging for context cancellation
			_ = flush()
			return
		case cond := <-s.condCh:
			s.mu.Lock()
			idleTimer.Reset(10 * time.Second)

			// Deduplicate: find existing condition with same Type
			found := false
			for i, st := range s.state {
				if st.Type.GetValue() == cond.Type.GetValue() {
					// Found matching type - compare timestamps
					if cond.LastTransitionTime.AsTime().After(st.LastTransitionTime.AsTime()) {
						// New condition is newer, replace old one
						s.state[i] = cond
						found = true
					} else {
						// New condition is stale, drop it
						found = true
					}
					break
				}
			}
			// If no matching type found, append new condition
			if !found {
				s.state = append(s.state, cond)
			}
			s.mu.Unlock()

		case <-ticker.C:
			if err := flush(); err != nil {
				fmt.Printf("error flushing: %v\n", err)
			}
		case <-idleTimer.C:
			if err := flush(); err != nil {
				fmt.Printf("error flushing: %v\n", err)
			}
			s.done <- s.id
			return
		}
	}
}

// Run implements [Publisher].
// TODO: Implement error handling for subpublishers
func (p *publisher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// TODO: use logging for context cancellation
			return
		case state := <-p.ch:
			go state.runForTask(ctx)
		case id := <-p.done:
			p.mu.Lock()
			delete(p.subPublishers, id)
			p.mu.Unlock()
		}
	}
}

func NewPublisher(client ConditionClient) Publisher {
	return &publisher{
		subPublishers: make(map[string]*ConditionPublisher),
		ch:            make(chan *ConditionPublisher, 256),
		client:        client,
		done:          make(chan string),
	}
}
