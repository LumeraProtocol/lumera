// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: pastel/supernode/metrics_aggregate.proto

package types

import (
	encoding_binary "encoding/binary"
	fmt "fmt"
	_ "github.com/cosmos/gogoproto/gogoproto"
	proto "github.com/cosmos/gogoproto/proto"
	_ "google.golang.org/protobuf/types/known/timestamppb"
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

type MetricsAggregate struct {
	Metrics     map[string]float64 `protobuf:"bytes,1,rep,name=metrics,proto3" json:"metrics,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"fixed64,2,opt,name=value,proto3"`
	ReportCount uint64             `protobuf:"varint,2,opt,name=report_count,json=reportCount,proto3" json:"report_count,omitempty"`
	Height      int64              `protobuf:"varint,3,opt,name=height,proto3" json:"height,omitempty"`
}

func (m *MetricsAggregate) Reset()         { *m = MetricsAggregate{} }
func (m *MetricsAggregate) String() string { return proto.CompactTextString(m) }
func (*MetricsAggregate) ProtoMessage()    {}
func (*MetricsAggregate) Descriptor() ([]byte, []int) {
	return fileDescriptor_f954e079082b9d37, []int{0}
}
func (m *MetricsAggregate) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *MetricsAggregate) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_MetricsAggregate.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *MetricsAggregate) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MetricsAggregate.Merge(m, src)
}
func (m *MetricsAggregate) XXX_Size() int {
	return m.Size()
}
func (m *MetricsAggregate) XXX_DiscardUnknown() {
	xxx_messageInfo_MetricsAggregate.DiscardUnknown(m)
}

var xxx_messageInfo_MetricsAggregate proto.InternalMessageInfo

func (m *MetricsAggregate) GetMetrics() map[string]float64 {
	if m != nil {
		return m.Metrics
	}
	return nil
}

func (m *MetricsAggregate) GetReportCount() uint64 {
	if m != nil {
		return m.ReportCount
	}
	return 0
}

func (m *MetricsAggregate) GetHeight() int64 {
	if m != nil {
		return m.Height
	}
	return 0
}

func init() {
	proto.RegisterType((*MetricsAggregate)(nil), "pastel.supernode.MetricsAggregate")
	proto.RegisterMapType((map[string]float64)(nil), "pastel.supernode.MetricsAggregate.MetricsEntry")
}

func init() {
	proto.RegisterFile("pastel/supernode/metrics_aggregate.proto", fileDescriptor_f954e079082b9d37)
}

var fileDescriptor_f954e079082b9d37 = []byte{
	// 299 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x5c, 0x90, 0x31, 0x4f, 0xc3, 0x30,
	0x10, 0x85, 0xeb, 0x06, 0x8a, 0x70, 0x3b, 0x54, 0x56, 0x85, 0xa2, 0x0e, 0x26, 0x30, 0x65, 0x8a,
	0x05, 0x2c, 0xa8, 0x1b, 0x20, 0x06, 0x84, 0x58, 0x32, 0xb2, 0x54, 0x69, 0x39, 0xdc, 0xa8, 0x4d,
	0x6c, 0x39, 0x17, 0x20, 0xff, 0x82, 0x9f, 0xc5, 0xd8, 0x81, 0x81, 0x11, 0x25, 0x7f, 0x04, 0x25,
	0x4e, 0x50, 0xd5, 0xed, 0xbd, 0xe7, 0x7b, 0xf7, 0xc9, 0x47, 0x7d, 0x1d, 0x65, 0x08, 0x1b, 0x91,
	0xe5, 0x1a, 0x4c, 0xaa, 0x5e, 0x40, 0x24, 0x80, 0x26, 0x5e, 0x66, 0xf3, 0x48, 0x4a, 0x03, 0x32,
	0x42, 0x08, 0xb4, 0x51, 0xa8, 0xd8, 0xd8, 0x4e, 0x06, 0xff, 0x93, 0xd3, 0x53, 0xa9, 0x94, 0xdc,
	0x80, 0x68, 0xde, 0x17, 0xf9, 0xab, 0xc0, 0x38, 0x81, 0x0c, 0xa3, 0x44, 0xdb, 0xca, 0x74, 0x22,
	0x95, 0x54, 0x8d, 0x14, 0xb5, 0xb2, 0xe9, 0xf9, 0x37, 0xa1, 0xe3, 0x27, 0x0b, 0xb9, 0xe9, 0x18,
	0xec, 0x81, 0x1e, 0xb5, 0x60, 0x97, 0x78, 0x8e, 0x3f, 0xbc, 0x14, 0xc1, 0x3e, 0x2f, 0xd8, 0x2f,
	0x75, 0xc1, 0x7d, 0x8a, 0xa6, 0x08, 0xbb, 0x3e, 0x3b, 0xa3, 0x23, 0x03, 0x5a, 0x19, 0x9c, 0x2f,
	0x55, 0x9e, 0xa2, 0xdb, 0xf7, 0x88, 0x7f, 0x10, 0x0e, 0x6d, 0x76, 0x57, 0x47, 0xec, 0x84, 0x0e,
	0x56, 0x10, 0xcb, 0x15, 0xba, 0x8e, 0x47, 0x7c, 0x27, 0x6c, 0xdd, 0x74, 0x46, 0x47, 0xbb, 0x3b,
	0xd9, 0x98, 0x3a, 0x6b, 0x28, 0x5c, 0xe2, 0x11, 0xff, 0x38, 0xac, 0x25, 0x9b, 0xd0, 0xc3, 0xb7,
	0x68, 0x93, 0x43, 0xb3, 0x95, 0x84, 0xd6, 0xcc, 0xfa, 0xd7, 0xe4, 0xf6, 0xf1, 0xab, 0xe4, 0x64,
	0x5b, 0x72, 0xf2, 0x5b, 0x72, 0xf2, 0x59, 0xf1, 0xde, 0xb6, 0xe2, 0xbd, 0x9f, 0x8a, 0xf7, 0x9e,
	0x2f, 0x64, 0x8c, 0xab, 0x7c, 0x11, 0x2c, 0x55, 0x22, 0xec, 0xa7, 0x52, 0xc0, 0x77, 0x65, 0xd6,
	0xad, 0x13, 0x1f, 0x3b, 0xe7, 0xc7, 0x42, 0x43, 0xb6, 0x18, 0x34, 0xa7, 0xba, 0xfa, 0x0b, 0x00,
	0x00, 0xff, 0xff, 0x85, 0xad, 0x2e, 0xa8, 0x9f, 0x01, 0x00, 0x00,
}

func (m *MetricsAggregate) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MetricsAggregate) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MetricsAggregate) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Height != 0 {
		i = encodeVarintMetricsAggregate(dAtA, i, uint64(m.Height))
		i--
		dAtA[i] = 0x18
	}
	if m.ReportCount != 0 {
		i = encodeVarintMetricsAggregate(dAtA, i, uint64(m.ReportCount))
		i--
		dAtA[i] = 0x10
	}
	if len(m.Metrics) > 0 {
		for k := range m.Metrics {
			v := m.Metrics[k]
			baseI := i
			i -= 8
			encoding_binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(v))))
			i--
			dAtA[i] = 0x11
			i -= len(k)
			copy(dAtA[i:], k)
			i = encodeVarintMetricsAggregate(dAtA, i, uint64(len(k)))
			i--
			dAtA[i] = 0xa
			i = encodeVarintMetricsAggregate(dAtA, i, uint64(baseI-i))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func encodeVarintMetricsAggregate(dAtA []byte, offset int, v uint64) int {
	offset -= sovMetricsAggregate(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *MetricsAggregate) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Metrics) > 0 {
		for k, v := range m.Metrics {
			_ = k
			_ = v
			mapEntrySize := 1 + len(k) + sovMetricsAggregate(uint64(len(k))) + 1 + 8
			n += mapEntrySize + 1 + sovMetricsAggregate(uint64(mapEntrySize))
		}
	}
	if m.ReportCount != 0 {
		n += 1 + sovMetricsAggregate(uint64(m.ReportCount))
	}
	if m.Height != 0 {
		n += 1 + sovMetricsAggregate(uint64(m.Height))
	}
	return n
}

func sovMetricsAggregate(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozMetricsAggregate(x uint64) (n int) {
	return sovMetricsAggregate(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *MetricsAggregate) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMetricsAggregate
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
			return fmt.Errorf("proto: MetricsAggregate: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MetricsAggregate: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Metrics", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMetricsAggregate
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
				return ErrInvalidLengthMetricsAggregate
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthMetricsAggregate
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Metrics == nil {
				m.Metrics = make(map[string]float64)
			}
			var mapkey string
			var mapvalue float64
			for iNdEx < postIndex {
				entryPreIndex := iNdEx
				var wire uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return ErrIntOverflowMetricsAggregate
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
				if fieldNum == 1 {
					var stringLenmapkey uint64
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowMetricsAggregate
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						stringLenmapkey |= uint64(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					intStringLenmapkey := int(stringLenmapkey)
					if intStringLenmapkey < 0 {
						return ErrInvalidLengthMetricsAggregate
					}
					postStringIndexmapkey := iNdEx + intStringLenmapkey
					if postStringIndexmapkey < 0 {
						return ErrInvalidLengthMetricsAggregate
					}
					if postStringIndexmapkey > l {
						return io.ErrUnexpectedEOF
					}
					mapkey = string(dAtA[iNdEx:postStringIndexmapkey])
					iNdEx = postStringIndexmapkey
				} else if fieldNum == 2 {
					var mapvaluetemp uint64
					if (iNdEx + 8) > l {
						return io.ErrUnexpectedEOF
					}
					mapvaluetemp = uint64(encoding_binary.LittleEndian.Uint64(dAtA[iNdEx:]))
					iNdEx += 8
					mapvalue = math.Float64frombits(mapvaluetemp)
				} else {
					iNdEx = entryPreIndex
					skippy, err := skipMetricsAggregate(dAtA[iNdEx:])
					if err != nil {
						return err
					}
					if (skippy < 0) || (iNdEx+skippy) < 0 {
						return ErrInvalidLengthMetricsAggregate
					}
					if (iNdEx + skippy) > postIndex {
						return io.ErrUnexpectedEOF
					}
					iNdEx += skippy
				}
			}
			m.Metrics[mapkey] = mapvalue
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ReportCount", wireType)
			}
			m.ReportCount = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMetricsAggregate
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ReportCount |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Height", wireType)
			}
			m.Height = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMetricsAggregate
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
			skippy, err := skipMetricsAggregate(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthMetricsAggregate
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
func skipMetricsAggregate(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowMetricsAggregate
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
					return 0, ErrIntOverflowMetricsAggregate
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
					return 0, ErrIntOverflowMetricsAggregate
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
				return 0, ErrInvalidLengthMetricsAggregate
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupMetricsAggregate
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthMetricsAggregate
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthMetricsAggregate        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowMetricsAggregate          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupMetricsAggregate = fmt.Errorf("proto: unexpected end of group")
)
