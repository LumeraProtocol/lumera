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
	ActionStateDone        ActionState = 2
	ActionStateApproved    ActionState = 3
	ActionStateRejected    ActionState = 4
	ActionStateFailed      ActionState = 5
)

var ActionState_name = map[int32]string{
	0: "ACTION_STATE_UNSPECIFIED",
	1: "ACTION_STATE_PENDING",
	2: "ACTION_STATE_DONE",
	3: "ACTION_STATE_APPROVED",
	4: "ACTION_STATE_REJECTED",
	5: "ACTION_STATE_FAILED",
}

var ActionState_value = map[string]int32{
	"ACTION_STATE_UNSPECIFIED": 0,
	"ACTION_STATE_PENDING":     1,
	"ACTION_STATE_DONE":        2,
	"ACTION_STATE_APPROVED":    3,
	"ACTION_STATE_REJECTED":    4,
	"ACTION_STATE_FAILED":      5,
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
	// 344 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x91, 0x3d, 0x4f, 0xc2, 0x40,
	0x1c, 0x87, 0x5b, 0x7c, 0x89, 0x39, 0x63, 0x2c, 0xc7, 0x8b, 0xe6, 0x86, 0x4b, 0x67, 0x86, 0xd6,
	0xe8, 0xe2, 0x5a, 0xb9, 0xc3, 0xd4, 0x90, 0xd2, 0x40, 0x71, 0x70, 0x21, 0xa5, 0x3d, 0x6b, 0x0d,
	0xf4, 0x1a, 0x38, 0x8c, 0x7e, 0x03, 0xd3, 0xc9, 0x2f, 0xd0, 0x49, 0x07, 0xbf, 0x89, 0x8e, 0x8c,
	0x8e, 0x06, 0xbe, 0x88, 0xa1, 0x60, 0xd2, 0x86, 0xe9, 0x2e, 0xf9, 0x3d, 0xcf, 0xf2, 0x7f, 0x80,
	0x3a, 0x9a, 0x8d, 0xd9, 0xc4, 0xd5, 0x5d, 0x4f, 0x84, 0x3c, 0xda, 0x3c, 0x83, 0xa9, 0x70, 0x05,
	0xd3, 0xe2, 0x09, 0x17, 0x1c, 0x1e, 0xad, 0x09, 0x6d, 0x3d, 0xa1, 0x6a, 0xc0, 0x03, 0x9e, 0x2d,
	0xfa, 0xea, 0xb7, 0x86, 0x1a, 0x5f, 0x25, 0x70, 0x68, 0x64, 0x40, 0x6f, 0xa5, 0xc2, 0x4b, 0x70,
	0x6a, 0x34, 0x1d, 0xb3, 0x63, 0x0d, 0x7a, 0x8e, 0xe1, 0xd0, 0x41, 0xdf, 0xea, 0xd9, 0xb4, 0x69,
	0xb6, 0x4c, 0x4a, 0x14, 0x09, 0xa1, 0x24, 0x55, 0xeb, 0x39, 0xbc, 0x1f, 0x4d, 0x63, 0xe6, 0x85,
	0xf7, 0x21, 0xf3, 0xe1, 0x19, 0xa8, 0x16, 0x4c, 0x9b, 0x5a, 0xc4, 0xb4, 0xae, 0x15, 0x19, 0xd5,
	0x93, 0x54, 0x85, 0x39, 0xcb, 0x66, 0x91, 0x1f, 0x46, 0x01, 0x6c, 0x80, 0x72, 0xc1, 0x20, 0x1d,
	0x8b, 0x2a, 0x25, 0x54, 0x49, 0x52, 0xf5, 0x38, 0x87, 0x13, 0x1e, 0x31, 0x78, 0x0e, 0x6a, 0x05,
	0xd6, 0xb0, 0xed, 0x6e, 0xe7, 0x96, 0x12, 0x65, 0x07, 0x9d, 0x24, 0xa9, 0x5a, 0xc9, 0xf1, 0x46,
	0x1c, 0x4f, 0xf8, 0x13, 0xf3, 0xb7, 0x9c, 0x2e, 0xbd, 0xa1, 0x4d, 0x87, 0x12, 0x65, 0x77, 0xcb,
	0xe9, 0xb2, 0x47, 0xe6, 0x09, 0xe6, 0x43, 0x0d, 0x54, 0x0a, 0x4e, 0xcb, 0x30, 0xdb, 0x94, 0x28,
	0x7b, 0xa8, 0x96, 0xa4, 0x6a, 0x39, 0x67, 0xb4, 0xdc, 0x70, 0xc4, 0x7c, 0x74, 0xf0, 0xfa, 0x8e,
	0xa5, 0xcf, 0x0f, 0x2c, 0x5f, 0x99, 0xdf, 0x0b, 0x2c, 0xcf, 0x17, 0x58, 0xfe, 0x5d, 0x60, 0xf9,
	0x6d, 0x89, 0xa5, 0xf9, 0x12, 0x4b, 0x3f, 0x4b, 0x2c, 0xdd, 0xe9, 0x41, 0x28, 0x1e, 0x66, 0x43,
	0xcd, 0xe3, 0x63, 0xbd, 0x9d, 0x35, 0xb1, 0x57, 0xb7, 0xf7, 0xf8, 0x48, 0xdf, 0x44, 0x7c, 0xfe,
	0xcf, 0x28, 0x5e, 0x62, 0x36, 0x1d, 0xee, 0x67, 0x6d, 0x2e, 0xfe, 0x02, 0x00, 0x00, 0xff, 0xff,
	0x13, 0x27, 0xc7, 0xab, 0xe4, 0x01, 0x00, 0x00,
}
