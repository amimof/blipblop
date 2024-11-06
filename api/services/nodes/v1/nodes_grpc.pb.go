// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             (unknown)
// source: api/services/nodes/v1/nodes.proto

package nodes

import (
	context "context"
	v1 "github.com/amimof/blipblop/api/services/events/v1"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// NodeServiceClient is the client API for NodeService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type NodeServiceClient interface {
	List(ctx context.Context, in *ListNodeRequest, opts ...grpc.CallOption) (*ListNodeResponse, error)
	Get(ctx context.Context, in *GetNodeRequest, opts ...grpc.CallOption) (*GetNodeResponse, error)
	Create(ctx context.Context, in *CreateNodeRequest, opts ...grpc.CallOption) (*CreateNodeResponse, error)
	Update(ctx context.Context, in *UpdateNodeRequest, opts ...grpc.CallOption) (*UpdateNodeResponse, error)
	Delete(ctx context.Context, in *DeleteNodeRequest, opts ...grpc.CallOption) (*DeleteNodeResponse, error)
	Join(ctx context.Context, in *JoinRequest, opts ...grpc.CallOption) (*JoinResponse, error)
	Forget(ctx context.Context, in *ForgetRequest, opts ...grpc.CallOption) (*ForgetResponse, error)
	Connect(ctx context.Context, opts ...grpc.CallOption) (NodeService_ConnectClient, error)
}

type nodeServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewNodeServiceClient(cc grpc.ClientConnInterface) NodeServiceClient {
	return &nodeServiceClient{cc}
}

func (c *nodeServiceClient) List(ctx context.Context, in *ListNodeRequest, opts ...grpc.CallOption) (*ListNodeResponse, error) {
	out := new(ListNodeResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.nodes.v1.NodeService/List", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeServiceClient) Get(ctx context.Context, in *GetNodeRequest, opts ...grpc.CallOption) (*GetNodeResponse, error) {
	out := new(GetNodeResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.nodes.v1.NodeService/Get", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeServiceClient) Create(ctx context.Context, in *CreateNodeRequest, opts ...grpc.CallOption) (*CreateNodeResponse, error) {
	out := new(CreateNodeResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.nodes.v1.NodeService/Create", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeServiceClient) Update(ctx context.Context, in *UpdateNodeRequest, opts ...grpc.CallOption) (*UpdateNodeResponse, error) {
	out := new(UpdateNodeResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.nodes.v1.NodeService/Update", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeServiceClient) Delete(ctx context.Context, in *DeleteNodeRequest, opts ...grpc.CallOption) (*DeleteNodeResponse, error) {
	out := new(DeleteNodeResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.nodes.v1.NodeService/Delete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeServiceClient) Join(ctx context.Context, in *JoinRequest, opts ...grpc.CallOption) (*JoinResponse, error) {
	out := new(JoinResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.nodes.v1.NodeService/Join", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeServiceClient) Forget(ctx context.Context, in *ForgetRequest, opts ...grpc.CallOption) (*ForgetResponse, error) {
	out := new(ForgetResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.nodes.v1.NodeService/Forget", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeServiceClient) Connect(ctx context.Context, opts ...grpc.CallOption) (NodeService_ConnectClient, error) {
	stream, err := c.cc.NewStream(ctx, &NodeService_ServiceDesc.Streams[0], "/blipblop.services.nodes.v1.NodeService/Connect", opts...)
	if err != nil {
		return nil, err
	}
	x := &nodeServiceConnectClient{stream}
	return x, nil
}

type NodeService_ConnectClient interface {
	Send(*v1.Event) error
	Recv() (*v1.Event, error)
	grpc.ClientStream
}

type nodeServiceConnectClient struct {
	grpc.ClientStream
}

func (x *nodeServiceConnectClient) Send(m *v1.Event) error {
	return x.ClientStream.SendMsg(m)
}

func (x *nodeServiceConnectClient) Recv() (*v1.Event, error) {
	m := new(v1.Event)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// NodeServiceServer is the server API for NodeService service.
// All implementations must embed UnimplementedNodeServiceServer
// for forward compatibility
type NodeServiceServer interface {
	List(context.Context, *ListNodeRequest) (*ListNodeResponse, error)
	Get(context.Context, *GetNodeRequest) (*GetNodeResponse, error)
	Create(context.Context, *CreateNodeRequest) (*CreateNodeResponse, error)
	Update(context.Context, *UpdateNodeRequest) (*UpdateNodeResponse, error)
	Delete(context.Context, *DeleteNodeRequest) (*DeleteNodeResponse, error)
	Join(context.Context, *JoinRequest) (*JoinResponse, error)
	Forget(context.Context, *ForgetRequest) (*ForgetResponse, error)
	Connect(NodeService_ConnectServer) error
	mustEmbedUnimplementedNodeServiceServer()
}

// UnimplementedNodeServiceServer must be embedded to have forward compatible implementations.
type UnimplementedNodeServiceServer struct {
}

func (UnimplementedNodeServiceServer) List(context.Context, *ListNodeRequest) (*ListNodeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedNodeServiceServer) Get(context.Context, *GetNodeRequest) (*GetNodeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (UnimplementedNodeServiceServer) Create(context.Context, *CreateNodeRequest) (*CreateNodeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}
func (UnimplementedNodeServiceServer) Update(context.Context, *UpdateNodeRequest) (*UpdateNodeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedNodeServiceServer) Delete(context.Context, *DeleteNodeRequest) (*DeleteNodeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedNodeServiceServer) Join(context.Context, *JoinRequest) (*JoinResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Join not implemented")
}
func (UnimplementedNodeServiceServer) Forget(context.Context, *ForgetRequest) (*ForgetResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Forget not implemented")
}
func (UnimplementedNodeServiceServer) Connect(NodeService_ConnectServer) error {
	return status.Errorf(codes.Unimplemented, "method Connect not implemented")
}
func (UnimplementedNodeServiceServer) mustEmbedUnimplementedNodeServiceServer() {}

// UnsafeNodeServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to NodeServiceServer will
// result in compilation errors.
type UnsafeNodeServiceServer interface {
	mustEmbedUnimplementedNodeServiceServer()
}

func RegisterNodeServiceServer(s grpc.ServiceRegistrar, srv NodeServiceServer) {
	s.RegisterService(&NodeService_ServiceDesc, srv)
}

func _NodeService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListNodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.nodes.v1.NodeService/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServiceServer).List(ctx, req.(*ListNodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _NodeService_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetNodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServiceServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.nodes.v1.NodeService/Get",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServiceServer).Get(ctx, req.(*GetNodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _NodeService_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateNodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServiceServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.nodes.v1.NodeService/Create",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServiceServer).Create(ctx, req.(*CreateNodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _NodeService_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateNodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServiceServer).Update(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.nodes.v1.NodeService/Update",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServiceServer).Update(ctx, req.(*UpdateNodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _NodeService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteNodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.nodes.v1.NodeService/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServiceServer).Delete(ctx, req.(*DeleteNodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _NodeService_Join_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(JoinRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServiceServer).Join(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.nodes.v1.NodeService/Join",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServiceServer).Join(ctx, req.(*JoinRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _NodeService_Forget_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ForgetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServiceServer).Forget(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.nodes.v1.NodeService/Forget",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServiceServer).Forget(ctx, req.(*ForgetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _NodeService_Connect_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(NodeServiceServer).Connect(&nodeServiceConnectServer{stream})
}

type NodeService_ConnectServer interface {
	Send(*v1.Event) error
	Recv() (*v1.Event, error)
	grpc.ServerStream
}

type nodeServiceConnectServer struct {
	grpc.ServerStream
}

func (x *nodeServiceConnectServer) Send(m *v1.Event) error {
	return x.ServerStream.SendMsg(m)
}

func (x *nodeServiceConnectServer) Recv() (*v1.Event, error) {
	m := new(v1.Event)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// NodeService_ServiceDesc is the grpc.ServiceDesc for NodeService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var NodeService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "blipblop.services.nodes.v1.NodeService",
	HandlerType: (*NodeServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "List",
			Handler:    _NodeService_List_Handler,
		},
		{
			MethodName: "Get",
			Handler:    _NodeService_Get_Handler,
		},
		{
			MethodName: "Create",
			Handler:    _NodeService_Create_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _NodeService_Update_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _NodeService_Delete_Handler,
		},
		{
			MethodName: "Join",
			Handler:    _NodeService_Join_Handler,
		},
		{
			MethodName: "Forget",
			Handler:    _NodeService_Forget_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Connect",
			Handler:       _NodeService_Connect_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "api/services/nodes/v1/nodes.proto",
}
