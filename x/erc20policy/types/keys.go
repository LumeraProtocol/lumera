package types

import "bytes"

// Registration policy mode constants.
const (
	// PolicyModeAll allows all IBC denoms to auto-register as ERC20.
	PolicyModeAll = "all"
	// PolicyModeAllowlist only allows governance-approved IBC denoms to auto-register.
	PolicyModeAllowlist = "allowlist"
	// PolicyModeNone disables all IBC denom auto-registration.
	PolicyModeNone = "none"
)

// KV store keys and prefixes under the erc20 store key for policy state.
var (
	PolicyModeKey  = []byte("lumera/erc20policy/mode")
	PolicyAllowPfx = []byte("lumera/erc20policy/allow/")

	// PolicyAllowBaseTracePfx stores provenance-bound base denom entries.
	// Key format: PolicyAllowBaseTracePfx + baseDenom + "\x00" + traceKey
	// where traceKey encodes the expected full IBC trace as null-separated
	// port/channel pairs: "port1/channel1\x00port2/channel2".
	PolicyAllowBaseTracePfx = []byte("lumera/erc20policy/allowbasetrace/")
)

// DefaultAllowedBaseDenomTraces are well-known token base denominations that
// are pre-populated as inert placeholders on genesis. Each entry has an empty
// trace, meaning it will never match a real IBC packet (all packets have at
// least one hop). Governance must bind real IBC channels via
// MsgSetRegistrationPolicy before these entries become active.
var DefaultAllowedBaseDenomTraces = []AllowedBaseDenomTrace{
	{BaseDenom: "uatom"}, // Cosmos Hub ATOM
	{BaseDenom: "uosmo"}, // Osmosis OSMO
	{BaseDenom: "uusdc"}, // Noble USDC (Circle)
	{BaseDenom: "inj"},   // Injective INJ
}

// EncodeTraceKey encodes a sequence of SourceHop into a deterministic byte key.
// Single-hop: "port1/channel1". Multi-hop: "port1/channel1\x00port2/channel2".
// Empty hops returns nil (used for placeholder entries).
func EncodeTraceKey(hops []*SourceHop) []byte {
	if len(hops) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for i, hop := range hops {
		if i > 0 {
			buf.WriteByte(0x00)
		}
		buf.WriteString(hop.PortId)
		buf.WriteByte('/')
		buf.WriteString(hop.ChannelId)
	}
	return buf.Bytes()
}

// DecodeTraceKey decodes a trace key back into SourceHop entries.
// Returns nil for a nil or empty key (placeholder entry).
func DecodeTraceKey(key []byte) []*SourceHop {
	if len(key) == 0 {
		return nil
	}

	parts := bytes.Split(key, []byte{0x00})
	hops := make([]*SourceHop, 0, len(parts))
	for _, part := range parts {
		idx := bytes.IndexByte(part, '/')
		if idx < 0 {
			continue // malformed entry, skip
		}
		hops = append(hops, &SourceHop{
			PortId:    string(part[:idx]),
			ChannelId: string(part[idx+1:]),
		})
	}
	return hops
}
