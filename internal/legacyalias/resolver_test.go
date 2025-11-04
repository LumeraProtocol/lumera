package legacyalias_test

import (
	"testing"

	gogoproto "github.com/cosmos/gogoproto/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/LumeraProtocol/lumera/internal/legacyalias"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

func TestResolverHandlesLegacyAndCanonicalNames(t *testing.T) {
	const (
		legacyName    = "lumera.test.MsgResolverLegacy"
		canonicalName = "lumera.action.v1.MsgUpdateParams"
	)

	legacyalias.Register(legacyalias.Alias{
		Legacy:    legacyName,
		Canonical: canonicalName,
		Factory: func() gogoproto.Message {
			return &actiontypes.MsgUpdateParams{}
		},
	})

	files, err := gogoproto.MergedRegistry()
	if err != nil {
		t.Fatalf("MergedRegistry: %v", err)
	}

	resolver := legacyalias.WrapResolver(protoregistry.GlobalTypes, files)

	mt, err := resolver.(protoregistry.MessageTypeResolver).FindMessageByName(protoreflect.FullName(legacyName))
	if err != nil {
		t.Fatalf("FindMessageByName legacy: %v", err)
	}
	if got := mt.Descriptor().FullName(); got != protoreflect.FullName(canonicalName) {
		t.Fatalf("descriptor mismatch for legacy lookup: got %q, want %q", got, canonicalName)
	}

	mt, err = resolver.(protoregistry.MessageTypeResolver).FindMessageByName(protoreflect.FullName(canonicalName))
	if err != nil {
		t.Fatalf("FindMessageByName canonical: %v", err)
	}
	if got := mt.Descriptor().FullName(); got != protoreflect.FullName(canonicalName) {
		t.Fatalf("descriptor mismatch for canonical lookup: got %q, want %q", got, canonicalName)
	}

	mt, err = resolver.(protoregistry.MessageTypeResolver).FindMessageByURL("/" + legacyName)
	if err != nil {
		t.Fatalf("FindMessageByURL legacy: %v", err)
	}
	if got := mt.Descriptor().FullName(); got != protoreflect.FullName(canonicalName) {
		t.Fatalf("descriptor mismatch for URL lookup: got %q, want %q", got, canonicalName)
	}
}
