package protobridge_test

import (
	"testing"

	stdproto "github.com/golang/protobuf/proto"

	"github.com/LumeraProtocol/lumera/internal/protobridge"
)

func TestRegisterEnumRegistersValues(t *testing.T) {
	enumName := "lumera.test.EnumBridge"
	valueMap := map[string]int32{
		"ENUM_BRIDGE_ALPHA": 1,
		"ENUM_BRIDGE_BETA":  2,
	}

	protobridge.RegisterEnum(enumName, valueMap)

	got := stdproto.EnumValueMap(enumName)
	if got == nil {
		t.Fatalf("EnumValueMap returned nil for %s", enumName)
	}
	if got["ENUM_BRIDGE_BETA"] != 2 {
		t.Fatalf("unexpected enum value: got %d, want 2", got["ENUM_BRIDGE_BETA"])
	}

	// Ensure subsequent registrations do not panic and keep the map intact.
	protobridge.RegisterEnum(enumName, valueMap)
	got = stdproto.EnumValueMap(enumName)
	if got["ENUM_BRIDGE_ALPHA"] != 1 {
		t.Fatalf("unexpected enum value after re-register: got %d, want 1", got["ENUM_BRIDGE_ALPHA"])
	}
}
