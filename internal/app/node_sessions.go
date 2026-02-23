package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/amimof/voiyd/pkg/events"
	"github.com/amimof/voiyd/pkg/keys"
	"github.com/amimof/voiyd/pkg/logger"

	eventsv1 "github.com/amimof/voiyd/api/services/events/v1"
	nodesv1 "github.com/amimof/voiyd/api/services/nodes/v1"
	tasksv1 "github.com/amimof/voiyd/api/services/tasks/v1"
)

var (
	ErrNodeNotConnected = errors.New("node not connected")
	ErrNodeQueueFull    = errors.New("node outbound queue full")
)

type TaskGetter interface {
	Get(ctx context.Context, id keys.ID) (*tasksv1.Task, error)
	List(ctx context.Context, limit int32) ([]*tasksv1.Task, error)
}

type NodeSender interface {
	SendToNode(ctx context.Context, nodeUID string, ev *eventsv1.Event) error
	IsNodeConnected(nodeUID string) bool
}

type SessionManager interface {
	Connect(ctx context.Context, node *nodesv1.Node, in NodeConnectInput) (Session, error)
	Set(ctx context.Context, nodeUID string, node *nodesv1.Node) error
}

type Session interface {
	Handle(ctx context.Context, ev *eventsv1.Event) error
	Next(ctx context.Context) (*eventsv1.Event, error)
	Close() error
}

type NodeSessionManagerOption func(*NodeSessionManager)

// NodeConnectInput represents the identity established for a node stream.
// NodeUID and NodeName are expected to come from transport metadata.
type NodeConnectInput struct {
	NodeUID  string
	NodeName string
}

func WithOutboundBuffer(size int) NodeSessionManagerOption {
	return func(m *NodeSessionManager) {
		if size > 0 {
			m.outBuf = size
		}
	}
}

// NodeSessionManager owns node stream sessions and enables targeted delivery.
//
// Intended usage:
// - transport calls Connect() to open a session for a stream
// - transport forwards inbound stream messages to Session.Handle()
// - transport writes outbound messages from Session.Next() to the stream
// - business logic sends targeted events using SendToNode()
type NodeSessionManager struct {
	exchange *events.Exchange
	logger   logger.Logger

	mu       sync.Mutex
	sessions map[string]*nodeSession
	outBuf   int
}

type nodeSession struct {
	manager *NodeSessionManager

	nodeUID  string
	nodeName string
	node     *nodesv1.Node

	out chan *eventsv1.Event

	closeOnce sync.Once
	closed    chan struct{}
}

// func (m *NodeSessionManager) Subscribe(ctx context.Context, in NodeConnectInput, evType ...eventsv1.EventType) (Session, error) {
// 	eventChan := m.exchange.Subscribe(ctx, evType...)
// 	sess := &nodeSession{
// 		manager:  m,
// 		nodeUID:  in.NodeUID,
// 		nodeName: in.NodeName,
// 		out:      make(chan *eventsv1.Event, m.outBuf),
// 		closed:   make(chan struct{}),
// 	}
// }

func NewNodeSessionManager(exchange *events.Exchange, l logger.Logger, opts ...NodeSessionManagerOption) *NodeSessionManager {
	m := &NodeSessionManager{
		exchange: exchange,
		logger:   l,
		sessions: make(map[string]*nodeSession),
		outBuf:   64,
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.logger == nil {
		m.logger = logger.ConsoleLogger{}
	}
	return m
}

func (m *NodeSessionManager) List(ctx context.Context, limit int32) ([]*nodesv1.Node, error) {
	var nodes []*nodesv1.Node
	for _, sess := range m.sessions {
		nodes = append(nodes, sess.node)
	}
	return nodes, nil
}

func (m *NodeSessionManager) Set(ctx context.Context, nodeUID string, node *nodesv1.Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[nodeUID]; ok {
		m.sessions[nodeUID].node = node
		return nil
	}
	return errors.New("node session not found")
}

func (m *NodeSessionManager) Get(ctx context.Context, nodeUID string) (*nodesv1.Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[nodeUID]; ok {
		return m.sessions[nodeUID].node, nil
	}
	return nil, errors.New("node session not found")
}

func (m *NodeSessionManager) Connect(ctx context.Context, node *nodesv1.Node, in NodeConnectInput) (Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if in.NodeUID == "" {
		return nil, status.Error(codes.FailedPrecondition, "missing node uid")
	}
	if in.NodeName == "" {
		return nil, status.Error(codes.FailedPrecondition, "missing node name")
	}

	sess := &nodeSession{
		manager:  m,
		nodeUID:  in.NodeUID,
		nodeName: in.NodeName,
		node:     node,
		out:      make(chan *eventsv1.Event, m.outBuf),
		closed:   make(chan struct{}),
	}

	var old *nodeSession
	m.mu.Lock()
	old = m.sessions[in.NodeUID]
	m.sessions[in.NodeUID] = sess
	m.mu.Unlock()

	// Close old session (reconnect) outside the lock.
	if old != nil {
		_ = old.Close()
	}

	// Publish NodeConnect.
	if m.exchange != nil {
		if err := m.exchange.Publish(ctx, events.NewEvent(events.NodeConnect, node)); err != nil {
			m.logger.Error("failed to publish node connect", "error", err, "nodeUID", in.NodeUID, "nodeName", in.NodeName)
		}
	}

	m.logger.Info("node connected", "nodeUID", in.NodeUID, "nodeName", in.NodeName)
	return sess, nil
}

func (m *NodeSessionManager) IsNodeConnected(nodeUID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.sessions[nodeUID]
	return ok
}

// SendToNode enqueues an event for delivery to a single connected node.
//
// It returns ErrNodeNotConnected if the node has no active session.
// It returns ErrNodeQueueFull if the node session is backpressured.
func (m *NodeSessionManager) SendToNode(ctx context.Context, nodeUID string, ev *eventsv1.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if nodeUID == "" {
		return status.Error(codes.InvalidArgument, "nodeUID is required")
	}
	if ev == nil {
		return status.Error(codes.InvalidArgument, "event is required")
	}

	m.mu.Lock()
	sess := m.sessions[nodeUID]
	m.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("%w: %s", ErrNodeNotConnected, nodeUID)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-sess.closed:
		return fmt.Errorf("%w: %s", ErrNodeNotConnected, nodeUID)
	case sess.out <- ev:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrNodeQueueFull, nodeUID)
	}
}

// Disconnect closes and removes the session for nodeUID, if present.
func (m *NodeSessionManager) Disconnect(nodeUID string) {
	m.mu.Lock()
	sess := m.sessions[nodeUID]
	m.mu.Unlock()
	if sess != nil {
		_ = sess.Close()
	}
}

func (s *nodeSession) Handle(ctx context.Context, ev *eventsv1.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ev == nil {
		return status.Error(codes.InvalidArgument, "event is required")
	}
	if s.manager.exchange == nil {
		return status.Error(codes.FailedPrecondition, "event exchange is not configured")
	}
	return s.manager.exchange.Publish(ctx, ev)
}

func (s *nodeSession) Next(ctx context.Context) (*eventsv1.Event, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.closed:
		return nil, io.EOF
	case ev, ok := <-s.out:
		if !ok {
			return nil, io.EOF
		}
		return ev, nil
	}
}

func (s *nodeSession) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)

		// Remove from manager map if we're still the active session.
		m := s.manager
		m.mu.Lock()
		cur := m.sessions[s.nodeUID]
		if cur == s {
			delete(m.sessions, s.nodeUID)
		}
		m.mu.Unlock()

		// Stop writers.
		close(s.out)

		// Publish NodeForget.
		if m.exchange != nil && s.node != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := m.exchange.Publish(ctx, events.NewEvent(events.NodeForget, s.node)); err != nil {
				m.logger.Error("failed to publish node forget", "error", err, "nodeUID", s.nodeUID, "nodeName", s.nodeName)
			}
		}

		m.logger.Info("node disconnected", "nodeUID", s.nodeUID, "nodeName", s.nodeName)
	})
	return nil
}
