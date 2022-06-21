// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.1
// source: event.proto

package proto

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

// EventServiceClient is the client API for EventService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type EventServiceClient interface {
	Subscribe(ctx context.Context, in *SubscribeRequest, opts ...grpc.CallOption) (EventService_SubscribeClient, error)
	FireEvent(ctx context.Context, opts ...grpc.CallOption) (EventService_FireEventClient, error)
}

type eventServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewEventServiceClient(cc grpc.ClientConnInterface) EventServiceClient {
	return &eventServiceClient{cc}
}

func (c *eventServiceClient) Subscribe(ctx context.Context, in *SubscribeRequest, opts ...grpc.CallOption) (EventService_SubscribeClient, error) {
	stream, err := c.cc.NewStream(ctx, &EventService_ServiceDesc.Streams[0], "/eventservice.EventService/Subscribe", opts...)
	if err != nil {
		return nil, err
	}
	x := &eventServiceSubscribeClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type EventService_SubscribeClient interface {
	Recv() (*Event, error)
	grpc.ClientStream
}

type eventServiceSubscribeClient struct {
	grpc.ClientStream
}

func (x *eventServiceSubscribeClient) Recv() (*Event, error) {
	m := new(Event)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *eventServiceClient) FireEvent(ctx context.Context, opts ...grpc.CallOption) (EventService_FireEventClient, error) {
	stream, err := c.cc.NewStream(ctx, &EventService_ServiceDesc.Streams[1], "/eventservice.EventService/FireEvent", opts...)
	if err != nil {
		return nil, err
	}
	x := &eventServiceFireEventClient{stream}
	return x, nil
}

type EventService_FireEventClient interface {
	Send(*Event) error
	CloseAndRecv() (*EventAck, error)
	grpc.ClientStream
}

type eventServiceFireEventClient struct {
	grpc.ClientStream
}

func (x *eventServiceFireEventClient) Send(m *Event) error {
	return x.ClientStream.SendMsg(m)
}

func (x *eventServiceFireEventClient) CloseAndRecv() (*EventAck, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(EventAck)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// EventServiceServer is the server API for EventService service.
// All implementations must embed UnimplementedEventServiceServer
// for forward compatibility
type EventServiceServer interface {
	Subscribe(*SubscribeRequest, EventService_SubscribeServer) error
	FireEvent(EventService_FireEventServer) error
	mustEmbedUnimplementedEventServiceServer()
}

// UnimplementedEventServiceServer must be embedded to have forward compatible implementations.
type UnimplementedEventServiceServer struct {
}

func (UnimplementedEventServiceServer) Subscribe(*SubscribeRequest, EventService_SubscribeServer) error {
	return status.Errorf(codes.Unimplemented, "method Subscribe not implemented")
}
func (UnimplementedEventServiceServer) FireEvent(EventService_FireEventServer) error {
	return status.Errorf(codes.Unimplemented, "method FireEvent not implemented")
}
func (UnimplementedEventServiceServer) mustEmbedUnimplementedEventServiceServer() {}

// UnsafeEventServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to EventServiceServer will
// result in compilation errors.
type UnsafeEventServiceServer interface {
	mustEmbedUnimplementedEventServiceServer()
}

func RegisterEventServiceServer(s grpc.ServiceRegistrar, srv EventServiceServer) {
	s.RegisterService(&EventService_ServiceDesc, srv)
}

func _EventService_Subscribe_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SubscribeRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(EventServiceServer).Subscribe(m, &eventServiceSubscribeServer{stream})
}

type EventService_SubscribeServer interface {
	Send(*Event) error
	grpc.ServerStream
}

type eventServiceSubscribeServer struct {
	grpc.ServerStream
}

func (x *eventServiceSubscribeServer) Send(m *Event) error {
	return x.ServerStream.SendMsg(m)
}

func _EventService_FireEvent_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(EventServiceServer).FireEvent(&eventServiceFireEventServer{stream})
}

type EventService_FireEventServer interface {
	SendAndClose(*EventAck) error
	Recv() (*Event, error)
	grpc.ServerStream
}

type eventServiceFireEventServer struct {
	grpc.ServerStream
}

func (x *eventServiceFireEventServer) SendAndClose(m *EventAck) error {
	return x.ServerStream.SendMsg(m)
}

func (x *eventServiceFireEventServer) Recv() (*Event, error) {
	m := new(Event)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// EventService_ServiceDesc is the grpc.ServiceDesc for EventService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var EventService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "eventservice.EventService",
	HandlerType: (*EventServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Subscribe",
			Handler:       _EventService_Subscribe_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "FireEvent",
			Handler:       _EventService_FireEvent_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "event.proto",
}
