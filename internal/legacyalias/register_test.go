package legacyalias_test

import (
	"testing"

	gogoproto "github.com/cosmos/gogoproto/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/LumeraProtocol/lumera/internal/legacyalias"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"
)

func TestRegisterStoresLegacyAlias(t *testing.T) {
	const (
		legacyName    = "lumera.test.MsgLegacyAlias"
		canonicalName = "lumera.action.v1.MsgUpdateParams"
	)

	legacyalias.Register(legacyalias.Alias{
		Legacy:    legacyName,
		Canonical: canonicalName,
		Factory: func() gogoproto.Message {
			return &actiontypes.MsgUpdateParams{}
		},
	})

	snapshot := legacyalias.Snapshot()
	if got := snapshot[protoreflect.FullName(legacyName)]; got != protoreflect.FullName(canonicalName) {
		t.Fatalf("alias mismatch: got %q, want %q", got, canonicalName)
	}
}
