package events

import (
	"sync"

	logsv1 "github.com/amimof/blipblop/api/services/logs/v1"
)

type LogKey struct {
	NodeID      string
	ContainerID string
}

type LogExchange struct {
	mu     sync.Mutex
	topics map[LogKey][]chan *logsv1.LogEntry
}

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

func (lx *LogExchange) Publish(entry *logsv1.LogEntry) {
	key := LogKey{NodeID: entry.NodeId, ContainerID: entry.ContainerId}
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
