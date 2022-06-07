// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0-devel
// 	protoc        v3.21.1
// source: proto/node.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type EventType int32

const (
	EventType_ContainerCreate EventType = 0
	EventType_ContainerDelete EventType = 1
	EventType_ContainerUpdate EventType = 2
)

// Enum value maps for EventType.
var (
	EventType_name = map[int32]string{
		0: "ContainerCreate",
		1: "ContainerDelete",
		2: "ContainerUpdate",
	}
	EventType_value = map[string]int32{
		"ContainerCreate": 0,
		"ContainerDelete": 1,
		"ContainerUpdate": 2,
	}
)

func (x EventType) Enum() *EventType {
	p := new(EventType)
	*p = x
	return p
}

func (x EventType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (EventType) Descriptor() protoreflect.EnumDescriptor {
	return file_proto_node_proto_enumTypes[0].Descriptor()
}

func (EventType) Type() protoreflect.EnumType {
	return &file_proto_node_proto_enumTypes[0]
}

func (x EventType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use EventType.Descriptor instead.
func (EventType) EnumDescriptor() ([]byte, []int) {
	return file_proto_node_proto_rawDescGZIP(), []int{0}
}

type Status int32

const (
	Status_JoinFail    Status = 0
	Status_JoinSuccess Status = 1
)

// Enum value maps for Status.
var (
	Status_name = map[int32]string{
		0: "JoinFail",
		1: "JoinSuccess",
	}
	Status_value = map[string]int32{
		"JoinFail":    0,
		"JoinSuccess": 1,
	}
)

func (x Status) Enum() *Status {
	p := new(Status)
	*p = x
	return p
}

func (x Status) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Status) Descriptor() protoreflect.EnumDescriptor {
	return file_proto_node_proto_enumTypes[1].Descriptor()
}

func (Status) Type() protoreflect.EnumType {
	return &file_proto_node_proto_enumTypes[1]
}

func (x Status) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Status.Descriptor instead.
func (Status) EnumDescriptor() ([]byte, []int) {
	return file_proto_node_proto_rawDescGZIP(), []int{1}
}

type Node struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
}

func (x *Node) Reset() {
	*x = Node{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_node_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Node) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Node) ProtoMessage() {}

func (x *Node) ProtoReflect() protoreflect.Message {
	mi := &file_proto_node_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Node.ProtoReflect.Descriptor instead.
func (*Node) Descriptor() ([]byte, []int) {
	return file_proto_node_proto_rawDescGZIP(), []int{0}
}

func (x *Node) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

type Event struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name    string    `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Node    *Node     `protobuf:"bytes,2,opt,name=node,proto3" json:"node,omitempty"`
	Payload []byte    `protobuf:"bytes,3,opt,name=payload,proto3" json:"payload,omitempty"`
	Type    EventType `protobuf:"varint,4,opt,name=type,proto3,enum=nodeservice.EventType" json:"type,omitempty"`
}

func (x *Event) Reset() {
	*x = Event{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_node_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Event) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Event) ProtoMessage() {}

func (x *Event) ProtoReflect() protoreflect.Message {
	mi := &file_proto_node_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Event.ProtoReflect.Descriptor instead.
func (*Event) Descriptor() ([]byte, []int) {
	return file_proto_node_proto_rawDescGZIP(), []int{1}
}

func (x *Event) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Event) GetNode() *Node {
	if x != nil {
		return x.Node
	}
	return nil
}

func (x *Event) GetPayload() []byte {
	if x != nil {
		return x.Payload
	}
	return nil
}

func (x *Event) GetType() EventType {
	if x != nil {
		return x.Type
	}
	return EventType_ContainerCreate
}

type EventAck struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Status string `protobuf:"bytes,1,opt,name=status,proto3" json:"status,omitempty"`
}

func (x *EventAck) Reset() {
	*x = EventAck{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_node_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EventAck) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventAck) ProtoMessage() {}

func (x *EventAck) ProtoReflect() protoreflect.Message {
	mi := &file_proto_node_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventAck.ProtoReflect.Descriptor instead.
func (*EventAck) Descriptor() ([]byte, []int) {
	return file_proto_node_proto_rawDescGZIP(), []int{2}
}

func (x *EventAck) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

type JoinRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	At   *timestamppb.Timestamp `protobuf:"bytes,1,opt,name=at,proto3" json:"at,omitempty"`
	Node *Node                  `protobuf:"bytes,2,opt,name=node,proto3" json:"node,omitempty"`
}

func (x *JoinRequest) Reset() {
	*x = JoinRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_node_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *JoinRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*JoinRequest) ProtoMessage() {}

func (x *JoinRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_node_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use JoinRequest.ProtoReflect.Descriptor instead.
func (*JoinRequest) Descriptor() ([]byte, []int) {
	return file_proto_node_proto_rawDescGZIP(), []int{3}
}

func (x *JoinRequest) GetAt() *timestamppb.Timestamp {
	if x != nil {
		return x.At
	}
	return nil
}

func (x *JoinRequest) GetNode() *Node {
	if x != nil {
		return x.Node
	}
	return nil
}

type JoinResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	At     *timestamppb.Timestamp `protobuf:"bytes,1,opt,name=at,proto3" json:"at,omitempty"`
	Node   *Node                  `protobuf:"bytes,2,opt,name=node,proto3" json:"node,omitempty"`
	Status Status                 `protobuf:"varint,3,opt,name=status,proto3,enum=nodeservice.Status" json:"status,omitempty"`
}

func (x *JoinResponse) Reset() {
	*x = JoinResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_node_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *JoinResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*JoinResponse) ProtoMessage() {}

func (x *JoinResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_node_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use JoinResponse.ProtoReflect.Descriptor instead.
func (*JoinResponse) Descriptor() ([]byte, []int) {
	return file_proto_node_proto_rawDescGZIP(), []int{4}
}

func (x *JoinResponse) GetAt() *timestamppb.Timestamp {
	if x != nil {
		return x.At
	}
	return nil
}

func (x *JoinResponse) GetNode() *Node {
	if x != nil {
		return x.Node
	}
	return nil
}

func (x *JoinResponse) GetStatus() Status {
	if x != nil {
		return x.Status
	}
	return Status_JoinFail
}

var File_proto_node_proto protoreflect.FileDescriptor

var file_proto_node_proto_rawDesc = []byte{
	0x0a, 0x10, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6e, 0x6f, 0x64, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x0b, 0x6e, 0x6f, 0x64, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x1a,
	0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0x16, 0x0a, 0x04, 0x4e, 0x6f, 0x64, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x22, 0x88, 0x01, 0x0a, 0x05, 0x45, 0x76, 0x65,
	0x6e, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x25, 0x0a, 0x04, 0x6e, 0x6f, 0x64, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x52, 0x04, 0x6e, 0x6f, 0x64, 0x65, 0x12, 0x18, 0x0a,
	0x07, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x07,
	0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x12, 0x2a, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x16, 0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x73, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x74,
	0x79, 0x70, 0x65, 0x22, 0x22, 0x0a, 0x08, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x41, 0x63, 0x6b, 0x12,
	0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0x60, 0x0a, 0x0b, 0x4a, 0x6f, 0x69, 0x6e, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x2a, 0x0a, 0x02, 0x61, 0x74, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x02,
	0x61, 0x74, 0x12, 0x25, 0x0a, 0x04, 0x6e, 0x6f, 0x64, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x11, 0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x4e,
	0x6f, 0x64, 0x65, 0x52, 0x04, 0x6e, 0x6f, 0x64, 0x65, 0x22, 0x8e, 0x01, 0x0a, 0x0c, 0x4a, 0x6f,
	0x69, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x2a, 0x0a, 0x02, 0x61, 0x74,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61,
	0x6d, 0x70, 0x52, 0x02, 0x61, 0x74, 0x12, 0x25, 0x0a, 0x04, 0x6e, 0x6f, 0x64, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x52, 0x04, 0x6e, 0x6f, 0x64, 0x65, 0x12, 0x2b, 0x0a,
	0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x13, 0x2e,
	0x6e, 0x6f, 0x64, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x53, 0x74, 0x61, 0x74,
	0x75, 0x73, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x2a, 0x4a, 0x0a, 0x09, 0x45, 0x76,
	0x65, 0x6e, 0x74, 0x54, 0x79, 0x70, 0x65, 0x12, 0x13, 0x0a, 0x0f, 0x43, 0x6f, 0x6e, 0x74, 0x61,
	0x69, 0x6e, 0x65, 0x72, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x10, 0x00, 0x12, 0x13, 0x0a, 0x0f,
	0x43, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x10,
	0x01, 0x12, 0x13, 0x0a, 0x0f, 0x43, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x55, 0x70,
	0x64, 0x61, 0x74, 0x65, 0x10, 0x02, 0x2a, 0x27, 0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73,
	0x12, 0x0c, 0x0a, 0x08, 0x4a, 0x6f, 0x69, 0x6e, 0x46, 0x61, 0x69, 0x6c, 0x10, 0x00, 0x12, 0x0f,
	0x0a, 0x0b, 0x4a, 0x6f, 0x69, 0x6e, 0x53, 0x75, 0x63, 0x63, 0x65, 0x73, 0x73, 0x10, 0x01, 0x32,
	0xc0, 0x01, 0x0a, 0x0b, 0x4e, 0x6f, 0x64, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12,
	0x3d, 0x0a, 0x04, 0x4a, 0x6f, 0x69, 0x6e, 0x12, 0x18, 0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x4a, 0x6f, 0x69, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x19, 0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e,
	0x4a, 0x6f, 0x69, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x36,
	0x0a, 0x09, 0x53, 0x75, 0x62, 0x73, 0x63, 0x72, 0x69, 0x62, 0x65, 0x12, 0x11, 0x2e, 0x6e, 0x6f,
	0x64, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x1a, 0x12,
	0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x45, 0x76, 0x65,
	0x6e, 0x74, 0x22, 0x00, 0x30, 0x01, 0x12, 0x3a, 0x0a, 0x09, 0x46, 0x69, 0x72, 0x65, 0x45, 0x76,
	0x65, 0x6e, 0x74, 0x12, 0x12, 0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x1a, 0x15, 0x2e, 0x6e, 0x6f, 0x64, 0x65, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x41, 0x63, 0x6b, 0x22, 0x00,
	0x28, 0x01, 0x42, 0x22, 0x5a, 0x20, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x61, 0x6d, 0x69, 0x6d, 0x6f, 0x66, 0x2f, 0x62, 0x6c, 0x69, 0x70, 0x62, 0x6c, 0x6f, 0x70,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_node_proto_rawDescOnce sync.Once
	file_proto_node_proto_rawDescData = file_proto_node_proto_rawDesc
)

func file_proto_node_proto_rawDescGZIP() []byte {
	file_proto_node_proto_rawDescOnce.Do(func() {
		file_proto_node_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_node_proto_rawDescData)
	})
	return file_proto_node_proto_rawDescData
}

var file_proto_node_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_proto_node_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_proto_node_proto_goTypes = []interface{}{
	(EventType)(0),                // 0: nodeservice.EventType
	(Status)(0),                   // 1: nodeservice.Status
	(*Node)(nil),                  // 2: nodeservice.Node
	(*Event)(nil),                 // 3: nodeservice.Event
	(*EventAck)(nil),              // 4: nodeservice.EventAck
	(*JoinRequest)(nil),           // 5: nodeservice.JoinRequest
	(*JoinResponse)(nil),          // 6: nodeservice.JoinResponse
	(*timestamppb.Timestamp)(nil), // 7: google.protobuf.Timestamp
}
var file_proto_node_proto_depIdxs = []int32{
	2,  // 0: nodeservice.Event.node:type_name -> nodeservice.Node
	0,  // 1: nodeservice.Event.type:type_name -> nodeservice.EventType
	7,  // 2: nodeservice.JoinRequest.at:type_name -> google.protobuf.Timestamp
	2,  // 3: nodeservice.JoinRequest.node:type_name -> nodeservice.Node
	7,  // 4: nodeservice.JoinResponse.at:type_name -> google.protobuf.Timestamp
	2,  // 5: nodeservice.JoinResponse.node:type_name -> nodeservice.Node
	1,  // 6: nodeservice.JoinResponse.status:type_name -> nodeservice.Status
	5,  // 7: nodeservice.NodeService.Join:input_type -> nodeservice.JoinRequest
	2,  // 8: nodeservice.NodeService.Subscribe:input_type -> nodeservice.Node
	3,  // 9: nodeservice.NodeService.FireEvent:input_type -> nodeservice.Event
	6,  // 10: nodeservice.NodeService.Join:output_type -> nodeservice.JoinResponse
	3,  // 11: nodeservice.NodeService.Subscribe:output_type -> nodeservice.Event
	4,  // 12: nodeservice.NodeService.FireEvent:output_type -> nodeservice.EventAck
	10, // [10:13] is the sub-list for method output_type
	7,  // [7:10] is the sub-list for method input_type
	7,  // [7:7] is the sub-list for extension type_name
	7,  // [7:7] is the sub-list for extension extendee
	0,  // [0:7] is the sub-list for field type_name
}

func init() { file_proto_node_proto_init() }
func file_proto_node_proto_init() {
	if File_proto_node_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_node_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Node); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_node_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Event); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_node_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EventAck); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_node_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*JoinRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_node_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*JoinResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_node_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_node_proto_goTypes,
		DependencyIndexes: file_proto_node_proto_depIdxs,
		EnumInfos:         file_proto_node_proto_enumTypes,
		MessageInfos:      file_proto_node_proto_msgTypes,
	}.Build()
	File_proto_node_proto = out.File
	file_proto_node_proto_rawDesc = nil
	file_proto_node_proto_goTypes = nil
	file_proto_node_proto_depIdxs = nil
}
