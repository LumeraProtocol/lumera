// Code generated by protoc-gen-go-pulsar. DO NOT EDIT.
package supernode

import (
	_ "github.com/cosmos/gogoproto/gogoproto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.0
// 	protoc        (unknown)
// source: pastel/supernode/supernode_state.proto

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type SuperNodeState int32

const (
	SuperNodeState_SUPERNODE_STATE_UNSPECIFIED SuperNodeState = 0
	SuperNodeState_SUPERNODE_STATE_ACTIVE      SuperNodeState = 1
	SuperNodeState_SUPERNODE_STATE_DISABLED    SuperNodeState = 2
	SuperNodeState_SUPERNODE_STATE_STOPPED     SuperNodeState = 3
	SuperNodeState_SUPERNODE_STATE_PENALIZED   SuperNodeState = 4
)

// Enum value maps for SuperNodeState.
var (
	SuperNodeState_name = map[int32]string{
		0: "SUPERNODE_STATE_UNSPECIFIED",
		1: "SUPERNODE_STATE_ACTIVE",
		2: "SUPERNODE_STATE_DISABLED",
		3: "SUPERNODE_STATE_STOPPED",
		4: "SUPERNODE_STATE_PENALIZED",
	}
	SuperNodeState_value = map[string]int32{
		"SUPERNODE_STATE_UNSPECIFIED": 0,
		"SUPERNODE_STATE_ACTIVE":      1,
		"SUPERNODE_STATE_DISABLED":    2,
		"SUPERNODE_STATE_STOPPED":     3,
		"SUPERNODE_STATE_PENALIZED":   4,
	}
)

func (x SuperNodeState) Enum() *SuperNodeState {
	p := new(SuperNodeState)
	*p = x
	return p
}

func (x SuperNodeState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (SuperNodeState) Descriptor() protoreflect.EnumDescriptor {
	return file_pastel_supernode_supernode_state_proto_enumTypes[0].Descriptor()
}

func (SuperNodeState) Type() protoreflect.EnumType {
	return &file_pastel_supernode_supernode_state_proto_enumTypes[0]
}

func (x SuperNodeState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use SuperNodeState.Descriptor instead.
func (SuperNodeState) EnumDescriptor() ([]byte, []int) {
	return file_pastel_supernode_supernode_state_proto_rawDescGZIP(), []int{0}
}

var File_pastel_supernode_supernode_state_proto protoreflect.FileDescriptor

var file_pastel_supernode_supernode_state_proto_rawDesc = []byte{
	0x0a, 0x26, 0x70, 0x61, 0x73, 0x74, 0x65, 0x6c, 0x2f, 0x73, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f,
	0x64, 0x65, 0x2f, 0x73, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x73, 0x74, 0x61,
	0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x10, 0x70, 0x61, 0x73, 0x74, 0x65, 0x6c,
	0x2e, 0x73, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f, 0x64, 0x65, 0x1a, 0x14, 0x67, 0x6f, 0x67, 0x6f,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x67, 0x6f, 0x67, 0x6f, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2a, 0xf8, 0x01, 0x0a, 0x0e, 0x53, 0x75, 0x70, 0x65, 0x72, 0x4e, 0x6f, 0x64, 0x65, 0x53, 0x74,
	0x61, 0x74, 0x65, 0x12, 0x30, 0x0a, 0x1b, 0x53, 0x55, 0x50, 0x45, 0x52, 0x4e, 0x4f, 0x44, 0x45,
	0x5f, 0x53, 0x54, 0x41, 0x54, 0x45, 0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49,
	0x45, 0x44, 0x10, 0x00, 0x1a, 0x0f, 0x8a, 0x9d, 0x20, 0x0b, 0x55, 0x6e, 0x73, 0x70, 0x65, 0x63,
	0x69, 0x66, 0x69, 0x65, 0x64, 0x12, 0x26, 0x0a, 0x16, 0x53, 0x55, 0x50, 0x45, 0x52, 0x4e, 0x4f,
	0x44, 0x45, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x45, 0x5f, 0x41, 0x43, 0x54, 0x49, 0x56, 0x45, 0x10,
	0x01, 0x1a, 0x0a, 0x8a, 0x9d, 0x20, 0x06, 0x41, 0x63, 0x74, 0x69, 0x76, 0x65, 0x12, 0x2a, 0x0a,
	0x18, 0x53, 0x55, 0x50, 0x45, 0x52, 0x4e, 0x4f, 0x44, 0x45, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x45,
	0x5f, 0x44, 0x49, 0x53, 0x41, 0x42, 0x4c, 0x45, 0x44, 0x10, 0x02, 0x1a, 0x0c, 0x8a, 0x9d, 0x20,
	0x08, 0x44, 0x69, 0x73, 0x61, 0x62, 0x6c, 0x65, 0x64, 0x12, 0x28, 0x0a, 0x17, 0x53, 0x55, 0x50,
	0x45, 0x52, 0x4e, 0x4f, 0x44, 0x45, 0x5f, 0x53, 0x54, 0x41, 0x54, 0x45, 0x5f, 0x53, 0x54, 0x4f,
	0x50, 0x50, 0x45, 0x44, 0x10, 0x03, 0x1a, 0x0b, 0x8a, 0x9d, 0x20, 0x07, 0x53, 0x74, 0x6f, 0x70,
	0x70, 0x65, 0x64, 0x12, 0x2c, 0x0a, 0x19, 0x53, 0x55, 0x50, 0x45, 0x52, 0x4e, 0x4f, 0x44, 0x45,
	0x5f, 0x53, 0x54, 0x41, 0x54, 0x45, 0x5f, 0x50, 0x45, 0x4e, 0x41, 0x4c, 0x49, 0x5a, 0x45, 0x44,
	0x10, 0x04, 0x1a, 0x0d, 0x8a, 0x9d, 0x20, 0x09, 0x50, 0x65, 0x6e, 0x61, 0x6c, 0x69, 0x7a, 0x65,
	0x64, 0x1a, 0x08, 0x88, 0xa3, 0x1e, 0x00, 0xa8, 0xa4, 0x1e, 0x01, 0x42, 0xc2, 0x01, 0x0a, 0x14,
	0x63, 0x6f, 0x6d, 0x2e, 0x70, 0x61, 0x73, 0x74, 0x65, 0x6c, 0x2e, 0x73, 0x75, 0x70, 0x65, 0x72,
	0x6e, 0x6f, 0x64, 0x65, 0x42, 0x13, 0x53, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f, 0x64, 0x65, 0x53,
	0x74, 0x61, 0x74, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x34, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x61, 0x73, 0x74, 0x65, 0x6c, 0x6e, 0x65,
	0x74, 0x77, 0x6f, 0x72, 0x6b, 0x2f, 0x70, 0x61, 0x73, 0x74, 0x65, 0x6c, 0x2f, 0x61, 0x70, 0x69,
	0x2f, 0x70, 0x61, 0x73, 0x74, 0x65, 0x6c, 0x2f, 0x73, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f, 0x64,
	0x65, 0xa2, 0x02, 0x03, 0x50, 0x53, 0x58, 0xaa, 0x02, 0x10, 0x50, 0x61, 0x73, 0x74, 0x65, 0x6c,
	0x2e, 0x53, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f, 0x64, 0x65, 0xca, 0x02, 0x10, 0x50, 0x61, 0x73,
	0x74, 0x65, 0x6c, 0x5c, 0x53, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f, 0x64, 0x65, 0xe2, 0x02, 0x1c,
	0x50, 0x61, 0x73, 0x74, 0x65, 0x6c, 0x5c, 0x53, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f, 0x64, 0x65,
	0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x11, 0x50,
	0x61, 0x73, 0x74, 0x65, 0x6c, 0x3a, 0x3a, 0x53, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f, 0x64, 0x65,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pastel_supernode_supernode_state_proto_rawDescOnce sync.Once
	file_pastel_supernode_supernode_state_proto_rawDescData = file_pastel_supernode_supernode_state_proto_rawDesc
)

func file_pastel_supernode_supernode_state_proto_rawDescGZIP() []byte {
	file_pastel_supernode_supernode_state_proto_rawDescOnce.Do(func() {
		file_pastel_supernode_supernode_state_proto_rawDescData = protoimpl.X.CompressGZIP(file_pastel_supernode_supernode_state_proto_rawDescData)
	})
	return file_pastel_supernode_supernode_state_proto_rawDescData
}

var file_pastel_supernode_supernode_state_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_pastel_supernode_supernode_state_proto_goTypes = []interface{}{
	(SuperNodeState)(0), // 0: pastel.supernode.SuperNodeState
}
var file_pastel_supernode_supernode_state_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_pastel_supernode_supernode_state_proto_init() }
func file_pastel_supernode_supernode_state_proto_init() {
	if File_pastel_supernode_supernode_state_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pastel_supernode_supernode_state_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   0,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pastel_supernode_supernode_state_proto_goTypes,
		DependencyIndexes: file_pastel_supernode_supernode_state_proto_depIdxs,
		EnumInfos:         file_pastel_supernode_supernode_state_proto_enumTypes,
	}.Build()
	File_pastel_supernode_supernode_state_proto = out.File
	file_pastel_supernode_supernode_state_proto_rawDesc = nil
	file_pastel_supernode_supernode_state_proto_goTypes = nil
	file_pastel_supernode_supernode_state_proto_depIdxs = nil
}
