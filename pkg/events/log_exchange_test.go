package events

import (
	"testing"
	"time"

	logsv1 "github.com/amimof/voiyd/api/services/logs/v1"
)

// helper to create a basic LogEntry
func newLogEntry(node, container, session, line string) *logsv1.LogEntry {
	return &logsv1.LogEntry{
		NodeId:      node,
		ContainerId: container,
		SessionId:   session,
		Line:        line,
	}
}

func TestLogExchange_SubscribeAndPublishSingleSubscriber(t *testing.T) {
	lx := &LogExchange{
		topics: make(map[LogKey][]chan *logsv1.LogEntry),
	}
	key := LogKey{
		NodeID:      "node-1",
		ContainerID: "ctr-1",
		SessionID:   "sess-1",
	}

	ch := lx.Subscribe(key)
	if ch == nil {
		t.Fatal("Subscribe returned a nil channel")
	}

	// Ensure topic is registered
	lx.mu.Lock()
	if subs, ok := lx.topics[key]; !ok {
		lx.mu.Unlock()
		t.Fatal("expected topic to be added to LogExchange")
	} else if len(subs) != 1 {
		lx.mu.Unlock()
		t.Fatalf("expected 1 subscriber, got %d", len(subs))
	}
	lx.mu.Unlock()

	entry := newLogEntry("node-1", "ctr-1", "sess-1", "hello")
	lx.Publish(entry)

	select {
	case got := <-ch:
		if got.Line != entry.Line {
			t.Fatalf("expected line %q, got %q", entry.Line, got.Line)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for log entry")
	}
}

func TestLogExchange_PublishToMultipleSubscribersSameKey(t *testing.T) {
	lx := &LogExchange{
		topics: make(map[LogKey][]chan *logsv1.LogEntry),
	}
	key := LogKey{
		NodeID:      "node-1",
		ContainerID: "ctr-1",
		SessionID:   "sess-1",
	}

	ch1 := lx.Subscribe(key)
	ch2 := lx.Subscribe(key)

	entry := newLogEntry("node-1", "ctr-1", "sess-1", "fanout")

	lx.Publish(entry)

	timeout := time.After(1 * time.Second)

	for i, ch := range []<-chan *logsv1.LogEntry{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Line != entry.Line {
				t.Fatalf("subscriber %d: expected line %q, got %q", i+1, entry.Line, got.Line)
			}
		case <-timeout:
			t.Fatalf("subscriber %d: timed out waiting for log entry", i+1)
		}
	}
}

func TestLogExchange_SessionIsolation(t *testing.T) {
	lx := &LogExchange{
		topics: make(map[LogKey][]chan *logsv1.LogEntry),
	}
	baseKey := LogKey{
		NodeID:      "node-1",
		ContainerID: "ctr-1",
	}

	keySess1 := LogKey{NodeID: baseKey.NodeID, ContainerID: baseKey.ContainerID, SessionID: "sess-1"}
	keySess2 := LogKey{NodeID: baseKey.NodeID, ContainerID: baseKey.ContainerID, SessionID: "sess-2"}

	ch1 := lx.Subscribe(keySess1)
	ch2 := lx.Subscribe(keySess2)

	entry1 := newLogEntry(baseKey.NodeID, baseKey.ContainerID, "sess-1", "from sess-1")
	entry2 := newLogEntry(baseKey.NodeID, baseKey.ContainerID, "sess-2", "from sess-2")

	// Publish entry for session 1; only ch1 should receive it.
	lx.Publish(entry1)

	select {
	case got := <-ch1:
		if got.Line != entry1.Line {
			t.Fatalf("expected ch1 line %q, got %q", entry1.Line, got.Line)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for entry on ch1")
	}

	select {
	case got := <-ch2:
		t.Fatalf("expected no line on ch2 for session 1, got %q", got.Line)
	case <-time.After(200 * time.Millisecond):
		// ok, nothing received
	}

	// Publish entry for session 2; only ch2 should receive it.
	lx.Publish(entry2)

	select {
	case got := <-ch2:
		if got.Line != entry2.Line {
			t.Fatalf("expected ch2 line %q, got %q", entry2.Line, got.Line)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for entry on ch2")
	}

	select {
	case got := <-ch1:
		t.Fatalf("expected no additional line on ch1 for session 2, got %q", got.Line)
	case <-time.After(200 * time.Millisecond):
		// ok, nothing received
	}
}

func TestLogExchange_UnsubscribeRemovesSubscriberAndCleansUpTopic(t *testing.T) {
	lx := &LogExchange{
		topics: make(map[LogKey][]chan *logsv1.LogEntry),
	}
	key := LogKey{
		NodeID:      "node-1",
		ContainerID: "ctr-1",
		SessionID:   "sess-1",
	}

	ch1 := lx.Subscribe(key)
	ch2 := lx.Subscribe(key)

	// Unsubscribe one subscriber; topic should still exist with one subscriber.
	lx.Unsubscribe(key, ch1)

	lx.mu.Lock()
	subs, ok := lx.topics[key]
	if !ok {
		lx.mu.Unlock()
		t.Fatal("expected topic to still exist after unsubscribing one subscriber")
	}
	if len(subs) != 1 {
		lx.mu.Unlock()
		t.Fatalf("expected 1 remaining subscriber, got %d", len(subs))
	}
	if subs[0] != ch2 {
		lx.mu.Unlock()
		t.Fatal("remaining subscriber is not the expected channel")
	}
	lx.mu.Unlock()

	// Unsubscribe the last subscriber; topic should be removed.
	lx.Unsubscribe(key, ch2)

	lx.mu.Lock()
	if _, ok := lx.topics[key]; ok {
		lx.mu.Unlock()
		t.Fatal("expected topic to be removed after unsubscribing last subscriber")
	}
	lx.mu.Unlock()
}

func TestLogExchange_CloseKeyClosesAllSubscribersAndRemovesTopic(t *testing.T) {
	lx := &LogExchange{
		topics: make(map[LogKey][]chan *logsv1.LogEntry),
	}
	key := LogKey{
		NodeID:      "node-1",
		ContainerID: "ctr-1",
		SessionID:   "sess-1",
	}

	ch1 := lx.Subscribe(key)
	ch2 := lx.Subscribe(key)

	lx.CloseKey(key)

	// After CloseKey, topic should be removed.
	lx.mu.Lock()
	if _, ok := lx.topics[key]; ok {
		lx.mu.Unlock()
		t.Fatal("expected topic to be removed after CloseKey")
	}
	lx.mu.Unlock()

	// Channels should be closed; reads should return immediately with ok == false.
	for i, ch := range []<-chan *logsv1.LogEntry{ch1, ch2} {
		select {
		case _, ok := <-ch:
			if ok {
				t.Fatalf("subscriber %d: expected channel to be closed", i+1)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("subscriber %d: timed out waiting for channel close", i+1)
		}
	}

	// Publishing after CloseKey should be a no-op (no panic, no send).
	entry := newLogEntry("node-1", "ctr-1", "sess-1", "after-close")
	lx.Publish(entry)
}

func TestLogExchange_PublishWithNoSubscribersDoesNotPanic(t *testing.T) {
	lx := &LogExchange{
		topics: make(map[LogKey][]chan *logsv1.LogEntry),
	}
	entry := newLogEntry("node-1", "ctr-1", "sess-1", "orphan")

	// No subscribers have been added. Publish should simply be a no-op.
	lx.Publish(entry)
}
