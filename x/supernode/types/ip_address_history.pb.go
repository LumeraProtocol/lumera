// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: pastel/supernode/ip_address_history.proto

package types

import (
	fmt "fmt"
	_ "github.com/cosmos/gogoproto/gogoproto"
	proto "github.com/cosmos/gogoproto/proto"
	github_com_cosmos_gogoproto_types "github.com/cosmos/gogoproto/types"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	io "io"
	math "math"
	math_bits "math/bits"
	time "time"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf
var _ = time.Kitchen

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type IPAddressHistory struct {
	Address   string    `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	UpdatedAt time.Time `protobuf:"bytes,2,opt,name=updated_at,json=updatedAt,proto3,stdtime" json:"updated_at"`
	EndedAt   time.Time `protobuf:"bytes,3,opt,name=ended_at,json=endedAt,proto3,stdtime" json:"ended_at"`
}

func (m *IPAddressHistory) Reset()         { *m = IPAddressHistory{} }
func (m *IPAddressHistory) String() string { return proto.CompactTextString(m) }
func (*IPAddressHistory) ProtoMessage()    {}
func (*IPAddressHistory) Descriptor() ([]byte, []int) {
	return fileDescriptor_9597d96f5b80e0e9, []int{0}
}
func (m *IPAddressHistory) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *IPAddressHistory) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_IPAddressHistory.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *IPAddressHistory) XXX_Merge(src proto.Message) {
	xxx_messageInfo_IPAddressHistory.Merge(m, src)
}
func (m *IPAddressHistory) XXX_Size() int {
	return m.Size()
}
func (m *IPAddressHistory) XXX_DiscardUnknown() {
	xxx_messageInfo_IPAddressHistory.DiscardUnknown(m)
}

var xxx_messageInfo_IPAddressHistory proto.InternalMessageInfo

func (m *IPAddressHistory) GetAddress() string {
	if m != nil {
		return m.Address
	}
	return ""
}

func (m *IPAddressHistory) GetUpdatedAt() time.Time {
	if m != nil {
		return m.UpdatedAt
	}
	return time.Time{}
}

func (m *IPAddressHistory) GetEndedAt() time.Time {
	if m != nil {
		return m.EndedAt
	}
	return time.Time{}
}

func init() {
	proto.RegisterType((*IPAddressHistory)(nil), "pastel.supernode.IPAddressHistory")
}

func init() {
	proto.RegisterFile("pastel/supernode/ip_address_history.proto", fileDescriptor_9597d96f5b80e0e9)
}

var fileDescriptor_9597d96f5b80e0e9 = []byte{
	// 273 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xd2, 0x2c, 0x48, 0x2c, 0x2e,
	0x49, 0xcd, 0xd1, 0x2f, 0x2e, 0x2d, 0x48, 0x2d, 0xca, 0xcb, 0x4f, 0x49, 0xd5, 0xcf, 0x2c, 0x88,
	0x4f, 0x4c, 0x49, 0x29, 0x4a, 0x2d, 0x2e, 0x8e, 0xcf, 0xc8, 0x2c, 0x2e, 0xc9, 0x2f, 0xaa, 0xd4,
	0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x12, 0x80, 0x28, 0xd5, 0x83, 0x2b, 0x95, 0x92, 0x4f, 0xcf,
	0xcf, 0x4f, 0xcf, 0x49, 0xd5, 0x07, 0xcb, 0x27, 0x95, 0xa6, 0xe9, 0x97, 0x64, 0xe6, 0xa6, 0x16,
	0x97, 0x24, 0xe6, 0x16, 0x40, 0xb4, 0x48, 0x89, 0xa4, 0xe7, 0xa7, 0xe7, 0x83, 0x99, 0xfa, 0x20,
	0x16, 0x44, 0x54, 0x69, 0x13, 0x23, 0x97, 0x80, 0x67, 0x80, 0x23, 0xc4, 0x12, 0x0f, 0x88, 0x1d,
	0x42, 0x12, 0x5c, 0xec, 0x50, 0x6b, 0x25, 0x18, 0x15, 0x18, 0x35, 0x38, 0x83, 0x60, 0x5c, 0x21,
	0x67, 0x2e, 0xae, 0xd2, 0x82, 0x94, 0xc4, 0x92, 0xd4, 0x94, 0xf8, 0xc4, 0x12, 0x09, 0x26, 0x05,
	0x46, 0x0d, 0x6e, 0x23, 0x29, 0x3d, 0x88, 0xd5, 0x7a, 0x30, 0xab, 0xf5, 0x42, 0x60, 0x56, 0x3b,
	0x71, 0x9c, 0xb8, 0x27, 0xcf, 0x30, 0xe1, 0xbe, 0x3c, 0x63, 0x10, 0x27, 0x54, 0x9f, 0x63, 0x89,
	0x90, 0x3d, 0x17, 0x47, 0x6a, 0x5e, 0x0a, 0xc4, 0x08, 0x66, 0x12, 0x8c, 0x60, 0x07, 0xeb, 0x72,
	0x2c, 0x71, 0xf2, 0x3e, 0xf1, 0x48, 0x8e, 0xf1, 0xc2, 0x23, 0x39, 0xc6, 0x07, 0x8f, 0xe4, 0x18,
	0x27, 0x3c, 0x96, 0x63, 0xb8, 0xf0, 0x58, 0x8e, 0xe1, 0xc6, 0x63, 0x39, 0x86, 0x28, 0xc3, 0xf4,
	0xcc, 0x92, 0x8c, 0xd2, 0x24, 0xbd, 0xe4, 0xfc, 0x5c, 0x7d, 0x48, 0x10, 0xe5, 0xa5, 0x96, 0x94,
	0xe7, 0x17, 0x65, 0x43, 0x79, 0xfa, 0x15, 0x48, 0xa1, 0x5b, 0x52, 0x59, 0x90, 0x5a, 0x9c, 0xc4,
	0x06, 0xb6, 0xd3, 0x18, 0x10, 0x00, 0x00, 0xff, 0xff, 0x6e, 0x0a, 0x34, 0xa4, 0x7e, 0x01, 0x00,
	0x00,
}

func (m *IPAddressHistory) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *IPAddressHistory) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *IPAddressHistory) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	n1, err1 := github_com_cosmos_gogoproto_types.StdTimeMarshalTo(m.EndedAt, dAtA[i-github_com_cosmos_gogoproto_types.SizeOfStdTime(m.EndedAt):])
	if err1 != nil {
		return 0, err1
	}
	i -= n1
	i = encodeVarintIpAddressHistory(dAtA, i, uint64(n1))
	i--
	dAtA[i] = 0x1a
	n2, err2 := github_com_cosmos_gogoproto_types.StdTimeMarshalTo(m.UpdatedAt, dAtA[i-github_com_cosmos_gogoproto_types.SizeOfStdTime(m.UpdatedAt):])
	if err2 != nil {
		return 0, err2
	}
	i -= n2
	i = encodeVarintIpAddressHistory(dAtA, i, uint64(n2))
	i--
	dAtA[i] = 0x12
	if len(m.Address) > 0 {
		i -= len(m.Address)
		copy(dAtA[i:], m.Address)
		i = encodeVarintIpAddressHistory(dAtA, i, uint64(len(m.Address)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintIpAddressHistory(dAtA []byte, offset int, v uint64) int {
	offset -= sovIpAddressHistory(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *IPAddressHistory) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Address)
	if l > 0 {
		n += 1 + l + sovIpAddressHistory(uint64(l))
	}
	l = github_com_cosmos_gogoproto_types.SizeOfStdTime(m.UpdatedAt)
	n += 1 + l + sovIpAddressHistory(uint64(l))
	l = github_com_cosmos_gogoproto_types.SizeOfStdTime(m.EndedAt)
	n += 1 + l + sovIpAddressHistory(uint64(l))
	return n
}

func sovIpAddressHistory(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozIpAddressHistory(x uint64) (n int) {
	return sovIpAddressHistory(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *IPAddressHistory) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowIpAddressHistory
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
			return fmt.Errorf("proto: IPAddressHistory: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: IPAddressHistory: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Address", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowIpAddressHistory
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
				return ErrInvalidLengthIpAddressHistory
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthIpAddressHistory
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Address = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field UpdatedAt", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowIpAddressHistory
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthIpAddressHistory
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthIpAddressHistory
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := github_com_cosmos_gogoproto_types.StdTimeUnmarshal(&m.UpdatedAt, dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field EndedAt", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowIpAddressHistory
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthIpAddressHistory
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthIpAddressHistory
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := github_com_cosmos_gogoproto_types.StdTimeUnmarshal(&m.EndedAt, dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipIpAddressHistory(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthIpAddressHistory
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
func skipIpAddressHistory(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowIpAddressHistory
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
					return 0, ErrIntOverflowIpAddressHistory
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
					return 0, ErrIntOverflowIpAddressHistory
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
				return 0, ErrInvalidLengthIpAddressHistory
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupIpAddressHistory
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthIpAddressHistory
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthIpAddressHistory        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowIpAddressHistory          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupIpAddressHistory = fmt.Errorf("proto: unexpected end of group")
)
