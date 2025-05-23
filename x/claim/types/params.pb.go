// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: lumera/claim/params.proto

package types

import (
	fmt "fmt"
	_ "github.com/cosmos/cosmos-sdk/types/msgservice"
	_ "github.com/cosmos/gogoproto/gogoproto"
	proto "github.com/cosmos/gogoproto/proto"
	_ "google.golang.org/protobuf/types/known/durationpb"
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

// Params defines the parameters for the module.
type Params struct {
	EnableClaims      bool   `protobuf:"varint,1,opt,name=enable_claims,json=enableClaims,proto3" json:"enable_claims"`
	ClaimEndTime      int64  `protobuf:"varint,3,opt,name=claim_end_time,json=claimEndTime,proto3" json:"claim_end_time,omitempty"`
	MaxClaimsPerBlock uint64 `protobuf:"varint,4,opt,name=max_claims_per_block,json=maxClaimsPerBlock,proto3" json:"max_claims_per_block"`
}

func (m *Params) Reset()         { *m = Params{} }
func (m *Params) String() string { return proto.CompactTextString(m) }
func (*Params) ProtoMessage()    {}
func (*Params) Descriptor() ([]byte, []int) {
	return fileDescriptor_8e7f982a60f53ac8, []int{0}
}
func (m *Params) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Params) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Params.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Params) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Params.Merge(m, src)
}
func (m *Params) XXX_Size() int {
	return m.Size()
}
func (m *Params) XXX_DiscardUnknown() {
	xxx_messageInfo_Params.DiscardUnknown(m)
}

var xxx_messageInfo_Params proto.InternalMessageInfo

func (m *Params) GetEnableClaims() bool {
	if m != nil {
		return m.EnableClaims
	}
	return false
}

func (m *Params) GetClaimEndTime() int64 {
	if m != nil {
		return m.ClaimEndTime
	}
	return 0
}

func (m *Params) GetMaxClaimsPerBlock() uint64 {
	if m != nil {
		return m.MaxClaimsPerBlock
	}
	return 0
}

func init() {
	proto.RegisterType((*Params)(nil), "lumera.claim.Params")
}

func init() { proto.RegisterFile("lumera/claim/params.proto", fileDescriptor_8e7f982a60f53ac8) }

var fileDescriptor_8e7f982a60f53ac8 = []byte{
	// 305 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x92, 0xcc, 0x29, 0xcd, 0x4d,
	0x2d, 0x4a, 0xd4, 0x4f, 0xce, 0x49, 0xcc, 0xcc, 0xd5, 0x2f, 0x48, 0x2c, 0x4a, 0xcc, 0x2d, 0xd6,
	0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0xe2, 0x81, 0x48, 0xe9, 0x81, 0xa5, 0xa4, 0x44, 0xd2, 0xf3,
	0xd3, 0xf3, 0xc1, 0x12, 0xfa, 0x20, 0x16, 0x44, 0x8d, 0x94, 0x5c, 0x7a, 0x7e, 0x7e, 0x7a, 0x4e,
	0xaa, 0x3e, 0x98, 0x97, 0x54, 0x9a, 0xa6, 0x9f, 0x52, 0x5a, 0x94, 0x58, 0x92, 0x99, 0x9f, 0x07,
	0x95, 0x17, 0x4f, 0xce, 0x2f, 0xce, 0xcd, 0x2f, 0xd6, 0xcf, 0x2d, 0x4e, 0xd7, 0x2f, 0x33, 0x04,
	0x51, 0x10, 0x09, 0xa5, 0xed, 0x8c, 0x5c, 0x6c, 0x01, 0x60, 0xdb, 0x84, 0xcc, 0xb8, 0x78, 0x53,
	0xf3, 0x12, 0x93, 0x72, 0x52, 0xe3, 0xc1, 0x36, 0x15, 0x4b, 0x30, 0x2a, 0x30, 0x6a, 0x70, 0x38,
	0x09, 0xbe, 0xba, 0x27, 0x8f, 0x2a, 0x11, 0xc4, 0x03, 0xe1, 0x3a, 0x83, 0x79, 0x42, 0x2a, 0x5c,
	0x7c, 0x60, 0xf1, 0xf8, 0xd4, 0xbc, 0x94, 0xf8, 0x92, 0xcc, 0xdc, 0x54, 0x09, 0x66, 0x05, 0x46,
	0x0d, 0xe6, 0x20, 0x1e, 0xb0, 0xa8, 0x6b, 0x5e, 0x4a, 0x48, 0x66, 0x6e, 0xaa, 0x90, 0x27, 0x97,
	0x48, 0x6e, 0x62, 0x05, 0xd4, 0x84, 0xf8, 0x82, 0xd4, 0xa2, 0xf8, 0xa4, 0x9c, 0xfc, 0xe4, 0x6c,
	0x09, 0x16, 0x05, 0x46, 0x0d, 0x16, 0x27, 0x89, 0x57, 0xf7, 0xe4, 0xb1, 0xca, 0x07, 0x09, 0xe6,
	0x26, 0x56, 0x40, 0x2c, 0x0a, 0x48, 0x2d, 0x72, 0x02, 0x09, 0x59, 0xb1, 0xbc, 0x58, 0x20, 0xcf,
	0xe8, 0xe4, 0x71, 0xe2, 0x91, 0x1c, 0xe3, 0x85, 0x47, 0x72, 0x8c, 0x0f, 0x1e, 0xc9, 0x31, 0x4e,
	0x78, 0x2c, 0xc7, 0x70, 0xe1, 0xb1, 0x1c, 0xc3, 0x8d, 0xc7, 0x72, 0x0c, 0x51, 0x7a, 0xe9, 0x99,
	0x25, 0x19, 0xa5, 0x49, 0x7a, 0xc9, 0xf9, 0xb9, 0xfa, 0x3e, 0xe0, 0xb0, 0x0b, 0x00, 0xf9, 0x35,
	0x39, 0x3f, 0x47, 0x1f, 0x1a, 0xca, 0x15, 0xd0, 0x70, 0x2e, 0xa9, 0x2c, 0x48, 0x2d, 0x4e, 0x62,
	0x03, 0x07, 0x85, 0x31, 0x20, 0x00, 0x00, 0xff, 0xff, 0x9b, 0xbf, 0x8f, 0xa0, 0x84, 0x01, 0x00,
	0x00,
}

func (this *Params) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Params)
	if !ok {
		that2, ok := that.(Params)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.EnableClaims != that1.EnableClaims {
		return false
	}
	if this.ClaimEndTime != that1.ClaimEndTime {
		return false
	}
	if this.MaxClaimsPerBlock != that1.MaxClaimsPerBlock {
		return false
	}
	return true
}
func (m *Params) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Params) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Params) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.MaxClaimsPerBlock != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.MaxClaimsPerBlock))
		i--
		dAtA[i] = 0x20
	}
	if m.ClaimEndTime != 0 {
		i = encodeVarintParams(dAtA, i, uint64(m.ClaimEndTime))
		i--
		dAtA[i] = 0x18
	}
	if m.EnableClaims {
		i--
		if m.EnableClaims {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintParams(dAtA []byte, offset int, v uint64) int {
	offset -= sovParams(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *Params) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.EnableClaims {
		n += 2
	}
	if m.ClaimEndTime != 0 {
		n += 1 + sovParams(uint64(m.ClaimEndTime))
	}
	if m.MaxClaimsPerBlock != 0 {
		n += 1 + sovParams(uint64(m.MaxClaimsPerBlock))
	}
	return n
}

func sovParams(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozParams(x uint64) (n int) {
	return sovParams(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *Params) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowParams
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
			return fmt.Errorf("proto: Params: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Params: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field EnableClaims", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.EnableClaims = bool(v != 0)
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ClaimEndTime", wireType)
			}
			m.ClaimEndTime = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ClaimEndTime |= int64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxClaimsPerBlock", wireType)
			}
			m.MaxClaimsPerBlock = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowParams
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.MaxClaimsPerBlock |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipParams(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthParams
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
func skipParams(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowParams
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
					return 0, ErrIntOverflowParams
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
					return 0, ErrIntOverflowParams
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
				return 0, ErrInvalidLengthParams
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupParams
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthParams
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthParams        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowParams          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupParams = fmt.Errorf("proto: unexpected end of group")
)
