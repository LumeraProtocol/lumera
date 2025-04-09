// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: lumera/supernode/supernode_state.proto

package types

import (
	fmt "fmt"
	_ "github.com/cosmos/gogoproto/gogoproto"
	proto "github.com/cosmos/gogoproto/proto"
	io "io"
	math "math"
	math_bits "math/bits"
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

type SuperNodeState int32

const (
	SuperNodeStateUnspecified SuperNodeState = 0
	SuperNodeStateActive      SuperNodeState = 1
	SuperNodeStateDisabled    SuperNodeState = 2
	SuperNodeStateStopped     SuperNodeState = 3
	SuperNodeStatePenalized   SuperNodeState = 4
)

var SuperNodeState_name = map[int32]string{
	0: "SUPERNODE_STATE_UNSPECIFIED",
	1: "SUPERNODE_STATE_ACTIVE",
	2: "SUPERNODE_STATE_DISABLED",
	3: "SUPERNODE_STATE_STOPPED",
	4: "SUPERNODE_STATE_PENALIZED",
}

var SuperNodeState_value = map[string]int32{
	"SUPERNODE_STATE_UNSPECIFIED": 0,
	"SUPERNODE_STATE_ACTIVE":      1,
	"SUPERNODE_STATE_DISABLED":    2,
	"SUPERNODE_STATE_STOPPED":     3,
	"SUPERNODE_STATE_PENALIZED":   4,
}

func (x SuperNodeState) String() string {
	return proto.EnumName(SuperNodeState_name, int32(x))
}

func (SuperNodeState) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_1e8544c7b0c375ed, []int{0}
}

type SuperNodeStateRecord struct {
	State  SuperNodeState `protobuf:"varint,1,opt,name=state,proto3,enum=lumera.supernode.SuperNodeState" json:"state,omitempty" yaml:"state"`
	Height int64          `protobuf:"varint,2,opt,name=height,proto3" json:"height,omitempty"`
}

func (m *SuperNodeStateRecord) Reset()         { *m = SuperNodeStateRecord{} }
func (m *SuperNodeStateRecord) String() string { return proto.CompactTextString(m) }
func (*SuperNodeStateRecord) ProtoMessage()    {}
func (*SuperNodeStateRecord) Descriptor() ([]byte, []int) {
	return fileDescriptor_1e8544c7b0c375ed, []int{0}
}
func (m *SuperNodeStateRecord) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SuperNodeStateRecord) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SuperNodeStateRecord.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SuperNodeStateRecord) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SuperNodeStateRecord.Merge(m, src)
}
func (m *SuperNodeStateRecord) XXX_Size() int {
	return m.Size()
}
func (m *SuperNodeStateRecord) XXX_DiscardUnknown() {
	xxx_messageInfo_SuperNodeStateRecord.DiscardUnknown(m)
}

var xxx_messageInfo_SuperNodeStateRecord proto.InternalMessageInfo

func (m *SuperNodeStateRecord) GetState() SuperNodeState {
	if m != nil {
		return m.State
	}
	return SuperNodeStateUnspecified
}

func (m *SuperNodeStateRecord) GetHeight() int64 {
	if m != nil {
		return m.Height
	}
	return 0
}

func init() {
	proto.RegisterEnum("lumera.supernode.SuperNodeState", SuperNodeState_name, SuperNodeState_value)
	proto.RegisterType((*SuperNodeStateRecord)(nil), "lumera.supernode.SuperNodeStateRecord")
}

func init() {
	proto.RegisterFile("lumera/supernode/supernode_state.proto", fileDescriptor_1e8544c7b0c375ed)
}

var fileDescriptor_1e8544c7b0c375ed = []byte{
	// 416 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x52, 0xcb, 0x29, 0xcd, 0x4d,
	0x2d, 0x4a, 0xd4, 0x2f, 0x2e, 0x2d, 0x48, 0x2d, 0xca, 0xcb, 0x4f, 0x49, 0x45, 0xb0, 0xe2, 0x8b,
	0x4b, 0x12, 0x4b, 0x52, 0xf5, 0x0a, 0x8a, 0xf2, 0x4b, 0xf2, 0x85, 0x04, 0x20, 0xea, 0xf4, 0xe0,
	0xb2, 0x52, 0x22, 0xe9, 0xf9, 0xe9, 0xf9, 0x60, 0x49, 0x7d, 0x10, 0x0b, 0xa2, 0x4e, 0xa9, 0x82,
	0x4b, 0x24, 0x18, 0xa4, 0xc4, 0x2f, 0x3f, 0x25, 0x35, 0x18, 0xa4, 0x3f, 0x28, 0x35, 0x39, 0xbf,
	0x28, 0x45, 0xc8, 0x83, 0x8b, 0x15, 0x6c, 0x9c, 0x04, 0xa3, 0x02, 0xa3, 0x06, 0x9f, 0x91, 0x82,
	0x1e, 0xba, 0x79, 0x7a, 0xa8, 0xda, 0x9c, 0x04, 0x3e, 0xdd, 0x93, 0xe7, 0xa9, 0x4c, 0xcc, 0xcd,
	0xb1, 0x52, 0x02, 0x6b, 0x54, 0x0a, 0x82, 0x18, 0x20, 0x24, 0xc6, 0xc5, 0x96, 0x91, 0x9a, 0x99,
	0x9e, 0x51, 0x22, 0xc1, 0xa4, 0xc0, 0xa8, 0xc1, 0x1c, 0x04, 0xe5, 0x69, 0xed, 0x63, 0xe2, 0xe2,
	0x43, 0x35, 0x43, 0xc8, 0x8e, 0x4b, 0x3a, 0x38, 0x34, 0xc0, 0x35, 0xc8, 0xcf, 0xdf, 0xc5, 0x35,
	0x3e, 0x38, 0xc4, 0x31, 0xc4, 0x35, 0x3e, 0xd4, 0x2f, 0x38, 0xc0, 0xd5, 0xd9, 0xd3, 0xcd, 0xd3,
	0xd5, 0x45, 0x80, 0x41, 0x4a, 0xb6, 0x6b, 0xae, 0x82, 0x24, 0xaa, 0xa6, 0xd0, 0xbc, 0xe2, 0x82,
	0xd4, 0xe4, 0xcc, 0xb4, 0xcc, 0xd4, 0x14, 0x21, 0x13, 0x2e, 0x31, 0x74, 0xfd, 0x8e, 0xce, 0x21,
	0x9e, 0x61, 0xae, 0x02, 0x8c, 0x52, 0x12, 0x5d, 0x73, 0x15, 0xd0, 0xbc, 0xea, 0x98, 0x5c, 0x92,
	0x59, 0x96, 0x2a, 0x64, 0xc1, 0x25, 0x81, 0xae, 0xcb, 0xc5, 0x33, 0xd8, 0xd1, 0xc9, 0xc7, 0xd5,
	0x45, 0x80, 0x49, 0x4a, 0xaa, 0x6b, 0xae, 0x82, 0x18, 0xaa, 0x3e, 0x97, 0xcc, 0xe2, 0xc4, 0xa4,
	0x9c, 0xd4, 0x14, 0x21, 0x33, 0x2e, 0x71, 0x74, 0x9d, 0xc1, 0x21, 0xfe, 0x01, 0x01, 0xae, 0x2e,
	0x02, 0xcc, 0x52, 0x92, 0x5d, 0x73, 0x15, 0x44, 0x51, 0x35, 0x06, 0x97, 0xe4, 0x17, 0x14, 0xa4,
	0xa6, 0x08, 0x59, 0x71, 0x49, 0xa2, 0xeb, 0x0b, 0x70, 0xf5, 0x73, 0xf4, 0xf1, 0x8c, 0x72, 0x75,
	0x11, 0x60, 0x91, 0x92, 0xee, 0x9a, 0xab, 0x20, 0x8e, 0xaa, 0x33, 0x20, 0x35, 0x2f, 0x31, 0x27,
	0xb3, 0x2a, 0x35, 0x45, 0x8a, 0xa3, 0x63, 0xb1, 0x1c, 0xc3, 0x8a, 0x25, 0x72, 0x8c, 0x4e, 0x3e,
	0x27, 0x1e, 0xc9, 0x31, 0x5e, 0x78, 0x24, 0xc7, 0xf8, 0xe0, 0x91, 0x1c, 0xe3, 0x84, 0xc7, 0x72,
	0x0c, 0x17, 0x1e, 0xcb, 0x31, 0xdc, 0x78, 0x2c, 0xc7, 0x10, 0x65, 0x94, 0x9e, 0x59, 0x92, 0x51,
	0x9a, 0xa4, 0x97, 0x9c, 0x9f, 0xab, 0xef, 0x03, 0x8e, 0xb7, 0x00, 0x50, 0x64, 0x27, 0xe7, 0xe7,
	0xe8, 0x43, 0x93, 0x4f, 0x05, 0x52, 0x02, 0x2a, 0xa9, 0x2c, 0x48, 0x2d, 0x4e, 0x62, 0x03, 0xa7,
	0x07, 0x63, 0x40, 0x00, 0x00, 0x00, 0xff, 0xff, 0x34, 0xdd, 0x16, 0x14, 0x61, 0x02, 0x00, 0x00,
}

func (m *SuperNodeStateRecord) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SuperNodeStateRecord) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SuperNodeStateRecord) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Height != 0 {
		i = encodeVarintSupernodeState(dAtA, i, uint64(m.Height))
		i--
		dAtA[i] = 0x10
	}
	if m.State != 0 {
		i = encodeVarintSupernodeState(dAtA, i, uint64(m.State))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintSupernodeState(dAtA []byte, offset int, v uint64) int {
	offset -= sovSupernodeState(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *SuperNodeStateRecord) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.State != 0 {
		n += 1 + sovSupernodeState(uint64(m.State))
	}
	if m.Height != 0 {
		n += 1 + sovSupernodeState(uint64(m.Height))
	}
	return n
}

func sovSupernodeState(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozSupernodeState(x uint64) (n int) {
	return sovSupernodeState(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *SuperNodeStateRecord) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSupernodeState
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: SuperNodeStateRecord: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SuperNodeStateRecord: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field State", wireType)
			}
			m.State = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSupernodeState
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.State |= SuperNodeState(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Height", wireType)
			}
			m.Height = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSupernodeState
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Height |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipSupernodeState(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthSupernodeState
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipSupernodeState(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowSupernodeState
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowSupernodeState
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowSupernodeState
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthSupernodeState
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupSupernodeState
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthSupernodeState
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthSupernodeState        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowSupernodeState          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupSupernodeState = fmt.Errorf("proto: unexpected end of group")
)
