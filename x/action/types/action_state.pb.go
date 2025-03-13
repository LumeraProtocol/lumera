// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: lumera/action/action_state.proto

package types

import (
	fmt "fmt"
	_ "github.com/cosmos/gogoproto/gogoproto"
	proto "github.com/cosmos/gogoproto/proto"
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
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type ActionState int32

const (
	ActionStateUnspecified ActionState = 0
	ActionStatePending     ActionState = 1
	ActionStateProcessing  ActionState = 2
	ActionStateDone        ActionState = 3
	ActionStateApproved    ActionState = 4
	ActionStateRejected    ActionState = 5
	ActionStateFailed      ActionState = 6
	ActionStateExpired     ActionState = 7
)

var ActionState_name = map[int32]string{
	0: "ACTION_STATE_UNSPECIFIED",
	1: "ACTION_STATE_PENDING",
	2: "ACTION_STATE_PROCESSING",
	3: "ACTION_STATE_DONE",
	4: "ACTION_STATE_APPROVED",
	5: "ACTION_STATE_REJECTED",
	6: "ACTION_STATE_FAILED",
	7: "ACTION_STATE_EXPIRED",
}

var ActionState_value = map[string]int32{
	"ACTION_STATE_UNSPECIFIED": 0,
	"ACTION_STATE_PENDING":     1,
	"ACTION_STATE_PROCESSING":  2,
	"ACTION_STATE_DONE":        3,
	"ACTION_STATE_APPROVED":    4,
	"ACTION_STATE_REJECTED":    5,
	"ACTION_STATE_FAILED":      6,
	"ACTION_STATE_EXPIRED":     7,
}

func (x ActionState) String() string {
	return proto.EnumName(ActionState_name, int32(x))
}

func (ActionState) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_dbbc51526cb106aa, []int{0}
}

func init() {
	proto.RegisterEnum("lumera.action.ActionState", ActionState_name, ActionState_value)
}

func init() { proto.RegisterFile("lumera/action/action_state.proto", fileDescriptor_dbbc51526cb106aa) }

var fileDescriptor_dbbc51526cb106aa = []byte{
	// 391 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0xd2, 0x4f, 0x8f, 0x93, 0x40,
	0x18, 0x06, 0x70, 0xb0, 0x5a, 0xcd, 0x18, 0x23, 0xa5, 0xff, 0x74, 0x0e, 0x13, 0xce, 0x3d, 0x80,
	0xd1, 0xc4, 0x78, 0x45, 0x66, 0x6a, 0x30, 0x0d, 0x10, 0xa0, 0xc6, 0x78, 0x69, 0x28, 0x8c, 0x88,
	0x69, 0x19, 0x02, 0xd4, 0xd4, 0x6f, 0x60, 0x38, 0xf9, 0x05, 0x38, 0xed, 0x1e, 0xf6, 0xba, 0xdf,
	0x62, 0x8f, 0x3d, 0xee, 0x71, 0xd3, 0x7e, 0x91, 0x0d, 0xb4, 0x9b, 0x40, 0xba, 0xa7, 0x99, 0xe4,
	0x7d, 0x7e, 0x97, 0xf7, 0x7d, 0x80, 0xb4, 0xda, 0xac, 0x69, 0xea, 0x29, 0x9e, 0x9f, 0x47, 0x2c,
	0x3e, 0x3d, 0x8b, 0x2c, 0xf7, 0x72, 0x2a, 0x27, 0x29, 0xcb, 0x99, 0xf8, 0xea, 0x98, 0x90, 0x8f,
	0x23, 0x38, 0x08, 0x59, 0xc8, 0xea, 0x89, 0x52, 0xfd, 0x8e, 0xa1, 0xc9, 0x75, 0x07, 0xbc, 0x54,
	0xeb, 0x80, 0x53, 0x51, 0xf1, 0x13, 0x78, 0xa3, 0x6a, 0xae, 0x6e, 0x1a, 0x0b, 0xc7, 0x55, 0x5d,
	0xb2, 0x98, 0x1b, 0x8e, 0x45, 0x34, 0x7d, 0xaa, 0x13, 0x2c, 0x70, 0x10, 0x16, 0xa5, 0x34, 0x6a,
	0xc4, 0xe7, 0x71, 0x96, 0x50, 0x3f, 0xfa, 0x19, 0xd1, 0x40, 0x7c, 0x07, 0x06, 0x2d, 0x69, 0x11,
	0x03, 0xeb, 0xc6, 0x17, 0x81, 0x87, 0xa3, 0xa2, 0x94, 0xc4, 0x86, 0xb2, 0x68, 0x1c, 0x44, 0x71,
	0x28, 0x7e, 0x04, 0xe3, 0xb6, 0xb0, 0x4d, 0x8d, 0x38, 0x4e, 0x85, 0x9e, 0xc0, 0xb7, 0x45, 0x29,
	0x0d, 0x9b, 0x28, 0x65, 0x3e, 0xcd, 0xb2, 0xca, 0x4d, 0x40, 0xaf, 0xe5, 0xb0, 0x69, 0x10, 0xa1,
	0x03, 0xfb, 0x45, 0x29, 0xbd, 0x6e, 0x08, 0xcc, 0x62, 0x2a, 0xbe, 0x07, 0xc3, 0x56, 0x56, 0xb5,
	0x2c, 0xdb, 0xfc, 0x46, 0xb0, 0xf0, 0x14, 0x8e, 0x8b, 0x52, 0xea, 0x37, 0xf2, 0x6a, 0x92, 0xa4,
	0xec, 0x0f, 0x0d, 0xce, 0x8c, 0x4d, 0xbe, 0x12, 0xcd, 0x25, 0x58, 0x78, 0x76, 0x66, 0x6c, 0xfa,
	0x9b, 0xfa, 0x39, 0x0d, 0x44, 0x19, 0xf4, 0x5b, 0x66, 0xaa, 0xea, 0x33, 0x82, 0x85, 0x2e, 0x1c,
	0x16, 0xa5, 0xd4, 0x6b, 0x88, 0xa9, 0x17, 0xad, 0x1e, 0xd9, 0x16, 0xf9, 0x6e, 0xe9, 0x36, 0xc1,
	0xc2, 0xf3, 0xb3, 0x6d, 0x91, 0x6d, 0x12, 0xa5, 0x34, 0x80, 0x2f, 0xfe, 0x5d, 0x20, 0xee, 0xea,
	0x12, 0xf1, 0x9f, 0xf5, 0x9b, 0x3d, 0xe2, 0x77, 0x7b, 0xc4, 0xdf, 0xed, 0x11, 0xff, 0xff, 0x80,
	0xb8, 0xdd, 0x01, 0x71, 0xb7, 0x07, 0xc4, 0xfd, 0x50, 0xc2, 0x28, 0xff, 0xb5, 0x59, 0xca, 0x3e,
	0x5b, 0x2b, 0xb3, 0xfa, 0xfa, 0x56, 0x75, 0x65, 0x9f, 0xad, 0x94, 0x53, 0x5d, 0xb6, 0x0f, 0x85,
	0xc9, 0xff, 0x26, 0x34, 0x5b, 0x76, 0xeb, 0x16, 0x7c, 0xb8, 0x0f, 0x00, 0x00, 0xff, 0xff, 0x37,
	0x89, 0xf2, 0x99, 0x4e, 0x02, 0x00, 0x00,
}
