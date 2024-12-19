// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: pastel/supernode/super_node.proto

package types

import (
	fmt "fmt"
	_ "github.com/cosmos/cosmos-proto"
	_ "github.com/cosmos/cosmos-sdk/types"
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

type SuperNode struct {
	ValidatorAddress string              `protobuf:"bytes,1,opt,name=validator_address,json=validatorAddress,proto3" json:"validator_address,omitempty"`
	IpAddress        string              `protobuf:"bytes,2,opt,name=ip_address,json=ipAddress,proto3" json:"ip_address,omitempty"`
	State            SuperNodeState      `protobuf:"varint,3,opt,name=state,proto3,enum=pastel.supernode.SuperNodeState" json:"state,omitempty" yaml:"state"`
	Evidence         []*Evidence         `protobuf:"bytes,4,rep,name=evidence,proto3" json:"evidence,omitempty"`
	LastTimeActive   time.Time           `protobuf:"bytes,5,opt,name=last_time_active,json=lastTimeActive,proto3,stdtime" json:"last_time_active"`
	LastTimeDisabled time.Time           `protobuf:"bytes,6,opt,name=last_time_disabled,json=lastTimeDisabled,proto3,stdtime" json:"last_time_disabled"`
	StartedAt        time.Time           `protobuf:"bytes,7,opt,name=started_at,json=startedAt,proto3,stdtime" json:"started_at"`
	PrevIpAddresses  []*IPAddressHistory `protobuf:"bytes,8,rep,name=prev_ip_addresses,json=prevIpAddresses,proto3" json:"prev_ip_addresses,omitempty"`
	Version          string              `protobuf:"bytes,9,opt,name=version,proto3" json:"version,omitempty"`
	Metrics          *MetricsAggregate   `protobuf:"bytes,10,opt,name=metrics,proto3" json:"metrics,omitempty"`
}

func (m *SuperNode) Reset()         { *m = SuperNode{} }
func (m *SuperNode) String() string { return proto.CompactTextString(m) }
func (*SuperNode) ProtoMessage()    {}
func (*SuperNode) Descriptor() ([]byte, []int) {
	return fileDescriptor_f52a3952700b66b5, []int{0}
}
func (m *SuperNode) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SuperNode) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SuperNode.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SuperNode) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SuperNode.Merge(m, src)
}
func (m *SuperNode) XXX_Size() int {
	return m.Size()
}
func (m *SuperNode) XXX_DiscardUnknown() {
	xxx_messageInfo_SuperNode.DiscardUnknown(m)
}

var xxx_messageInfo_SuperNode proto.InternalMessageInfo

func (m *SuperNode) GetValidatorAddress() string {
	if m != nil {
		return m.ValidatorAddress
	}
	return ""
}

func (m *SuperNode) GetIpAddress() string {
	if m != nil {
		return m.IpAddress
	}
	return ""
}

func (m *SuperNode) GetState() SuperNodeState {
	if m != nil {
		return m.State
	}
	return SuperNodeStateUnspecified
}

func (m *SuperNode) GetEvidence() []*Evidence {
	if m != nil {
		return m.Evidence
	}
	return nil
}

func (m *SuperNode) GetLastTimeActive() time.Time {
	if m != nil {
		return m.LastTimeActive
	}
	return time.Time{}
}

func (m *SuperNode) GetLastTimeDisabled() time.Time {
	if m != nil {
		return m.LastTimeDisabled
	}
	return time.Time{}
}

func (m *SuperNode) GetStartedAt() time.Time {
	if m != nil {
		return m.StartedAt
	}
	return time.Time{}
}

func (m *SuperNode) GetPrevIpAddresses() []*IPAddressHistory {
	if m != nil {
		return m.PrevIpAddresses
	}
	return nil
}

func (m *SuperNode) GetVersion() string {
	if m != nil {
		return m.Version
	}
	return ""
}

func (m *SuperNode) GetMetrics() *MetricsAggregate {
	if m != nil {
		return m.Metrics
	}
	return nil
}

func init() {
	proto.RegisterType((*SuperNode)(nil), "pastel.supernode.SuperNode")
}

func init() { proto.RegisterFile("pastel/supernode/super_node.proto", fileDescriptor_f52a3952700b66b5) }

var fileDescriptor_f52a3952700b66b5 = []byte{
	// 546 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x92, 0xc1, 0x6e, 0xd3, 0x4e,
	0x10, 0xc6, 0xe3, 0x7f, 0xff, 0x69, 0x92, 0x05, 0x95, 0x60, 0x71, 0x30, 0x91, 0xea, 0xa4, 0x39,
	0xa0, 0x70, 0xc0, 0x56, 0x8a, 0xc4, 0x01, 0x71, 0x49, 0x00, 0xa9, 0x15, 0x22, 0x42, 0x0e, 0xe2,
	0xc0, 0xc5, 0x5a, 0xdb, 0x83, 0xbb, 0xc2, 0xf6, 0x5a, 0xbb, 0x1b, 0x43, 0x1e, 0x02, 0xa9, 0x0f,
	0xc3, 0x43, 0xf4, 0x58, 0x71, 0xe2, 0x54, 0x50, 0xf2, 0x06, 0x3c, 0x01, 0x5a, 0xef, 0x6e, 0x82,
	0x9a, 0x5e, 0x7a, 0xdb, 0xd9, 0xf9, 0x7d, 0x9f, 0x46, 0xf3, 0x0d, 0x3a, 0x2a, 0x31, 0x17, 0x90,
	0xf9, 0x7c, 0x51, 0x02, 0x2b, 0x68, 0x02, 0xea, 0x15, 0xca, 0xa7, 0x57, 0x32, 0x2a, 0xa8, 0xdd,
	0x55, 0x88, 0xb7, 0x41, 0x7a, 0x6e, 0x4c, 0x79, 0x4e, 0xb9, 0x1f, 0x61, 0x0e, 0x7e, 0x35, 0x8e,
	0x40, 0xe0, 0xb1, 0x1f, 0x53, 0x52, 0x28, 0x45, 0xef, 0xa1, 0xea, 0x87, 0x75, 0xe5, 0xab, 0x42,
	0xb7, 0xfa, 0x29, 0xa5, 0x69, 0x06, 0x7e, 0x5d, 0x45, 0x8b, 0x4f, 0xbe, 0x20, 0x39, 0x70, 0x81,
	0xf3, 0x52, 0x03, 0x0f, 0x52, 0x9a, 0x52, 0x25, 0x94, 0x2f, 0x23, 0xdb, 0x19, 0x13, 0x2a, 0x92,
	0x40, 0x11, 0xeb, 0x21, 0x7b, 0xa3, 0x1d, 0x20, 0x07, 0xc1, 0x48, 0xcc, 0x43, 0x9c, 0xa6, 0x0c,
	0x52, 0x2c, 0x0c, 0xf9, 0x78, 0x87, 0x24, 0x65, 0x88, 0x93, 0x84, 0x01, 0xe7, 0xe1, 0x19, 0xe1,
	0x82, 0xb2, 0xa5, 0x46, 0x1f, 0xdd, 0xbc, 0x1c, 0xf9, 0x0a, 0xb9, 0xd8, 0x58, 0x0e, 0xbf, 0x35,
	0x51, 0x67, 0x2e, 0x3b, 0x33, 0x9a, 0x80, 0x3d, 0x43, 0xf7, 0x2b, 0x9c, 0x91, 0x04, 0x0b, 0xca,
	0x8c, 0xb1, 0x63, 0x0d, 0xac, 0x51, 0x67, 0x7a, 0xf4, 0xe3, 0xfb, 0x93, 0x43, 0xbd, 0x8f, 0x0f,
	0x86, 0x99, 0x28, 0x64, 0x2e, 0x18, 0x29, 0xd2, 0xa0, 0x5b, 0x5d, 0xfb, 0xb7, 0x0f, 0x11, 0xda,
	0x4e, 0xe8, 0xfc, 0x27, 0x8d, 0x82, 0x0e, 0x29, 0x4d, 0xfb, 0x04, 0x35, 0xeb, 0x59, 0x9c, 0xbd,
	0x81, 0x35, 0x3a, 0x38, 0x1e, 0x78, 0xd7, 0xe3, 0xf2, 0x36, 0xa3, 0xcd, 0x25, 0x37, 0xed, 0xfe,
	0xb9, 0xea, 0xdf, 0x5d, 0xe2, 0x3c, 0x7b, 0x3e, 0xac, 0x85, 0xc3, 0x40, 0x19, 0xd8, 0xcf, 0x50,
	0xdb, 0x6c, 0xd5, 0xf9, 0x7f, 0xb0, 0x37, 0xba, 0x73, 0xdc, 0xdb, 0x35, 0x7b, 0xad, 0x89, 0x60,
	0xc3, 0xda, 0x33, 0xd4, 0xcd, 0x30, 0x17, 0xa1, 0x8c, 0x32, 0xc4, 0xb1, 0x20, 0x15, 0x38, 0xcd,
	0x81, 0x55, 0xeb, 0x55, 0xdc, 0x9e, 0x89, 0xdb, 0x7b, 0x6f, 0xe2, 0x9e, 0xb6, 0x2f, 0xae, 0xfa,
	0x8d, 0xf3, 0x5f, 0x7d, 0x2b, 0x38, 0x90, 0x6a, 0xd9, 0x98, 0xd4, 0x5a, 0x3b, 0x40, 0xf6, 0xd6,
	0x2f, 0x21, 0x1c, 0x47, 0x19, 0x24, 0xce, 0xfe, 0x2d, 0x1c, 0xbb, 0xc6, 0xf1, 0x95, 0x56, 0xdb,
	0x2f, 0x11, 0xe2, 0x02, 0x33, 0x01, 0x49, 0x88, 0x85, 0xd3, 0xba, 0x85, 0x57, 0x47, 0xeb, 0x26,
	0x42, 0x26, 0x5b, 0x32, 0xa8, 0xc2, 0x6d, 0x1c, 0xc0, 0x9d, 0x76, 0xbd, 0xa9, 0xe1, 0xee, 0xa6,
	0x4e, 0xdf, 0xe9, 0x88, 0x4e, 0xd4, 0x51, 0x05, 0xf7, 0xa4, 0xf8, 0xd4, 0x04, 0x07, 0xdc, 0x76,
	0x50, 0xab, 0x02, 0xc6, 0x09, 0x2d, 0x9c, 0x4e, 0x1d, 0xab, 0x29, 0xed, 0x17, 0xa8, 0xa5, 0xef,
	0xd7, 0x41, 0xf5, 0xac, 0x37, 0xf8, 0xbf, 0x55, 0xc0, 0xc4, 0xdc, 0x77, 0x60, 0x24, 0xd3, 0x37,
	0x17, 0x2b, 0xd7, 0xba, 0x5c, 0xb9, 0xd6, 0xef, 0x95, 0x6b, 0x9d, 0xaf, 0xdd, 0xc6, 0xe5, 0xda,
	0x6d, 0xfc, 0x5c, 0xbb, 0x8d, 0x8f, 0xe3, 0x94, 0x88, 0xb3, 0x45, 0xe4, 0xc5, 0x34, 0xf7, 0x95,
	0x61, 0x01, 0xe2, 0x0b, 0x65, 0x9f, 0x75, 0xe5, 0x7f, 0xfd, 0xe7, 0xd8, 0xc5, 0xb2, 0x04, 0x1e,
	0xed, 0xd7, 0xdb, 0x79, 0xfa, 0x37, 0x00, 0x00, 0xff, 0xff, 0x75, 0xfb, 0x46, 0x4a, 0x2a, 0x04,
	0x00, 0x00,
}

func (m *SuperNode) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SuperNode) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SuperNode) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Metrics != nil {
		{
			size, err := m.Metrics.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintSuperNode(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x52
	}
	if len(m.Version) > 0 {
		i -= len(m.Version)
		copy(dAtA[i:], m.Version)
		i = encodeVarintSuperNode(dAtA, i, uint64(len(m.Version)))
		i--
		dAtA[i] = 0x4a
	}
	if len(m.PrevIpAddresses) > 0 {
		for iNdEx := len(m.PrevIpAddresses) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.PrevIpAddresses[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintSuperNode(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x42
		}
	}
	n2, err2 := github_com_cosmos_gogoproto_types.StdTimeMarshalTo(m.StartedAt, dAtA[i-github_com_cosmos_gogoproto_types.SizeOfStdTime(m.StartedAt):])
	if err2 != nil {
		return 0, err2
	}
	i -= n2
	i = encodeVarintSuperNode(dAtA, i, uint64(n2))
	i--
	dAtA[i] = 0x3a
	n3, err3 := github_com_cosmos_gogoproto_types.StdTimeMarshalTo(m.LastTimeDisabled, dAtA[i-github_com_cosmos_gogoproto_types.SizeOfStdTime(m.LastTimeDisabled):])
	if err3 != nil {
		return 0, err3
	}
	i -= n3
	i = encodeVarintSuperNode(dAtA, i, uint64(n3))
	i--
	dAtA[i] = 0x32
	n4, err4 := github_com_cosmos_gogoproto_types.StdTimeMarshalTo(m.LastTimeActive, dAtA[i-github_com_cosmos_gogoproto_types.SizeOfStdTime(m.LastTimeActive):])
	if err4 != nil {
		return 0, err4
	}
	i -= n4
	i = encodeVarintSuperNode(dAtA, i, uint64(n4))
	i--
	dAtA[i] = 0x2a
	if len(m.Evidence) > 0 {
		for iNdEx := len(m.Evidence) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Evidence[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintSuperNode(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x22
		}
	}
	if m.State != 0 {
		i = encodeVarintSuperNode(dAtA, i, uint64(m.State))
		i--
		dAtA[i] = 0x18
	}
	if len(m.IpAddress) > 0 {
		i -= len(m.IpAddress)
		copy(dAtA[i:], m.IpAddress)
		i = encodeVarintSuperNode(dAtA, i, uint64(len(m.IpAddress)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.ValidatorAddress) > 0 {
		i -= len(m.ValidatorAddress)
		copy(dAtA[i:], m.ValidatorAddress)
		i = encodeVarintSuperNode(dAtA, i, uint64(len(m.ValidatorAddress)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintSuperNode(dAtA []byte, offset int, v uint64) int {
	offset -= sovSuperNode(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *SuperNode) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.ValidatorAddress)
	if l > 0 {
		n += 1 + l + sovSuperNode(uint64(l))
	}
	l = len(m.IpAddress)
	if l > 0 {
		n += 1 + l + sovSuperNode(uint64(l))
	}
	if m.State != 0 {
		n += 1 + sovSuperNode(uint64(m.State))
	}
	if len(m.Evidence) > 0 {
		for _, e := range m.Evidence {
			l = e.Size()
			n += 1 + l + sovSuperNode(uint64(l))
		}
	}
	l = github_com_cosmos_gogoproto_types.SizeOfStdTime(m.LastTimeActive)
	n += 1 + l + sovSuperNode(uint64(l))
	l = github_com_cosmos_gogoproto_types.SizeOfStdTime(m.LastTimeDisabled)
	n += 1 + l + sovSuperNode(uint64(l))
	l = github_com_cosmos_gogoproto_types.SizeOfStdTime(m.StartedAt)
	n += 1 + l + sovSuperNode(uint64(l))
	if len(m.PrevIpAddresses) > 0 {
		for _, e := range m.PrevIpAddresses {
			l = e.Size()
			n += 1 + l + sovSuperNode(uint64(l))
		}
	}
	l = len(m.Version)
	if l > 0 {
		n += 1 + l + sovSuperNode(uint64(l))
	}
	if m.Metrics != nil {
		l = m.Metrics.Size()
		n += 1 + l + sovSuperNode(uint64(l))
	}
	return n
}

func sovSuperNode(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozSuperNode(x uint64) (n int) {
	return sovSuperNode(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *SuperNode) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowSuperNode
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
			return fmt.Errorf("proto: SuperNode: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SuperNode: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ValidatorAddress", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSuperNode
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
				return ErrInvalidLengthSuperNode
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthSuperNode
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ValidatorAddress = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field IpAddress", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSuperNode
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
				return ErrInvalidLengthSuperNode
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthSuperNode
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.IpAddress = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field State", wireType)
			}
			m.State = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSuperNode
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
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Evidence", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSuperNode
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
				return ErrInvalidLengthSuperNode
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSuperNode
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Evidence = append(m.Evidence, &Evidence{})
			if err := m.Evidence[len(m.Evidence)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field LastTimeActive", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSuperNode
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
				return ErrInvalidLengthSuperNode
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSuperNode
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := github_com_cosmos_gogoproto_types.StdTimeUnmarshal(&m.LastTimeActive, dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field LastTimeDisabled", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSuperNode
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
				return ErrInvalidLengthSuperNode
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSuperNode
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := github_com_cosmos_gogoproto_types.StdTimeUnmarshal(&m.LastTimeDisabled, dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 7:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field StartedAt", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSuperNode
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
				return ErrInvalidLengthSuperNode
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSuperNode
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := github_com_cosmos_gogoproto_types.StdTimeUnmarshal(&m.StartedAt, dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 8:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PrevIpAddresses", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSuperNode
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
				return ErrInvalidLengthSuperNode
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSuperNode
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.PrevIpAddresses = append(m.PrevIpAddresses, &IPAddressHistory{})
			if err := m.PrevIpAddresses[len(m.PrevIpAddresses)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 9:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Version", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSuperNode
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
				return ErrInvalidLengthSuperNode
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthSuperNode
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Version = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 10:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Metrics", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowSuperNode
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
				return ErrInvalidLengthSuperNode
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthSuperNode
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Metrics == nil {
				m.Metrics = &MetricsAggregate{}
			}
			if err := m.Metrics.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipSuperNode(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthSuperNode
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
func skipSuperNode(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowSuperNode
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
					return 0, ErrIntOverflowSuperNode
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
					return 0, ErrIntOverflowSuperNode
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
				return 0, ErrInvalidLengthSuperNode
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupSuperNode
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthSuperNode
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthSuperNode        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowSuperNode          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupSuperNode = fmt.Errorf("proto: unexpected end of group")
)
