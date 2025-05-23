// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: lumera/action/action.proto

package types

import (
	fmt "fmt"
	_ "github.com/cosmos/cosmos-proto"
	github_com_cosmos_cosmos_sdk_types "github.com/cosmos/cosmos-sdk/types"
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

type Action struct {
	Creator        string                                   `protobuf:"bytes,1,opt,name=creator,proto3" json:"creator,omitempty"`
	ActionID       string                                   `protobuf:"bytes,2,opt,name=actionID,proto3" json:"actionID,omitempty"`
	ActionType     ActionType                               `protobuf:"varint,3,opt,name=actionType,proto3,enum=lumera.action.ActionType" json:"actionType,omitempty"`
	Metadata       []byte                                   `protobuf:"bytes,4,opt,name=metadata,proto3" json:"metadata,omitempty"`
	Price          *github_com_cosmos_cosmos_sdk_types.Coin `protobuf:"bytes,5,opt,name=price,proto3,customtype=github.com/cosmos/cosmos-sdk/types.Coin" json:"price,omitempty"`
	ExpirationTime int64                                    `protobuf:"varint,6,opt,name=expirationTime,proto3" json:"expirationTime,omitempty"`
	State          ActionState                              `protobuf:"varint,7,opt,name=state,proto3,enum=lumera.action.ActionState" json:"state,omitempty"`
	BlockHeight    int64                                    `protobuf:"varint,8,opt,name=blockHeight,proto3" json:"blockHeight,omitempty"`
	SuperNodes     []string                                 `protobuf:"bytes,9,rep,name=superNodes,proto3" json:"superNodes,omitempty"`
}

func (m *Action) Reset()         { *m = Action{} }
func (m *Action) String() string { return proto.CompactTextString(m) }
func (*Action) ProtoMessage()    {}
func (*Action) Descriptor() ([]byte, []int) {
	return fileDescriptor_fb97c90726166d4c, []int{0}
}
func (m *Action) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Action) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Action.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Action) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Action.Merge(m, src)
}
func (m *Action) XXX_Size() int {
	return m.Size()
}
func (m *Action) XXX_DiscardUnknown() {
	xxx_messageInfo_Action.DiscardUnknown(m)
}

var xxx_messageInfo_Action proto.InternalMessageInfo

func (m *Action) GetCreator() string {
	if m != nil {
		return m.Creator
	}
	return ""
}

func (m *Action) GetActionID() string {
	if m != nil {
		return m.ActionID
	}
	return ""
}

func (m *Action) GetActionType() ActionType {
	if m != nil {
		return m.ActionType
	}
	return ActionTypeUnspecified
}

func (m *Action) GetMetadata() []byte {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func (m *Action) GetExpirationTime() int64 {
	if m != nil {
		return m.ExpirationTime
	}
	return 0
}

func (m *Action) GetState() ActionState {
	if m != nil {
		return m.State
	}
	return ActionStateUnspecified
}

func (m *Action) GetBlockHeight() int64 {
	if m != nil {
		return m.BlockHeight
	}
	return 0
}

func (m *Action) GetSuperNodes() []string {
	if m != nil {
		return m.SuperNodes
	}
	return nil
}

func init() {
	proto.RegisterType((*Action)(nil), "lumera.action.Action")
}

func init() { proto.RegisterFile("lumera/action/action.proto", fileDescriptor_fb97c90726166d4c) }

var fileDescriptor_fb97c90726166d4c = []byte{
	// 421 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x52, 0xc1, 0x8e, 0xd3, 0x30,
	0x10, 0xad, 0x09, 0xed, 0x6e, 0x0d, 0xec, 0xc1, 0xda, 0x83, 0x37, 0x82, 0x6c, 0xe0, 0x00, 0x91,
	0x50, 0x13, 0x28, 0x27, 0x8e, 0x29, 0x1c, 0x40, 0xaa, 0x10, 0x4a, 0x11, 0x07, 0x2e, 0x95, 0x9b,
	0x58, 0xa9, 0xd5, 0xa4, 0x8e, 0x6c, 0x17, 0xb5, 0x7f, 0xc1, 0xaf, 0x20, 0xf5, 0x23, 0x38, 0x56,
	0x3d, 0x21, 0x0e, 0x08, 0xb5, 0x3f, 0x82, 0x62, 0xbb, 0x55, 0xba, 0xea, 0xc9, 0x9e, 0x79, 0x6f,
	0xc6, 0x6f, 0x9e, 0x07, 0xba, 0xc5, 0xa2, 0xa4, 0x82, 0x44, 0x24, 0x55, 0x8c, 0xcf, 0xed, 0x11,
	0x56, 0x82, 0x2b, 0x8e, 0x1e, 0x19, 0x2c, 0x34, 0x49, 0xf7, 0x3a, 0xe7, 0x39, 0xd7, 0x48, 0x54,
	0xdf, 0x0c, 0xc9, 0xbd, 0x49, 0xb9, 0x2c, 0xb9, 0x1c, 0x1b, 0xc0, 0x04, 0x16, 0x7a, 0x7c, 0xda,
	0xbb, 0xa4, 0x8a, 0x64, 0x44, 0x11, 0x8b, 0xfa, 0xe7, 0x5e, 0x1e, 0x4b, 0x45, 0x14, 0xb5, 0x8c,
	0xdb, 0xb3, 0x0c, 0xb5, 0xaa, 0x2c, 0xe1, 0xd9, 0x4f, 0x07, 0x76, 0x62, 0x9d, 0x45, 0x7d, 0x78,
	0x91, 0x0a, 0x4a, 0x14, 0x17, 0x18, 0xf8, 0x20, 0xe8, 0x0e, 0xf0, 0x76, 0xdd, 0xbb, 0xb6, 0x72,
	0xe2, 0x2c, 0x13, 0x54, 0xca, 0x91, 0x12, 0x6c, 0x9e, 0x27, 0x07, 0x22, 0x72, 0xe1, 0xa5, 0xe9,
	0xf9, 0xf1, 0x3d, 0xbe, 0x57, 0x17, 0x25, 0xc7, 0x18, 0xbd, 0x85, 0xd0, 0xdc, 0xbf, 0xac, 0x2a,
	0x8a, 0x1d, 0x1f, 0x04, 0x57, 0xfd, 0x9b, 0xf0, 0xc4, 0x90, 0x30, 0x3e, 0x12, 0x92, 0x06, 0xb9,
	0x6e, 0x7b, 0x18, 0x15, 0xdf, 0xf7, 0x41, 0xf0, 0x30, 0x39, 0xc6, 0x28, 0x86, 0xed, 0x4a, 0xb0,
	0x94, 0xe2, 0xb6, 0x16, 0xf9, 0xf2, 0xcf, 0xdf, 0xdb, 0x17, 0x39, 0x53, 0xd3, 0xc5, 0x24, 0x4c,
	0x79, 0x69, 0xed, 0xb3, 0x47, 0x4f, 0x66, 0xb3, 0xa8, 0x9e, 0x56, 0x86, 0xef, 0x38, 0x9b, 0x27,
	0xa6, 0x12, 0x3d, 0x87, 0x57, 0x74, 0x59, 0x31, 0x41, 0xf4, 0x83, 0xac, 0xa4, 0xb8, 0xe3, 0x83,
	0xc0, 0x49, 0xee, 0x64, 0xd1, 0x2b, 0xd8, 0xd6, 0x66, 0xe2, 0x0b, 0x2d, 0xde, 0x3d, 0x2b, 0x7e,
	0x54, 0x33, 0x12, 0x43, 0x44, 0x3e, 0x7c, 0x30, 0x29, 0x78, 0x3a, 0xfb, 0x40, 0x59, 0x3e, 0x55,
	0xf8, 0x52, 0xb7, 0x6d, 0xa6, 0x50, 0x0c, 0xa1, 0x5c, 0x54, 0x54, 0x7c, 0xe2, 0x19, 0x95, 0xb8,
	0xeb, 0x3b, 0x41, 0x77, 0xf0, 0x74, 0xbb, 0xee, 0x3d, 0xb1, 0x46, 0x7f, 0x25, 0x05, 0xcb, 0x6a,
	0x6f, 0x4f, 0x1d, 0x6f, 0x14, 0x0d, 0x86, 0xbf, 0x76, 0x1e, 0xd8, 0xec, 0x3c, 0xf0, 0x6f, 0xe7,
	0x81, 0x1f, 0x7b, 0xaf, 0xb5, 0xd9, 0x7b, 0xad, 0xdf, 0x7b, 0xaf, 0xf5, 0xad, 0xdf, 0x30, 0x62,
	0xa8, 0xb5, 0x7e, 0xae, 0x7f, 0x39, 0xe5, 0x45, 0x64, 0x17, 0x61, 0x79, 0x58, 0x85, 0xef, 0xaf,
	0x8d, 0x31, 0x93, 0x8e, 0x5e, 0x84, 0x37, 0xff, 0x03, 0x00, 0x00, 0xff, 0xff, 0xbe, 0xb6, 0xea,
	0xb0, 0xc7, 0x02, 0x00, 0x00,
}

func (m *Action) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Action) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Action) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.SuperNodes) > 0 {
		for iNdEx := len(m.SuperNodes) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.SuperNodes[iNdEx])
			copy(dAtA[i:], m.SuperNodes[iNdEx])
			i = encodeVarintAction(dAtA, i, uint64(len(m.SuperNodes[iNdEx])))
			i--
			dAtA[i] = 0x4a
		}
	}
	if m.BlockHeight != 0 {
		i = encodeVarintAction(dAtA, i, uint64(m.BlockHeight))
		i--
		dAtA[i] = 0x40
	}
	if m.State != 0 {
		i = encodeVarintAction(dAtA, i, uint64(m.State))
		i--
		dAtA[i] = 0x38
	}
	if m.ExpirationTime != 0 {
		i = encodeVarintAction(dAtA, i, uint64(m.ExpirationTime))
		i--
		dAtA[i] = 0x30
	}
	if m.Price != nil {
		{
			size := m.Price.Size()
			i -= size
			if _, err := m.Price.MarshalTo(dAtA[i:]); err != nil {
				return 0, err
			}
			i = encodeVarintAction(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x2a
	}
	if len(m.Metadata) > 0 {
		i -= len(m.Metadata)
		copy(dAtA[i:], m.Metadata)
		i = encodeVarintAction(dAtA, i, uint64(len(m.Metadata)))
		i--
		dAtA[i] = 0x22
	}
	if m.ActionType != 0 {
		i = encodeVarintAction(dAtA, i, uint64(m.ActionType))
		i--
		dAtA[i] = 0x18
	}
	if len(m.ActionID) > 0 {
		i -= len(m.ActionID)
		copy(dAtA[i:], m.ActionID)
		i = encodeVarintAction(dAtA, i, uint64(len(m.ActionID)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.Creator) > 0 {
		i -= len(m.Creator)
		copy(dAtA[i:], m.Creator)
		i = encodeVarintAction(dAtA, i, uint64(len(m.Creator)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintAction(dAtA []byte, offset int, v uint64) int {
	offset -= sovAction(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *Action) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Creator)
	if l > 0 {
		n += 1 + l + sovAction(uint64(l))
	}
	l = len(m.ActionID)
	if l > 0 {
		n += 1 + l + sovAction(uint64(l))
	}
	if m.ActionType != 0 {
		n += 1 + sovAction(uint64(m.ActionType))
	}
	l = len(m.Metadata)
	if l > 0 {
		n += 1 + l + sovAction(uint64(l))
	}
	if m.Price != nil {
		l = m.Price.Size()
		n += 1 + l + sovAction(uint64(l))
	}
	if m.ExpirationTime != 0 {
		n += 1 + sovAction(uint64(m.ExpirationTime))
	}
	if m.State != 0 {
		n += 1 + sovAction(uint64(m.State))
	}
	if m.BlockHeight != 0 {
		n += 1 + sovAction(uint64(m.BlockHeight))
	}
	if len(m.SuperNodes) > 0 {
		for _, s := range m.SuperNodes {
			l = len(s)
			n += 1 + l + sovAction(uint64(l))
		}
	}
	return n
}

func sovAction(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozAction(x uint64) (n int) {
	return sovAction(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *Action) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowAction
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
			return fmt.Errorf("proto: Action: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Action: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Creator", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowAction
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthAction
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthAction
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Creator = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ActionID", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowAction
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthAction
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthAction
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ActionID = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ActionType", wireType)
			}
			m.ActionType = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowAction
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ActionType |= ActionType(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Metadata", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowAction
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthAction
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthAction
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Metadata = append(m.Metadata[:0], dAtA[iNdEx:postIndex]...)
			if m.Metadata == nil {
				m.Metadata = []byte{}
			}
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Price", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowAction
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthAction
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthAction
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			var v github_com_cosmos_cosmos_sdk_types.Coin
			m.Price = &v
			if err := m.Price.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ExpirationTime", wireType)
			}
			m.ExpirationTime = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowAction
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ExpirationTime |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 7:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field State", wireType)
			}
			m.State = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowAction
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.State |= ActionState(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 8:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field BlockHeight", wireType)
			}
			m.BlockHeight = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowAction
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.BlockHeight |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 9:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SuperNodes", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowAction
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthAction
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthAction
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.SuperNodes = append(m.SuperNodes, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipAction(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthAction
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
func skipAction(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowAction
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
					return 0, ErrIntOverflowAction
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
					return 0, ErrIntOverflowAction
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
				return 0, ErrInvalidLengthAction
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupAction
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthAction
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthAction        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowAction          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupAction = fmt.Errorf("proto: unexpected end of group")
)
