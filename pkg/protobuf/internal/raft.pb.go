// Code generated by protoc-gen-go. DO NOT EDIT.
// source: raft.proto

package anndb_proto_internal

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

func init() {
	proto.RegisterFile("raft.proto", fileDescriptor_b042552c306ae59b)
}

var fileDescriptor_b042552c306ae59b = []byte{
	// 77 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x2a, 0x4a, 0x4c, 0x2b,
	0xd1, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x12, 0x49, 0xcc, 0xcb, 0x4b, 0x49, 0x8a, 0x07, 0x73,
	0xe2, 0x33, 0xf3, 0x4a, 0x52, 0x8b, 0xf2, 0x12, 0x73, 0x8c, 0xf8, 0xb9, 0x78, 0x83, 0x12, 0xd3,
	0x4a, 0x42, 0x8a, 0x12, 0xf3, 0x8a, 0x0b, 0xf2, 0x8b, 0x4a, 0x92, 0xd8, 0xc0, 0x0a, 0x8c, 0x01,
	0x01, 0x00, 0x00, 0xff, 0xff, 0xe8, 0x3b, 0x78, 0xc8, 0x3b, 0x00, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// RaftTransportClient is the client API for RaftTransport service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type RaftTransportClient interface {
}

type raftTransportClient struct {
	cc grpc.ClientConnInterface
}

func NewRaftTransportClient(cc grpc.ClientConnInterface) RaftTransportClient {
	return &raftTransportClient{cc}
}

// RaftTransportServer is the server API for RaftTransport service.
type RaftTransportServer interface {
}

// UnimplementedRaftTransportServer can be embedded to have forward compatible implementations.
type UnimplementedRaftTransportServer struct {
}

func RegisterRaftTransportServer(s *grpc.Server, srv RaftTransportServer) {
	s.RegisterService(&_RaftTransport_serviceDesc, srv)
}

var _RaftTransport_serviceDesc = grpc.ServiceDesc{
	ServiceName: "anndb_proto_internal.RaftTransport",
	HandlerType: (*RaftTransportServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams:     []grpc.StreamDesc{},
	Metadata:    "raft.proto",
}
