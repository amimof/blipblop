// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.1
// source: containers.proto

package containers

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ContainerServiceClient is the client API for ContainerService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ContainerServiceClient interface {
	Get(ctx context.Context, in *GetContainerRequest, opts ...grpc.CallOption) (*GetContainerResponse, error)
	List(ctx context.Context, in *ListContainerRequest, opts ...grpc.CallOption) (*ListContainerResponse, error)
	Create(ctx context.Context, in *CreateContainerRequest, opts ...grpc.CallOption) (*CreateContainerResponse, error)
	Update(ctx context.Context, in *UpdateContainerRequest, opts ...grpc.CallOption) (*UpdateContainerResponse, error)
	Delete(ctx context.Context, in *DeleteContainerRequest, opts ...grpc.CallOption) (*DeleteContainerResponse, error)
	Start(ctx context.Context, in *StartContainerRequest, opts ...grpc.CallOption) (*StartContainerResponse, error)
	Stop(ctx context.Context, in *StopContainerRequest, opts ...grpc.CallOption) (*StopContainerResponse, error)
	Kill(ctx context.Context, in *KillContainerRequest, opts ...grpc.CallOption) (*KillContainerResponse, error)
}

type containerServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewContainerServiceClient(cc grpc.ClientConnInterface) ContainerServiceClient {
	return &containerServiceClient{cc}
}

func (c *containerServiceClient) Get(ctx context.Context, in *GetContainerRequest, opts ...grpc.CallOption) (*GetContainerResponse, error) {
	out := new(GetContainerResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.containers.v1.ContainerService/Get", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *containerServiceClient) List(ctx context.Context, in *ListContainerRequest, opts ...grpc.CallOption) (*ListContainerResponse, error) {
	out := new(ListContainerResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.containers.v1.ContainerService/List", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *containerServiceClient) Create(ctx context.Context, in *CreateContainerRequest, opts ...grpc.CallOption) (*CreateContainerResponse, error) {
	out := new(CreateContainerResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.containers.v1.ContainerService/Create", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *containerServiceClient) Update(ctx context.Context, in *UpdateContainerRequest, opts ...grpc.CallOption) (*UpdateContainerResponse, error) {
	out := new(UpdateContainerResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.containers.v1.ContainerService/Update", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *containerServiceClient) Delete(ctx context.Context, in *DeleteContainerRequest, opts ...grpc.CallOption) (*DeleteContainerResponse, error) {
	out := new(DeleteContainerResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.containers.v1.ContainerService/Delete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *containerServiceClient) Start(ctx context.Context, in *StartContainerRequest, opts ...grpc.CallOption) (*StartContainerResponse, error) {
	out := new(StartContainerResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.containers.v1.ContainerService/Start", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *containerServiceClient) Stop(ctx context.Context, in *StopContainerRequest, opts ...grpc.CallOption) (*StopContainerResponse, error) {
	out := new(StopContainerResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.containers.v1.ContainerService/Stop", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *containerServiceClient) Kill(ctx context.Context, in *KillContainerRequest, opts ...grpc.CallOption) (*KillContainerResponse, error) {
	out := new(KillContainerResponse)
	err := c.cc.Invoke(ctx, "/blipblop.services.containers.v1.ContainerService/Kill", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ContainerServiceServer is the server API for ContainerService service.
// All implementations must embed UnimplementedContainerServiceServer
// for forward compatibility
type ContainerServiceServer interface {
	Get(context.Context, *GetContainerRequest) (*GetContainerResponse, error)
	List(context.Context, *ListContainerRequest) (*ListContainerResponse, error)
	Create(context.Context, *CreateContainerRequest) (*CreateContainerResponse, error)
	Update(context.Context, *UpdateContainerRequest) (*UpdateContainerResponse, error)
	Delete(context.Context, *DeleteContainerRequest) (*DeleteContainerResponse, error)
	Start(context.Context, *StartContainerRequest) (*StartContainerResponse, error)
	Stop(context.Context, *StopContainerRequest) (*StopContainerResponse, error)
	Kill(context.Context, *KillContainerRequest) (*KillContainerResponse, error)
	mustEmbedUnimplementedContainerServiceServer()
}

// UnimplementedContainerServiceServer must be embedded to have forward compatible implementations.
type UnimplementedContainerServiceServer struct {
}

func (UnimplementedContainerServiceServer) Get(context.Context, *GetContainerRequest) (*GetContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (UnimplementedContainerServiceServer) List(context.Context, *ListContainerRequest) (*ListContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedContainerServiceServer) Create(context.Context, *CreateContainerRequest) (*CreateContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}
func (UnimplementedContainerServiceServer) Update(context.Context, *UpdateContainerRequest) (*UpdateContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedContainerServiceServer) Delete(context.Context, *DeleteContainerRequest) (*DeleteContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedContainerServiceServer) Start(context.Context, *StartContainerRequest) (*StartContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Start not implemented")
}
func (UnimplementedContainerServiceServer) Stop(context.Context, *StopContainerRequest) (*StopContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Stop not implemented")
}
func (UnimplementedContainerServiceServer) Kill(context.Context, *KillContainerRequest) (*KillContainerResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Kill not implemented")
}
func (UnimplementedContainerServiceServer) mustEmbedUnimplementedContainerServiceServer() {}

// UnsafeContainerServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ContainerServiceServer will
// result in compilation errors.
type UnsafeContainerServiceServer interface {
	mustEmbedUnimplementedContainerServiceServer()
}

func RegisterContainerServiceServer(s grpc.ServiceRegistrar, srv ContainerServiceServer) {
	s.RegisterService(&ContainerService_ServiceDesc, srv)
}

func _ContainerService_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetContainerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ContainerServiceServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.containers.v1.ContainerService/Get",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ContainerServiceServer).Get(ctx, req.(*GetContainerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ContainerService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListContainerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ContainerServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.containers.v1.ContainerService/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ContainerServiceServer).List(ctx, req.(*ListContainerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ContainerService_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateContainerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ContainerServiceServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.containers.v1.ContainerService/Create",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ContainerServiceServer).Create(ctx, req.(*CreateContainerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ContainerService_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateContainerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ContainerServiceServer).Update(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.containers.v1.ContainerService/Update",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ContainerServiceServer).Update(ctx, req.(*UpdateContainerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ContainerService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteContainerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ContainerServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.containers.v1.ContainerService/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ContainerServiceServer).Delete(ctx, req.(*DeleteContainerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ContainerService_Start_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StartContainerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ContainerServiceServer).Start(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.containers.v1.ContainerService/Start",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ContainerServiceServer).Start(ctx, req.(*StartContainerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ContainerService_Stop_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StopContainerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ContainerServiceServer).Stop(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.containers.v1.ContainerService/Stop",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ContainerServiceServer).Stop(ctx, req.(*StopContainerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ContainerService_Kill_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(KillContainerRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ContainerServiceServer).Kill(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/blipblop.services.containers.v1.ContainerService/Kill",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ContainerServiceServer).Kill(ctx, req.(*KillContainerRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ContainerService_ServiceDesc is the grpc.ServiceDesc for ContainerService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ContainerService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "blipblop.services.containers.v1.ContainerService",
	HandlerType: (*ContainerServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Get",
			Handler:    _ContainerService_Get_Handler,
		},
		{
			MethodName: "List",
			Handler:    _ContainerService_List_Handler,
		},
		{
			MethodName: "Create",
			Handler:    _ContainerService_Create_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _ContainerService_Update_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _ContainerService_Delete_Handler,
		},
		{
			MethodName: "Start",
			Handler:    _ContainerService_Start_Handler,
		},
		{
			MethodName: "Stop",
			Handler:    _ContainerService_Stop_Handler,
		},
		{
			MethodName: "Kill",
			Handler:    _ContainerService_Kill_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "containers.proto",
}
