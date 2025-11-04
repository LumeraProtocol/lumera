package legacyalias

import (
	"sync"

	gogoproto "github.com/cosmos/gogoproto/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Alias struct {
	Legacy    string
	Canonical string
	Factory   func() gogoproto.Message
}

var (
	aliasesMu sync.RWMutex
	aliases   = make(map[protoreflect.FullName]protoreflect.FullName)
)

// Register wires a legacy (pre-versioned) type name into the legacy gogoproto registry
// and remembers the canonical proto name for resolver remapping.
func Register(a Alias) {
	msg := a.Factory()
	gogoproto.RegisterType(msg, a.Legacy)

	if a.Canonical == "" {
		a.Canonical = gogoproto.MessageName(msg)
	}

	aliasesMu.Lock()
	aliases[protoreflect.FullName(a.Legacy)] = protoreflect.FullName(a.Canonical)
	aliasesMu.Unlock()
}

// Snapshot returns a copy of the currently registered legacy aliases.
func Snapshot() map[protoreflect.FullName]protoreflect.FullName {
	aliasesMu.RLock()
	defer aliasesMu.RUnlock()

	out := make(map[protoreflect.FullName]protoreflect.FullName, len(aliases))
	for alias, canonical := range aliases {
		out[alias] = canonical
	}
	return out
}
