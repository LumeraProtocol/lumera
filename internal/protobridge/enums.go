package protobridge

import (
	gogoproto "github.com/cosmos/gogoproto/proto"
	stdproto "github.com/golang/protobuf/proto"
)

// RegisterEnum ensures the provided enum name is known to both the gogoproto
// and google.golang.org/protobuf v1 runtimes. The grpc-gateway code we rely on
// still queries the stdproto registry, while the gogo-generated modules expect
// the gogoproto registry to contain the same data. Calling this twice for the
// same enum is safe: the helpers bail out once it is registered.
func RegisterEnum(enumName string, valueMap map[string]int32) {
	if enumName == "" || len(valueMap) == 0 {
		return
	}

	if gogoproto.EnumValueMap(enumName) == nil {
		gogoproto.RegisterEnum(enumName, valueMapToNameMap(valueMap), valueMap)
	}

	if stdproto.EnumValueMap(enumName) != nil {
		return
	}

	stdproto.RegisterEnum(enumName, valueMapToNameMap(valueMap), valueMap)
}

func valueMapToNameMap(values map[string]int32) map[int32]string {
	nameMap := make(map[int32]string, len(values))
	for name, value := range values {
		nameMap[value] = name
	}
	return nameMap
}
