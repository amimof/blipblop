// Code generated by MockGen. DO NOT EDIT.
// Source: ./api/services/nodes/v1 (interfaces: NodeServiceClient)
//
// Generated by this command:
//
//	mockgen -package v1 ./api/services/nodes/v1 NodeServiceClient
//

// Package v1 is a generated GoMock package.
package v1

import (
	context "context"
	reflect "reflect"

	nodes "github.com/amimof/blipblop/api/services/nodes/v1"
	gomock "go.uber.org/mock/gomock"
	grpc "google.golang.org/grpc"
)

// MockNodeServiceClient is a mock of NodeServiceClient interface.
type MockNodeServiceClient struct {
	ctrl     *gomock.Controller
	recorder *MockNodeServiceClientMockRecorder
	isgomock struct{}
}

// MockNodeServiceClientMockRecorder is the mock recorder for MockNodeServiceClient.
type MockNodeServiceClientMockRecorder struct {
	mock *MockNodeServiceClient
}

// NewMockNodeServiceClient creates a new mock instance.
func NewMockNodeServiceClient(ctrl *gomock.Controller) *MockNodeServiceClient {
	mock := &MockNodeServiceClient{ctrl: ctrl}
	mock.recorder = &MockNodeServiceClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockNodeServiceClient) EXPECT() *MockNodeServiceClientMockRecorder {
	return m.recorder
}

// Connect mocks base method.
func (m *MockNodeServiceClient) Connect(ctx context.Context, opts ...grpc.CallOption) (nodes.NodeService_ConnectClient, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Connect", varargs...)
	ret0, _ := ret[0].(nodes.NodeService_ConnectClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Connect indicates an expected call of Connect.
func (mr *MockNodeServiceClientMockRecorder) Connect(ctx any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Connect", reflect.TypeOf((*MockNodeServiceClient)(nil).Connect), varargs...)
}

// Create mocks base method.
func (m *MockNodeServiceClient) Create(ctx context.Context, in *nodes.CreateNodeRequest, opts ...grpc.CallOption) (*nodes.CreateNodeResponse, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Create", varargs...)
	ret0, _ := ret[0].(*nodes.CreateNodeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockNodeServiceClientMockRecorder) Create(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockNodeServiceClient)(nil).Create), varargs...)
}

// Delete mocks base method.
func (m *MockNodeServiceClient) Delete(ctx context.Context, in *nodes.DeleteNodeRequest, opts ...grpc.CallOption) (*nodes.DeleteNodeResponse, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Delete", varargs...)
	ret0, _ := ret[0].(*nodes.DeleteNodeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Delete indicates an expected call of Delete.
func (mr *MockNodeServiceClientMockRecorder) Delete(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockNodeServiceClient)(nil).Delete), varargs...)
}

// Forget mocks base method.
func (m *MockNodeServiceClient) Forget(ctx context.Context, in *nodes.ForgetRequest, opts ...grpc.CallOption) (*nodes.ForgetResponse, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Forget", varargs...)
	ret0, _ := ret[0].(*nodes.ForgetResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Forget indicates an expected call of Forget.
func (mr *MockNodeServiceClientMockRecorder) Forget(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Forget", reflect.TypeOf((*MockNodeServiceClient)(nil).Forget), varargs...)
}

// Get mocks base method.
func (m *MockNodeServiceClient) Get(ctx context.Context, in *nodes.GetNodeRequest, opts ...grpc.CallOption) (*nodes.GetNodeResponse, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Get", varargs...)
	ret0, _ := ret[0].(*nodes.GetNodeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockNodeServiceClientMockRecorder) Get(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockNodeServiceClient)(nil).Get), varargs...)
}

// Join mocks base method.
func (m *MockNodeServiceClient) Join(ctx context.Context, in *nodes.JoinRequest, opts ...grpc.CallOption) (*nodes.JoinResponse, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Join", varargs...)
	ret0, _ := ret[0].(*nodes.JoinResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Join indicates an expected call of Join.
func (mr *MockNodeServiceClientMockRecorder) Join(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Join", reflect.TypeOf((*MockNodeServiceClient)(nil).Join), varargs...)
}

// List mocks base method.
func (m *MockNodeServiceClient) List(ctx context.Context, in *nodes.ListNodeRequest, opts ...grpc.CallOption) (*nodes.ListNodeResponse, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "List", varargs...)
	ret0, _ := ret[0].(*nodes.ListNodeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockNodeServiceClientMockRecorder) List(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockNodeServiceClient)(nil).List), varargs...)
}

// Update mocks base method.
func (m *MockNodeServiceClient) Update(ctx context.Context, in *nodes.UpdateNodeRequest, opts ...grpc.CallOption) (*nodes.UpdateNodeResponse, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, in}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Update", varargs...)
	ret0, _ := ret[0].(*nodes.UpdateNodeResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Update indicates an expected call of Update.
func (mr *MockNodeServiceClientMockRecorder) Update(ctx, in any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, in}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockNodeServiceClient)(nil).Update), varargs...)
}
