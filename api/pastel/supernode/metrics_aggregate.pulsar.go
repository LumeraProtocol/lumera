// Code generated by protoc-gen-go-pulsar. DO NOT EDIT.
package supernode

import (
	binary "encoding/binary"
	fmt "fmt"
	io "io"
	math "math"
	reflect "reflect"
	sort "sort"
	sync "sync"

	runtime "github.com/cosmos/cosmos-proto/runtime"
	_ "github.com/cosmos/gogoproto/gogoproto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoiface "google.golang.org/protobuf/runtime/protoiface"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

var _ protoreflect.Map = (*_MetricsAggregate_1_map)(nil)

type _MetricsAggregate_1_map struct {
	m *map[string]float64
}

func (x *_MetricsAggregate_1_map) Len() int {
	if x.m == nil {
		return 0
	}
	return len(*x.m)
}

func (x *_MetricsAggregate_1_map) Range(f func(protoreflect.MapKey, protoreflect.Value) bool) {
	if x.m == nil {
		return
	}
	for k, v := range *x.m {
		mapKey := (protoreflect.MapKey)(protoreflect.ValueOfString(k))
		mapValue := protoreflect.ValueOfFloat64(v)
		if !f(mapKey, mapValue) {
			break
		}
	}
}

func (x *_MetricsAggregate_1_map) Has(key protoreflect.MapKey) bool {
	if x.m == nil {
		return false
	}
	keyUnwrapped := key.String()
	concreteValue := keyUnwrapped
	_, ok := (*x.m)[concreteValue]
	return ok
}

func (x *_MetricsAggregate_1_map) Clear(key protoreflect.MapKey) {
	if x.m == nil {
		return
	}
	keyUnwrapped := key.String()
	concreteKey := keyUnwrapped
	delete(*x.m, concreteKey)
}

func (x *_MetricsAggregate_1_map) Get(key protoreflect.MapKey) protoreflect.Value {
	if x.m == nil {
		return protoreflect.Value{}
	}
	keyUnwrapped := key.String()
	concreteKey := keyUnwrapped
	v, ok := (*x.m)[concreteKey]
	if !ok {
		return protoreflect.Value{}
	}
	return protoreflect.ValueOfFloat64(v)
}

func (x *_MetricsAggregate_1_map) Set(key protoreflect.MapKey, value protoreflect.Value) {
	if !key.IsValid() || !value.IsValid() {
		panic("invalid key or value provided")
	}
	keyUnwrapped := key.String()
	concreteKey := keyUnwrapped
	valueUnwrapped := value.Float()
	concreteValue := valueUnwrapped
	(*x.m)[concreteKey] = concreteValue
}

func (x *_MetricsAggregate_1_map) Mutable(key protoreflect.MapKey) protoreflect.Value {
	panic("should not call Mutable on protoreflect.Map whose value is not of type protoreflect.Message")
}

func (x *_MetricsAggregate_1_map) NewValue() protoreflect.Value {
	v := float64(0)
	return protoreflect.ValueOfFloat64(v)
}

func (x *_MetricsAggregate_1_map) IsValid() bool {
	return x.m != nil
}

var (
	md_MetricsAggregate              protoreflect.MessageDescriptor
	fd_MetricsAggregate_metrics      protoreflect.FieldDescriptor
	fd_MetricsAggregate_report_count protoreflect.FieldDescriptor
	fd_MetricsAggregate_last_updated protoreflect.FieldDescriptor
)

func init() {
	file_pastel_supernode_metrics_aggregate_proto_init()
	md_MetricsAggregate = File_pastel_supernode_metrics_aggregate_proto.Messages().ByName("MetricsAggregate")
	fd_MetricsAggregate_metrics = md_MetricsAggregate.Fields().ByName("metrics")
	fd_MetricsAggregate_report_count = md_MetricsAggregate.Fields().ByName("report_count")
	fd_MetricsAggregate_last_updated = md_MetricsAggregate.Fields().ByName("last_updated")
}

var _ protoreflect.Message = (*fastReflection_MetricsAggregate)(nil)

type fastReflection_MetricsAggregate MetricsAggregate

func (x *MetricsAggregate) ProtoReflect() protoreflect.Message {
	return (*fastReflection_MetricsAggregate)(x)
}

func (x *MetricsAggregate) slowProtoReflect() protoreflect.Message {
	mi := &file_pastel_supernode_metrics_aggregate_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

var _fastReflection_MetricsAggregate_messageType fastReflection_MetricsAggregate_messageType
var _ protoreflect.MessageType = fastReflection_MetricsAggregate_messageType{}

type fastReflection_MetricsAggregate_messageType struct{}

func (x fastReflection_MetricsAggregate_messageType) Zero() protoreflect.Message {
	return (*fastReflection_MetricsAggregate)(nil)
}
func (x fastReflection_MetricsAggregate_messageType) New() protoreflect.Message {
	return new(fastReflection_MetricsAggregate)
}
func (x fastReflection_MetricsAggregate_messageType) Descriptor() protoreflect.MessageDescriptor {
	return md_MetricsAggregate
}

// Descriptor returns message descriptor, which contains only the protobuf
// type information for the message.
func (x *fastReflection_MetricsAggregate) Descriptor() protoreflect.MessageDescriptor {
	return md_MetricsAggregate
}

// Type returns the message type, which encapsulates both Go and protobuf
// type information. If the Go type information is not needed,
// it is recommended that the message descriptor be used instead.
func (x *fastReflection_MetricsAggregate) Type() protoreflect.MessageType {
	return _fastReflection_MetricsAggregate_messageType
}

// New returns a newly allocated and mutable empty message.
func (x *fastReflection_MetricsAggregate) New() protoreflect.Message {
	return new(fastReflection_MetricsAggregate)
}

// Interface unwraps the message reflection interface and
// returns the underlying ProtoMessage interface.
func (x *fastReflection_MetricsAggregate) Interface() protoreflect.ProtoMessage {
	return (*MetricsAggregate)(x)
}

// Range iterates over every populated field in an undefined order,
// calling f for each field descriptor and value encountered.
// Range returns immediately if f returns false.
// While iterating, mutating operations may only be performed
// on the current field descriptor.
func (x *fastReflection_MetricsAggregate) Range(f func(protoreflect.FieldDescriptor, protoreflect.Value) bool) {
	if len(x.Metrics) != 0 {
		value := protoreflect.ValueOfMap(&_MetricsAggregate_1_map{m: &x.Metrics})
		if !f(fd_MetricsAggregate_metrics, value) {
			return
		}
	}
	if x.ReportCount != uint64(0) {
		value := protoreflect.ValueOfUint64(x.ReportCount)
		if !f(fd_MetricsAggregate_report_count, value) {
			return
		}
	}
	if x.LastUpdated != nil {
		value := protoreflect.ValueOfMessage(x.LastUpdated.ProtoReflect())
		if !f(fd_MetricsAggregate_last_updated, value) {
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
func (x *fastReflection_MetricsAggregate) Has(fd protoreflect.FieldDescriptor) bool {
	switch fd.FullName() {
	case "pastel.supernode.MetricsAggregate.metrics":
		return len(x.Metrics) != 0
	case "pastel.supernode.MetricsAggregate.report_count":
		return x.ReportCount != uint64(0)
	case "pastel.supernode.MetricsAggregate.last_updated":
		return x.LastUpdated != nil
	default:
		if fd.IsExtension() {
			panic(fmt.Errorf("proto3 declared messages do not support extensions: pastel.supernode.MetricsAggregate"))
		}
		panic(fmt.Errorf("message pastel.supernode.MetricsAggregate does not contain field %s", fd.FullName()))
	}
}

// Clear clears the field such that a subsequent Has call reports false.
//
// Clearing an extension field clears both the extension type and value
// associated with the given field number.
//
// Clear is a mutating operation and unsafe for concurrent use.
func (x *fastReflection_MetricsAggregate) Clear(fd protoreflect.FieldDescriptor) {
	switch fd.FullName() {
	case "pastel.supernode.MetricsAggregate.metrics":
		x.Metrics = nil
	case "pastel.supernode.MetricsAggregate.report_count":
		x.ReportCount = uint64(0)
	case "pastel.supernode.MetricsAggregate.last_updated":
		x.LastUpdated = nil
	default:
		if fd.IsExtension() {
			panic(fmt.Errorf("proto3 declared messages do not support extensions: pastel.supernode.MetricsAggregate"))
		}
		panic(fmt.Errorf("message pastel.supernode.MetricsAggregate does not contain field %s", fd.FullName()))
	}
}

// Get retrieves the value for a field.
//
// For unpopulated scalars, it returns the default value, where
// the default value of a bytes scalar is guaranteed to be a copy.
// For unpopulated composite types, it returns an empty, read-only view
// of the value; to obtain a mutable reference, use Mutable.
func (x *fastReflection_MetricsAggregate) Get(descriptor protoreflect.FieldDescriptor) protoreflect.Value {
	switch descriptor.FullName() {
	case "pastel.supernode.MetricsAggregate.metrics":
		if len(x.Metrics) == 0 {
			return protoreflect.ValueOfMap(&_MetricsAggregate_1_map{})
		}
		mapValue := &_MetricsAggregate_1_map{m: &x.Metrics}
		return protoreflect.ValueOfMap(mapValue)
	case "pastel.supernode.MetricsAggregate.report_count":
		value := x.ReportCount
		return protoreflect.ValueOfUint64(value)
	case "pastel.supernode.MetricsAggregate.last_updated":
		value := x.LastUpdated
		return protoreflect.ValueOfMessage(value.ProtoReflect())
	default:
		if descriptor.IsExtension() {
			panic(fmt.Errorf("proto3 declared messages do not support extensions: pastel.supernode.MetricsAggregate"))
		}
		panic(fmt.Errorf("message pastel.supernode.MetricsAggregate does not contain field %s", descriptor.FullName()))
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
func (x *fastReflection_MetricsAggregate) Set(fd protoreflect.FieldDescriptor, value protoreflect.Value) {
	switch fd.FullName() {
	case "pastel.supernode.MetricsAggregate.metrics":
		mv := value.Map()
		cmv := mv.(*_MetricsAggregate_1_map)
		x.Metrics = *cmv.m
	case "pastel.supernode.MetricsAggregate.report_count":
		x.ReportCount = value.Uint()
	case "pastel.supernode.MetricsAggregate.last_updated":
		x.LastUpdated = value.Message().Interface().(*timestamppb.Timestamp)
	default:
		if fd.IsExtension() {
			panic(fmt.Errorf("proto3 declared messages do not support extensions: pastel.supernode.MetricsAggregate"))
		}
		panic(fmt.Errorf("message pastel.supernode.MetricsAggregate does not contain field %s", fd.FullName()))
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
func (x *fastReflection_MetricsAggregate) Mutable(fd protoreflect.FieldDescriptor) protoreflect.Value {
	switch fd.FullName() {
	case "pastel.supernode.MetricsAggregate.metrics":
		if x.Metrics == nil {
			x.Metrics = make(map[string]float64)
		}
		value := &_MetricsAggregate_1_map{m: &x.Metrics}
		return protoreflect.ValueOfMap(value)
	case "pastel.supernode.MetricsAggregate.last_updated":
		if x.LastUpdated == nil {
			x.LastUpdated = new(timestamppb.Timestamp)
		}
		return protoreflect.ValueOfMessage(x.LastUpdated.ProtoReflect())
	case "pastel.supernode.MetricsAggregate.report_count":
		panic(fmt.Errorf("field report_count of message pastel.supernode.MetricsAggregate is not mutable"))
	default:
		if fd.IsExtension() {
			panic(fmt.Errorf("proto3 declared messages do not support extensions: pastel.supernode.MetricsAggregate"))
		}
		panic(fmt.Errorf("message pastel.supernode.MetricsAggregate does not contain field %s", fd.FullName()))
	}
}

// NewField returns a new value that is assignable to the field
// for the given descriptor. For scalars, this returns the default value.
// For lists, maps, and messages, this returns a new, empty, mutable value.
func (x *fastReflection_MetricsAggregate) NewField(fd protoreflect.FieldDescriptor) protoreflect.Value {
	switch fd.FullName() {
	case "pastel.supernode.MetricsAggregate.metrics":
		m := make(map[string]float64)
		return protoreflect.ValueOfMap(&_MetricsAggregate_1_map{m: &m})
	case "pastel.supernode.MetricsAggregate.report_count":
		return protoreflect.ValueOfUint64(uint64(0))
	case "pastel.supernode.MetricsAggregate.last_updated":
		m := new(timestamppb.Timestamp)
		return protoreflect.ValueOfMessage(m.ProtoReflect())
	default:
		if fd.IsExtension() {
			panic(fmt.Errorf("proto3 declared messages do not support extensions: pastel.supernode.MetricsAggregate"))
		}
		panic(fmt.Errorf("message pastel.supernode.MetricsAggregate does not contain field %s", fd.FullName()))
	}
}

// WhichOneof reports which field within the oneof is populated,
// returning nil if none are populated.
// It panics if the oneof descriptor does not belong to this message.
func (x *fastReflection_MetricsAggregate) WhichOneof(d protoreflect.OneofDescriptor) protoreflect.FieldDescriptor {
	switch d.FullName() {
	default:
		panic(fmt.Errorf("%s is not a oneof field in pastel.supernode.MetricsAggregate", d.FullName()))
	}
	panic("unreachable")
}

// GetUnknown retrieves the entire list of unknown fields.
// The caller may only mutate the contents of the RawFields
// if the mutated bytes are stored back into the message with SetUnknown.
func (x *fastReflection_MetricsAggregate) GetUnknown() protoreflect.RawFields {
	return x.unknownFields
}

// SetUnknown stores an entire list of unknown fields.
// The raw fields must be syntactically valid according to the wire format.
// An implementation may panic if this is not the case.
// Once stored, the caller must not mutate the content of the RawFields.
// An empty RawFields may be passed to clear the fields.
//
// SetUnknown is a mutating operation and unsafe for concurrent use.
func (x *fastReflection_MetricsAggregate) SetUnknown(fields protoreflect.RawFields) {
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
func (x *fastReflection_MetricsAggregate) IsValid() bool {
	return x != nil
}

// ProtoMethods returns optional fastReflectionFeature-path implementations of various operations.
// This method may return nil.
//
// The returned methods type is identical to
// "google.golang.org/protobuf/runtime/protoiface".Methods.
// Consult the protoiface package documentation for details.
func (x *fastReflection_MetricsAggregate) ProtoMethods() *protoiface.Methods {
	size := func(input protoiface.SizeInput) protoiface.SizeOutput {
		x := input.Message.Interface().(*MetricsAggregate)
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
		if len(x.Metrics) > 0 {
			SiZeMaP := func(k string, v float64) {
				mapEntrySize := 1 + len(k) + runtime.Sov(uint64(len(k))) + 1 + 8
				n += mapEntrySize + 1 + runtime.Sov(uint64(mapEntrySize))
			}
			if options.Deterministic {
				sortme := make([]string, 0, len(x.Metrics))
				for k := range x.Metrics {
					sortme = append(sortme, k)
				}
				sort.Strings(sortme)
				for _, k := range sortme {
					v := x.Metrics[k]
					SiZeMaP(k, v)
				}
			} else {
				for k, v := range x.Metrics {
					SiZeMaP(k, v)
				}
			}
		}
		if x.ReportCount != 0 {
			n += 1 + runtime.Sov(uint64(x.ReportCount))
		}
		if x.LastUpdated != nil {
			l = options.Size(x.LastUpdated)
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
		x := input.Message.Interface().(*MetricsAggregate)
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
		if x.LastUpdated != nil {
			encoded, err := options.Marshal(x.LastUpdated)
			if err != nil {
				return protoiface.MarshalOutput{
					NoUnkeyedLiterals: input.NoUnkeyedLiterals,
					Buf:               input.Buf,
				}, err
			}
			i -= len(encoded)
			copy(dAtA[i:], encoded)
			i = runtime.EncodeVarint(dAtA, i, uint64(len(encoded)))
			i--
			dAtA[i] = 0x1a
		}
		if x.ReportCount != 0 {
			i = runtime.EncodeVarint(dAtA, i, uint64(x.ReportCount))
			i--
			dAtA[i] = 0x10
		}
		if len(x.Metrics) > 0 {
			MaRsHaLmAp := func(k string, v float64) (protoiface.MarshalOutput, error) {
				baseI := i
				i -= 8
				binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(v))))
				i--
				dAtA[i] = 0x11
				i -= len(k)
				copy(dAtA[i:], k)
				i = runtime.EncodeVarint(dAtA, i, uint64(len(k)))
				i--
				dAtA[i] = 0xa
				i = runtime.EncodeVarint(dAtA, i, uint64(baseI-i))
				i--
				dAtA[i] = 0xa
				return protoiface.MarshalOutput{}, nil
			}
			if options.Deterministic {
				keysForMetrics := make([]string, 0, len(x.Metrics))
				for k := range x.Metrics {
					keysForMetrics = append(keysForMetrics, string(k))
				}
				sort.Slice(keysForMetrics, func(i, j int) bool {
					return keysForMetrics[i] < keysForMetrics[j]
				})
				for iNdEx := len(keysForMetrics) - 1; iNdEx >= 0; iNdEx-- {
					v := x.Metrics[string(keysForMetrics[iNdEx])]
					out, err := MaRsHaLmAp(keysForMetrics[iNdEx], v)
					if err != nil {
						return out, err
					}
				}
			} else {
				for k := range x.Metrics {
					v := x.Metrics[k]
					out, err := MaRsHaLmAp(k, v)
					if err != nil {
						return out, err
					}
				}
			}
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
		x := input.Message.Interface().(*MetricsAggregate)
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
				return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, fmt.Errorf("proto: MetricsAggregate: wiretype end group for non-group")
			}
			if fieldNum <= 0 {
				return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, fmt.Errorf("proto: MetricsAggregate: illegal tag %d (wire type %d)", fieldNum, wire)
			}
			switch fieldNum {
			case 1:
				if wireType != 2 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, fmt.Errorf("proto: wrong wireType = %d for field Metrics", wireType)
				}
				var msglen int
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrIntOverflow
					}
					if iNdEx >= l {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					msglen |= int(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				if msglen < 0 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
				}
				postIndex := iNdEx + msglen
				if postIndex < 0 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
				}
				if postIndex > l {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
				}
				if x.Metrics == nil {
					x.Metrics = make(map[string]float64)
				}
				var mapkey string
				var mapvalue float64
				for iNdEx < postIndex {
					entryPreIndex := iNdEx
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
					if fieldNum == 1 {
						var stringLenmapkey uint64
						for shift := uint(0); ; shift += 7 {
							if shift >= 64 {
								return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrIntOverflow
							}
							if iNdEx >= l {
								return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
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
							return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
						}
						postStringIndexmapkey := iNdEx + intStringLenmapkey
						if postStringIndexmapkey < 0 {
							return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
						}
						if postStringIndexmapkey > l {
							return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
						}
						mapkey = string(dAtA[iNdEx:postStringIndexmapkey])
						iNdEx = postStringIndexmapkey
					} else if fieldNum == 2 {
						var mapvaluetemp uint64
						if (iNdEx + 8) > l {
							return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
						}
						mapvaluetemp = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
						iNdEx += 8
						mapvalue = math.Float64frombits(mapvaluetemp)
					} else {
						iNdEx = entryPreIndex
						skippy, err := runtime.Skip(dAtA[iNdEx:])
						if err != nil {
							return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, err
						}
						if (skippy < 0) || (iNdEx+skippy) < 0 {
							return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
						}
						if (iNdEx + skippy) > postIndex {
							return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
						}
						iNdEx += skippy
					}
				}
				x.Metrics[mapkey] = mapvalue
				iNdEx = postIndex
			case 2:
				if wireType != 0 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, fmt.Errorf("proto: wrong wireType = %d for field ReportCount", wireType)
				}
				x.ReportCount = 0
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrIntOverflow
					}
					if iNdEx >= l {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					x.ReportCount |= uint64(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
			case 3:
				if wireType != 2 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, fmt.Errorf("proto: wrong wireType = %d for field LastUpdated", wireType)
				}
				var msglen int
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrIntOverflow
					}
					if iNdEx >= l {
						return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					msglen |= int(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				if msglen < 0 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
				}
				postIndex := iNdEx + msglen
				if postIndex < 0 {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, runtime.ErrInvalidLength
				}
				if postIndex > l {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, io.ErrUnexpectedEOF
				}
				if x.LastUpdated == nil {
					x.LastUpdated = &timestamppb.Timestamp{}
				}
				if err := options.Unmarshal(dAtA[iNdEx:postIndex], x.LastUpdated); err != nil {
					return protoiface.UnmarshalOutput{NoUnkeyedLiterals: input.NoUnkeyedLiterals, Flags: input.Flags}, err
				}
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
// source: pastel/supernode/metrics_aggregate.proto

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type MetricsAggregate struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Metrics     map[string]float64     `protobuf:"bytes,1,rep,name=metrics,proto3" json:"metrics,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"fixed64,2,opt,name=value,proto3"`
	ReportCount uint64                 `protobuf:"varint,2,opt,name=report_count,json=reportCount,proto3" json:"report_count,omitempty"`
	LastUpdated *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=last_updated,json=lastUpdated,proto3" json:"last_updated,omitempty"`
}

func (x *MetricsAggregate) Reset() {
	*x = MetricsAggregate{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pastel_supernode_metrics_aggregate_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MetricsAggregate) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MetricsAggregate) ProtoMessage() {}

// Deprecated: Use MetricsAggregate.ProtoReflect.Descriptor instead.
func (*MetricsAggregate) Descriptor() ([]byte, []int) {
	return file_pastel_supernode_metrics_aggregate_proto_rawDescGZIP(), []int{0}
}

func (x *MetricsAggregate) GetMetrics() map[string]float64 {
	if x != nil {
		return x.Metrics
	}
	return nil
}

func (x *MetricsAggregate) GetReportCount() uint64 {
	if x != nil {
		return x.ReportCount
	}
	return 0
}

func (x *MetricsAggregate) GetLastUpdated() *timestamppb.Timestamp {
	if x != nil {
		return x.LastUpdated
	}
	return nil
}

var File_pastel_supernode_metrics_aggregate_proto protoreflect.FileDescriptor

var file_pastel_supernode_metrics_aggregate_proto_rawDesc = []byte{
	0x0a, 0x28, 0x70, 0x61, 0x73, 0x74, 0x65, 0x6c, 0x2f, 0x73, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f,
	0x64, 0x65, 0x2f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x5f, 0x61, 0x67, 0x67, 0x72, 0x65,
	0x67, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x10, 0x70, 0x61, 0x73, 0x74,
	0x65, 0x6c, 0x2e, 0x73, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f, 0x64, 0x65, 0x1a, 0x1f, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x14, 0x67,
	0x6f, 0x67, 0x6f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x67, 0x6f, 0x67, 0x6f, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0x85, 0x02, 0x0a, 0x10, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x41,
	0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x65, 0x12, 0x49, 0x0a, 0x07, 0x6d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2f, 0x2e, 0x70, 0x61, 0x73, 0x74,
	0x65, 0x6c, 0x2e, 0x73, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f, 0x64, 0x65, 0x2e, 0x4d, 0x65, 0x74,
	0x72, 0x69, 0x63, 0x73, 0x41, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x65, 0x2e, 0x4d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x07, 0x6d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x73, 0x12, 0x21, 0x0a, 0x0c, 0x72, 0x65, 0x70, 0x6f, 0x72, 0x74, 0x5f, 0x63, 0x6f,
	0x75, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0b, 0x72, 0x65, 0x70, 0x6f, 0x72,
	0x74, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x47, 0x0a, 0x0c, 0x6c, 0x61, 0x73, 0x74, 0x5f, 0x75,
	0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x42, 0x08, 0xc8, 0xde, 0x1f, 0x00, 0x90, 0xdf,
	0x1f, 0x01, 0x52, 0x0b, 0x6c, 0x61, 0x73, 0x74, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x1a,
	0x3a, 0x0a, 0x0c, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12,
	0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65,
	0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x01,
	0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x42, 0xc4, 0x01, 0x0a, 0x14,
	0x63, 0x6f, 0x6d, 0x2e, 0x70, 0x61, 0x73, 0x74, 0x65, 0x6c, 0x2e, 0x73, 0x75, 0x70, 0x65, 0x72,
	0x6e, 0x6f, 0x64, 0x65, 0x42, 0x15, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x41, 0x67, 0x67,
	0x72, 0x65, 0x67, 0x61, 0x74, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x34, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x61, 0x73, 0x74, 0x65, 0x6c,
	0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x2f, 0x70, 0x61, 0x73, 0x74, 0x65, 0x6c, 0x2f, 0x61,
	0x70, 0x69, 0x2f, 0x70, 0x61, 0x73, 0x74, 0x65, 0x6c, 0x2f, 0x73, 0x75, 0x70, 0x65, 0x72, 0x6e,
	0x6f, 0x64, 0x65, 0xa2, 0x02, 0x03, 0x50, 0x53, 0x58, 0xaa, 0x02, 0x10, 0x50, 0x61, 0x73, 0x74,
	0x65, 0x6c, 0x2e, 0x53, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f, 0x64, 0x65, 0xca, 0x02, 0x10, 0x50,
	0x61, 0x73, 0x74, 0x65, 0x6c, 0x5c, 0x53, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f, 0x64, 0x65, 0xe2,
	0x02, 0x1c, 0x50, 0x61, 0x73, 0x74, 0x65, 0x6c, 0x5c, 0x53, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f,
	0x64, 0x65, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02,
	0x11, 0x50, 0x61, 0x73, 0x74, 0x65, 0x6c, 0x3a, 0x3a, 0x53, 0x75, 0x70, 0x65, 0x72, 0x6e, 0x6f,
	0x64, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pastel_supernode_metrics_aggregate_proto_rawDescOnce sync.Once
	file_pastel_supernode_metrics_aggregate_proto_rawDescData = file_pastel_supernode_metrics_aggregate_proto_rawDesc
)

func file_pastel_supernode_metrics_aggregate_proto_rawDescGZIP() []byte {
	file_pastel_supernode_metrics_aggregate_proto_rawDescOnce.Do(func() {
		file_pastel_supernode_metrics_aggregate_proto_rawDescData = protoimpl.X.CompressGZIP(file_pastel_supernode_metrics_aggregate_proto_rawDescData)
	})
	return file_pastel_supernode_metrics_aggregate_proto_rawDescData
}

var file_pastel_supernode_metrics_aggregate_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_pastel_supernode_metrics_aggregate_proto_goTypes = []interface{}{
	(*MetricsAggregate)(nil),      // 0: pastel.supernode.MetricsAggregate
	nil,                           // 1: pastel.supernode.MetricsAggregate.MetricsEntry
	(*timestamppb.Timestamp)(nil), // 2: google.protobuf.Timestamp
}
var file_pastel_supernode_metrics_aggregate_proto_depIdxs = []int32{
	1, // 0: pastel.supernode.MetricsAggregate.metrics:type_name -> pastel.supernode.MetricsAggregate.MetricsEntry
	2, // 1: pastel.supernode.MetricsAggregate.last_updated:type_name -> google.protobuf.Timestamp
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_pastel_supernode_metrics_aggregate_proto_init() }
func file_pastel_supernode_metrics_aggregate_proto_init() {
	if File_pastel_supernode_metrics_aggregate_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pastel_supernode_metrics_aggregate_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MetricsAggregate); i {
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
			RawDescriptor: file_pastel_supernode_metrics_aggregate_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_pastel_supernode_metrics_aggregate_proto_goTypes,
		DependencyIndexes: file_pastel_supernode_metrics_aggregate_proto_depIdxs,
		MessageInfos:      file_pastel_supernode_metrics_aggregate_proto_msgTypes,
	}.Build()
	File_pastel_supernode_metrics_aggregate_proto = out.File
	file_pastel_supernode_metrics_aggregate_proto_rawDesc = nil
	file_pastel_supernode_metrics_aggregate_proto_goTypes = nil
	file_pastel_supernode_metrics_aggregate_proto_depIdxs = nil
}
