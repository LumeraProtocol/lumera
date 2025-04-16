// Code generated by protoc-gen-go-pulsar. DO NOT EDIT.
package lumeraid

import (
	fmt "fmt"
	runtime "github.com/cosmos/cosmos-proto/runtime"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoiface "google.golang.org/protobuf/runtime/protoiface"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	io "io"
	reflect "reflect"
	sync "sync"
)

var (
	md_HandshakeInfo                    protoreflect.MessageDescriptor
	fd_HandshakeInfo_address            protoreflect.FieldDescriptor
	fd_HandshakeInfo_peer_type          protoreflect.FieldDescriptor
	fd_HandshakeInfo_public_key         protoreflect.FieldDescriptor
	fd_HandshakeInfo_account_public_key protoreflect.FieldDescriptor
	fd_HandshakeInfo_curve              protoreflect.FieldDescriptor
)

func init() {
	file_lumera_lumeraid_handshake_info_proto_init()
	md_HandshakeInfo = File_lumera_lumeraid_handshake_info_proto.Messages().ByName("HandshakeInfo")
	fd_HandshakeInfo_address = md_HandshakeInfo.Fields().ByName("address")
	fd_HandshakeInfo_peer_type = md_HandshakeInfo.Fields().ByName("peer_type")
	fd_HandshakeInfo_public_key = md_HandshakeInfo.Fields().ByName("public_key")
	fd_HandshakeInfo_account_public_key = md_HandshakeInfo.Fields().ByName("account_public_key")
	fd_HandshakeInfo_curve = md_HandshakeInfo.Fields().ByName("curve")
}

var _ protoreflect.Message = (*fastReflection_HandshakeInfo)(nil)

type fastReflection_HandshakeInfo HandshakeInfo

func (x *HandshakeInfo) ProtoReflect() protoreflect.Message {
	return (*fastReflection_HandshakeInfo)(x)
}

func (x *HandshakeInfo) slowProtoReflect() protoreflect.Message {
	mi := &file_lumera_lumeraid_handshake_info_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

var _fastReflection_HandshakeInfo_messageType fastReflection_HandshakeInfo_messageType
var _ protoreflect.MessageType = fastReflection_HandshakeInfo_messageType{}

type fastReflection_HandshakeInfo_messageType struct{}

func (x fastReflection_HandshakeInfo_messageType) Zero() protoreflect.Message {
	return (*fastReflection_HandshakeInfo)(nil)
}
func (x fastReflection_HandshakeInfo_messageType) New() protoreflect.Message {
	return new(fastReflection_HandshakeInfo)
}
func (x fastReflection_HandshakeInfo_messageType) Descriptor() protoreflect.MessageDescriptor {
	return md_HandshakeInfo
}

// Descriptor returns message descriptor, which contains only the protobuf
// type information for the message.
func (x *fastReflection_HandshakeInfo) Descriptor() protoreflect.MessageDescriptor {
	return md_HandshakeInfo
}

// Type returns the message type, which encapsulates both Go and protobuf
// type information. If the Go type information is not needed,
// it is recommended that the message descriptor be used instead.
func (x *fastReflection_HandshakeInfo) Type() protoreflect.MessageType {
	return _fastReflection_HandshakeInfo_messageType
}

// New returns a newly allocated and mutable empty message.
func (x *fastReflection_HandshakeInfo) New() protoreflect.Message {
	return new(fastReflection_HandshakeInfo)
}

// Interface unwraps the message reflection interface and
// returns the underlying ProtoMessage interface.
func (x *fastReflection_HandshakeInfo) Interface() protoreflect.ProtoMessage {
	return (*HandshakeInfo)(x)
}

// Range iterates over every populated field in an undefined order,
// calling f for each field descriptor and value encountered.
// Range returns immediately if f returns false.
// While iterating, mutating operations may only be performed
// on the current field descriptor.
func (x *fastReflection_HandshakeInfo) Range(f func(protoreflect.FieldDescriptor, protoreflect.Value) bool) {
	if x.Address != "" {
		value := protoreflect.ValueOfString(x.Address)
		if !f(fd_HandshakeInfo_address, value) {
			return
		}
	}
	if x.PeerType != int32(0) {
		value := protoreflect.ValueOfInt32(x.PeerType)
		if !f(fd_HandshakeInfo_peer_type, value) {
			return
		}
	}
	if len(x.PublicKey) != 0 {
		value := protoreflect.ValueOfBytes(x.PublicKey)
		if !f(fd_HandshakeInfo_public_key, value) {
			return
		}
	}
	if x.Curve != "" {
		value := protoreflect.ValueOfString(x.Curve)
		if !f(fd_HandshakeInfo_curve, value) {
			return
		}
	}
}

// Has reports whether a field is populated.
//
// Some fields have the property of nullability where it is possible to
// distinguish between the default value of a field and whether the field
// was explicitly populated with the default value. Singular message fields,
// member fields of a oneof, and proto2 scalar fields are nullable. Such
// fields are populated only if explicitly set.
//
// In other cases (aside from the nullable cases above),
// a proto3 scalar field is populated if it contains a non-zero value, and
// a repeated field is populated if it is non-empty.
func (x *fastReflection_HandshakeInfo) Has(fd protoreflect.FieldDescriptor) bool {
	switch fd.FullName() {
	case "lumera.lumeraid.HandshakeInfo.address":
		return x.Address != ""
	case "lumera.lumeraid.HandshakeInfo.peer_type":
		return x.PeerType != int32(0)
	case "lumera.lumeraid.HandshakeInfo.public_key":
		return len(x.PublicKey) != 0
	case "lumera.lumeraid.HandshakeInfo.curve":
		return x.Curve != ""
	default:
		if fd.IsExtension() {
			panic(fmt.Errorf("proto3 declared messages do not support extensions: lumera.lumeraid.HandshakeInfo"))
		}
		panic(fmt.Errorf("message lumera.lumeraid.HandshakeInfo does not contain field %s", fd.FullName()))
	}
}

// Clear clears the field such that a subsequent Has call reports false.
//
// Clearing an extension field clears both the extension type and value
// associated with the given field number.
//
// Clear is a mutating operation and unsafe for concurrent use.
func (x *fastReflection_HandshakeInfo) Clear(fd protoreflect.FieldDescriptor) {
	switch fd.FullName() {
	case "lumera.lumeraid.HandshakeInfo.address":
		x.Address = ""
	case "lumera.lumeraid.HandshakeInfo.peer_type":
		x.PeerType = int32(0)
	case "lumera.lumeraid.HandshakeInfo.public_key":
		x.PublicKey = nil
	case "lumera.lumeraid.HandshakeInfo.curve":
		x.Curve = ""
	default:
		if fd.IsExtension() {
			panic(fmt.Errorf("proto3 declared messages do not support extensions: lumera.lumeraid.HandshakeInfo"))
		}
		panic(fmt.Errorf("message lumera.lumeraid.HandshakeInfo does not contain field %s", fd.FullName()))
	}
}

// Get retrieves the value for a field.
//
// For unpopulated scalars, it returns the default value, where
// the default value of a bytes scalar is guaranteed to be a copy.
// For unpopulated composite types, it returns an empty, read-only view
// of the value; to obtain a mutable reference, use Mutable.
func (x *fastReflection_HandshakeInfo) Get(descriptor protoreflect.FieldDescriptor) protoreflect.Value {
	switch descriptor.FullName() {
	case "lumera.lumeraid.HandshakeInfo.address":
		value := x.Address
		return protoreflect.ValueOfString(value)
	case "lumera.lumeraid.HandshakeInfo.peer_type":
		value := x.PeerType
		return protoreflect.ValueOfInt32(value)
	case "lumera.lumeraid.HandshakeInfo.public_key":
		value := x.PublicKey
		return protoreflect.ValueOfBytes(value)
	case "lumera.lumeraid.HandshakeInfo.curve":
		value := x.Curve
		return protoreflect.ValueOfString(value)
	default:
		if descriptor.IsExtension() {
			panic(fmt.Errorf("proto3 declared messages do not support extensions: lumera.lumeraid.HandshakeInfo"))
		}
		panic(fmt.Errorf("message lumera.lumeraid.HandshakeInfo does not contain field %s", descriptor.FullName()))
	}
}

// Set stores the value for a field.
//
// For a field belonging to a oneof, it implicitly clears any other field
// that may be currently set within the same oneof.
// For extension fields, it implicitly stores the provided ExtensionType.
// When setting a composite type, it is unspecified whether the stored value
// aliases the source's memory in any way. If the composite value is an
// empty, read-only value, then it panics.
//
// Set is a mutating operation and unsafe for concurrent use.
func (x *fastReflection_HandshakeInfo) Set(fd protoreflect.FieldDescriptor, value protoreflect.Value) {
	switch fd.FullName() {
	case "lumera.lumeraid.HandshakeInfo.address":
		x.Address = value.Interface().(string)
	case "lumera.lumeraid.HandshakeInfo.peer_type":
		x.PeerType = int32(value.Int())
	case "lumera.lumeraid.HandshakeInfo.public_key":
		x.PublicKey = value.Bytes()
	case "lumera.lumeraid.HandshakeInfo.curve":
		x.Curve = value.Interface().(string)
	default:
		if fd.IsExtension() {
			panic(fmt.Errorf("proto3 declared messages do not support extensions: lumera.lumeraid.HandshakeInfo"))
		}
		panic(fmt.Errorf("message lumera.lumeraid.HandshakeInfo does not contain field %s", fd.FullName()))
	}
}

// Mutable returns a mutable reference to a composite type.
//
// If the field is unpopulated, it may allocate a composite value.
// For a field belonging to a oneof, it implicitly clears any other field
// that may be currently set within the same oneof.
// For extension fields, it implicitly stores the provided ExtensionType
// if not already stored.
// It panics if the field does not contain a composite type.
//
// Mutable is a mutating operation and unsafe for concurrent use.
func (x *fastReflection_HandshakeInfo) Mutable(fd protoreflect.FieldDescriptor) protoreflect.Value {
	switch fd.FullName() {
	case "lumera.lumeraid.HandshakeInfo.address":
		panic(fmt.Errorf("field address of message lumera.lumeraid.HandshakeInfo is not mutable"))
	case "lumera.lumeraid.HandshakeInfo.peer_type":
		panic(fmt.Errorf("field peer_type of message lumera.lumeraid.HandshakeInfo is not mutable"))
	case "lumera.lumeraid.HandshakeInfo.public_key":
		panic(fmt.Errorf("field public_key of message lumera.lumeraid.HandshakeInfo is not mutable"))
	case "lumera.lumeraid.HandshakeInfo.curve":
		panic(fmt.Errorf("field curve of message lumera.lumeraid.HandshakeInfo is not mutable"))
	default:
		if fd.IsExtension() {
			panic(fmt.Errorf("proto3 declared messages do not support extensions: lumera.lumeraid.HandshakeInfo"))
		}
		panic(fmt.Errorf("message lumera.lumeraid.HandshakeInfo does not contain field %s", fd.FullName()))
	}
}

// NewField returns a new value that is assignable to the field
// for the given descriptor. For scalars, this returns the default value.
// For lists, maps, and messages, this returns a new, empty, mutable value.
func (x *fastReflection_HandshakeInfo) NewField(fd protoreflect.FieldDescriptor) protoreflect.Value {
	switch fd.FullName() {
	case "lumera.lumeraid.HandshakeInfo.address":
		return protoreflect.ValueOfString("")
	case "lumera.lumeraid.HandshakeInfo.peer_type":
		return protoreflect.ValueOfInt32(int32(0))
	case "lumera.lumeraid.HandshakeInfo.public_key":
		return protoreflect.ValueOfBytes(nil)
	case "lumera.lumeraid.HandshakeInfo.curve":
		return protoreflect.ValueOfString("")
	default:
		if fd.IsExtension() {
			panic(fmt.Errorf("proto3 declared messages do not support extensions: lumera.lumeraid.HandshakeInfo"))
		}
		panic(fmt.Errorf("message lumera.lumeraid.HandshakeInfo does not contain field %s", fd.FullName()))
	}
}

// WhichOneof reports which field within the oneof is populated,
// returning nil if none are populated.
// It panics if the oneof descriptor does not belong to this message.
func (x *fastReflection_HandshakeInfo) WhichOneof(d protoreflect.OneofDescriptor) protoreflect.FieldDescriptor {
	switch d.FullName() {
	default:
		panic(fmt.Errorf("%s is not a oneof field in lumera.lumeraid.HandshakeInfo", d.FullName()))
	}
	panic("unreachable")
}

// GetUnknown retrieves the entire list of unknown fields.
// The caller may only mutate the contents of the RawFields
// if the mutated bytes are stored back into the message with SetUnknown.
func (x *fastReflection_HandshakeInfo) GetUnknown() protoreflect.RawFields {
	return x.unknownFields
}

// SetUnknown stores an entire list of unknown fields.
// The raw fields must be syntactically valid according to the wire format.
// An implementation may panic if this is not the case.
// Once stored, the caller must not mutate the content of the RawFields.
// An empty RawFields may be passed to clear the fields.
//
// SetUnknown is a mutating operation and unsafe for concurrent use.
func (x *fastReflection_HandshakeInfo) SetUnknown(fields protoreflect.RawFields) {
	x.unknownFields = fields
}

// IsValid reports whether the message is valid.
//
// An invalid message is an empty, read-only value.
//
// An invalid message often corresponds to a nil pointer of the concrete
// message type, but the details are implementation dependent.
// Validity is not part of the protobuf data model, and may not
// be preserved in marshaling or other operations.
func (x *fastReflection_HandshakeInfo) IsValid() bool {
	return x != nil
}

// ProtoMethods returns optional fastReflectionFeature-path implementations of various operations.
// This method may return nil.
//
// The returned methods type is identical to
// "google.golang.org/protobuf/runtime/protoiface".Methods.
// Consult the protoiface package documentation for details.
func (x *fastReflection_HandshakeInfo) ProtoMethods() *protoiface.Methods {
	size := func(input protoiface.SizeInput) protoiface.SizeOutput {
		x := input.Message.Interface().(*HandshakeInfo)
		if x == nil {
			return protoiface.SizeOutput{
				NoUnkeyedLiterals: input.NoUnkeyedLiterals,
				Size:              0,
			}
		}
		options := runtime.SizeInputToOptions(input)
		_ = options
		var n int
		var l int
		_ = l
		l = len(x.Address)
		if l > 0 {
			n += 1 + l + runtime.Sov(uint64(l))
		}
		if x.PeerType != 0 {
			n += 1 + runtime.Sov(uint64(x.PeerType))
		}
		l = len(x.PublicKey)
		if l > 0 {
			n += 1 + l + runtime.Sov(uint64(l))
		}
		l = len(x.Curve)
		if l > 0 {
			n += 1 + l + runtime.Sov(uint64(l))
		}
		if x.unknownFields != nil {
			n += len(x.unknownFields)
		}
		return protoiface.SizeOutput{
			NoUnkeyedLiterals: input.NoUnkeyedLiterals,
			Size:              n,
		}
	}

	marshal := func(input protoiface.MarshalInput) (protoiface.MarshalOutput, error) {
		x := input.Message.Interface().(*HandshakeInfo)
		if x == nil {
			return protoiface.MarshalOutput{
				NoUnkeyedLiterals: input.NoUnkeyedLiterals,
				Buf:               input.Buf,
			}, nil
		}
		options := runtime.MarshalInputToOptions(input)
		_ = options
		size := options.Size(x)
		dAtA := make([]byte, size)
		i := len(dAtA)
		_ = i
		var l int
		_ = l
		if x.unknownFields != nil {
			i -= len(x.unknownFields)
			copy(dAtA[i:], x.unknownFields)
		}
		if len(x.Curve) > 0 {
			i -= len(x.Curve)
			copy(dAtA[i:], x.Curve)
			i = runtime.EncodeVarint(dAtA, i, uint64(len(x.Curve)))
			i--
			dAtA[i] = 0x22
		}
		if len(x.PublicKey) > 0 {
			i -= len(x.PublicKey)
			copy(dAtA[i:], x.PublicKey)
			i = runtime.EncodeVarint(dAtA, i, uint64(len(x.PublicKey)))
			i--
			dAtA[i] = 0x1a
		}
		if x.PeerType != 0 {
			i = runtime.EncodeVarint(dAtA, i, uint64(x.PeerType))
			i--
			dAtA[i] = 0x10
		}
		if len(x.Address) > 0 {
			i -= len(x.Address)
			copy(dAtA[i:], x.Address)
			i = runtime.EncodeVarint(dAtA, i, uint64(len(x.Address)))
			i--
			dAtA[i] = 0xa
		}
		if input.Buf != nil {
			input.Buf = append(input.Buf, dAtA...)
		} else {
			input.Buf = dAtA
		}
		return protoiface.MarshalOutput{
			NoUnkeyedLiterals: input.NoUnkeyedLiterals,
			Buf:               input.Buf,
		}, nil
	}
	unmarshal := func(input protoiface.UnmarshalInput) (protoiface.UnmarshalOutput, error) {
		x := input.Message.Interface().(*HandshakeInfo)
		if x == nil {
			return protoiface.UnmarshalOutput{
				NoUnkeyedLiterals: input.NoUnkeyedLiterals,
				Flags:             input.Flags,
			}, nil
		}
		options := runtime.UnmarshalInputToOptions(input)
		_ = options
		dAtA := input.Buf
		l := len(dAtA)
		iNdEx := 0
		for iNdEx < l {
			preIndex := iNdEx
			var wire uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrIntOverflow
				}
				if iNdEx >= l {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
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
				return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, fmt.Errorf("proto: HandshakeInfo: wiretype end group for non-group")
			}
			if fieldNum <= 0 {
				return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, fmt.Errorf("proto: HandshakeInfo: illegal tag %d (wire type %d)", fieldNum, wire)
			}
			switch fieldNum {
			case 1:
				if wireType != 2 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, fmt.Errorf("proto: wrong wireType = %d for field Address", wireType)
				}
				var stringLen uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrIntOverflow
					}
					if iNdEx >= l {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
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
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
				}
				postIndex := iNdEx + intStringLen
				if postIndex < 0 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
				}
				if postIndex > l {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
				}
				x.Address = string(dAtA[iNdEx:postIndex])
				iNdEx = postIndex
			case 2:
				if wireType != 0 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, fmt.Errorf("proto: wrong wireType = %d for field PeerType", wireType)
				}
				x.PeerType = 0
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrIntOverflow
					}
					if iNdEx >= l {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					x.PeerType |= int32(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
			case 3:
				if wireType != 2 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, fmt.Errorf("proto: wrong wireType = %d for field PublicKey", wireType)
				}
				var byteLen int
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrIntOverflow
					}
					if iNdEx >= l {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					byteLen |= int(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				if byteLen < 0 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
				}
				postIndex := iNdEx + byteLen
				if postIndex < 0 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
				}
				if postIndex > l {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
				}
				x.PublicKey = append(x.PublicKey[:0], dAtA[iNdEx:postIndex]...)
				if x.PublicKey == nil {
					x.PublicKey = []byte{}
				}
				iNdEx = postIndex
			case 4:
				if wireType != 2 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, fmt.Errorf("proto: wrong wireType = %d for field Curve", wireType)
				}
				var stringLen uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrIntOverflow
					}
					if iNdEx >= l {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
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
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
				}
				postIndex := iNdEx + intStringLen
				if postIndex < 0 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
				}
				if postIndex > l {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
				}
				x.Curve = string(dAtA[iNdEx:postIndex])
				iNdEx = postIndex
			default:
				iNdEx = preIndex
				skippy, err := runtime.Skip(dAtA[iNdEx:])
				if err != nil {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, err
				}
				if (skippy < 0) || (iNdEx+skippy) < 0 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
				}
				if (iNdEx + skippy) > l {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
				}
				if !options.DiscardUnknown {
					x.unknownFields = append(x.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
				}
				iNdEx += skippy
			}
		}

		if iNdEx > l {
			return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
		}
		return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, nil
	}
	return &protoiface.Methods{
		NoUnkeyedLiterals: struct{}{},
		Flags:             protoiface.SupportMarshalDeterministic | protoiface.SupportUnmarshalDiscardUnknown,
		Size:              size,
		Marshal:           marshal,
		Unmarshal:         unmarshal,
		Merge:             nil,
		CheckInitialized:  nil,
	}
}

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.0
// 	protoc        (unknown)
// source: lumera/lumeraid/handshake_info.proto

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// HandshakeInfo message
type HandshakeInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address   string `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`                      // Cosmos account address
	PeerType  int32  `protobuf:"varint,2,opt,name=peer_type,json=peerType,proto3" json:"peer_type,omitempty"`   // Peer type (0 = Simplenode, 1 = Supernode)
	PublicKey []byte `protobuf:"bytes,3,opt,name=public_key,json=publicKey,proto3" json:"public_key,omitempty"` // ephemeral public key
	Curve     string `protobuf:"bytes,4,opt,name=curve,proto3" json:"curve,omitempty"`                          // Curve type (e.g., P256, P384, P521)
}

func (x *HandshakeInfo) Reset() {
	*x = HandshakeInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lumera_lumeraid_handshake_info_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HandshakeInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HandshakeInfo) ProtoMessage() {}

// Deprecated: Use HandshakeInfo.ProtoReflect.Descriptor instead.
func (*HandshakeInfo) Descriptor() ([]byte, []int) {
	return file_lumera_lumeraid_handshake_info_proto_rawDescGZIP(), []int{0}
}

func (x *HandshakeInfo) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *HandshakeInfo) GetPeerType() int32 {
	if x != nil {
		return x.PeerType
	}
	return 0
}

func (x *HandshakeInfo) GetPublicKey() []byte {
	if x != nil {
		return x.PublicKey
	}
	return nil
}

func (x *HandshakeInfo) GetCurve() string {
	if x != nil {
		return x.Curve
	}
	return ""
}

var File_lumera_lumeraid_handshake_info_proto protoreflect.FileDescriptor

var file_lumera_lumeraid_handshake_info_proto_rawDesc = []byte{
	0x0a, 0x24, 0x6c, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x2f, 0x6c, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x69,
	0x64, 0x2f, 0x68, 0x61, 0x6e, 0x64, 0x73, 0x68, 0x61, 0x6b, 0x65, 0x5f, 0x69, 0x6e, 0x66, 0x6f,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0f, 0x6c, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x2e, 0x6c,
	0x75, 0x6d, 0x65, 0x72, 0x61, 0x69, 0x64, 0x22, 0x7b, 0x0a, 0x0d, 0x48, 0x61, 0x6e, 0x64, 0x73,
	0x68, 0x61, 0x6b, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72,
	0x65, 0x73, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65,
	0x73, 0x73, 0x12, 0x1b, 0x0a, 0x09, 0x70, 0x65, 0x65, 0x72, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x70, 0x65, 0x65, 0x72, 0x54, 0x79, 0x70, 0x65, 0x12,
	0x1d, 0x0a, 0x0a, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x09, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x4b, 0x65, 0x79, 0x12, 0x14,
	0x0a, 0x05, 0x63, 0x75, 0x72, 0x76, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x63,
	0x75, 0x72, 0x76, 0x65, 0x42, 0xbc, 0x01, 0x0a, 0x13, 0x63, 0x6f, 0x6d, 0x2e, 0x6c, 0x75, 0x6d,
	0x65, 0x72, 0x61, 0x2e, 0x6c, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x69, 0x64, 0x42, 0x12, 0x48, 0x61,
	0x6e, 0x64, 0x73, 0x68, 0x61, 0x6b, 0x65, 0x49, 0x6e, 0x66, 0x6f, 0x50, 0x72, 0x6f, 0x74, 0x6f,
	0x50, 0x01, 0x5a, 0x34, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x4c,
	0x75, 0x6d, 0x65, 0x72, 0x61, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2f, 0x6c, 0x75,
	0x6d, 0x65, 0x72, 0x61, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x6c, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x2f,
	0x6c, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x69, 0x64, 0xa2, 0x02, 0x03, 0x4c, 0x4c, 0x58, 0xaa, 0x02,
	0x0f, 0x4c, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x2e, 0x4c, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x69, 0x64,
	0xca, 0x02, 0x0f, 0x4c, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x5c, 0x4c, 0x75, 0x6d, 0x65, 0x72, 0x61,
	0x69, 0x64, 0xe2, 0x02, 0x1b, 0x4c, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x5c, 0x4c, 0x75, 0x6d, 0x65,
	0x72, 0x61, 0x69, 0x64, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61,
	0xea, 0x02, 0x10, 0x4c, 0x75, 0x6d, 0x65, 0x72, 0x61, 0x3a, 0x3a, 0x4c, 0x75, 0x6d, 0x65, 0x72,
	0x61, 0x69, 0x64, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_lumera_lumeraid_handshake_info_proto_rawDescOnce sync.Once
	file_lumera_lumeraid_handshake_info_proto_rawDescData = file_lumera_lumeraid_handshake_info_proto_rawDesc
)

func file_lumera_lumeraid_handshake_info_proto_rawDescGZIP() []byte {
	file_lumera_lumeraid_handshake_info_proto_rawDescOnce.Do(func() {
		file_lumera_lumeraid_handshake_info_proto_rawDescData = protoimpl.X.CompressGZIP(file_lumera_lumeraid_handshake_info_proto_rawDescData)
	})
	return file_lumera_lumeraid_handshake_info_proto_rawDescData
}

var file_lumera_lumeraid_handshake_info_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_lumera_lumeraid_handshake_info_proto_goTypes = []interface{}{
	(*HandshakeInfo)(nil), // 0: lumera.lumeraid.HandshakeInfo
}
var file_lumera_lumeraid_handshake_info_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_lumera_lumeraid_handshake_info_proto_init() }
func file_lumera_lumeraid_handshake_info_proto_init() {
	if File_lumera_lumeraid_handshake_info_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_lumera_lumeraid_handshake_info_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HandshakeInfo); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_lumera_lumeraid_handshake_info_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_lumera_lumeraid_handshake_info_proto_goTypes,
		DependencyIndexes: file_lumera_lumeraid_handshake_info_proto_depIdxs,
		MessageInfos:      file_lumera_lumeraid_handshake_info_proto_msgTypes,
	}.Build()
	File_lumera_lumeraid_handshake_info_proto = out.File
	file_lumera_lumeraid_handshake_info_proto_rawDesc = nil
	file_lumera_lumeraid_handshake_info_proto_goTypes = nil
	file_lumera_lumeraid_handshake_info_proto_depIdxs = nil
}
