package events

import (
	"sync"

	logsv1 "github.com/amimof/voiyd/api/services/logs/v1"
)

type LogKey struct {
	NodeID    string
	TaskID    string
	SessionID string
}

type LogExchange struct {
	mu     sync.Mutex
	topics map[LogKey][]chan *logsv1.LogEntry
}

// Unsubscribe removes subscription from the exchange identified by key
func (lx *LogExchange) Unsubscribe(key LogKey, ch <-chan *logsv1.LogEntry) {
	lx.mu.Lock()
	defer lx.mu.Unlock()

	subs := lx.topics[key]
	for i, sub := range subs {
		if sub == ch {
			lx.topics[key] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	if len(lx.topics[key]) == 0 {
		delete(lx.topics, key)
	}
}

// CloseKey closes all subscriber channels for the given key and removes the topic.
func (lx *LogExchange) CloseKey(key LogKey) {
	lx.mu.Lock()
	subs := lx.topics[key]
	if len(subs) > 0 {
		delete(lx.topics, key)
	}
	lx.mu.Unlock()

	if len(subs) == 0 {
		return
	}

	for _, ch := range subs {
		close(ch)
	}
}

// Subscribe adds a subscription on the exchange using key as the ID
func (lx *LogExchange) Subscribe(key LogKey) <-chan *logsv1.LogEntry {
	if lx.topics == nil {
		lx.topics = make(map[LogKey][]chan *logsv1.LogEntry)
	}
	ch := make(chan *logsv1.LogEntry, 100)
	lx.mu.Lock()
	lx.topics[key] = append(lx.topics[key], ch)
	lx.mu.Unlock()
	return ch
}

// Publish sends the entry to all subscribers on the exchange
func (lx *LogExchange) Publish(entry *logsv1.LogEntry) {
	key := LogKey{NodeID: entry.NodeId, TaskID: entry.TaskId, SessionID: entry.SessionId}
	lx.mu.Lock()
	subs := lx.topics[key]
	lx.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- entry:
		default:
		}
	}
}
