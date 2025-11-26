package types

func init() {
	// Extend generated enum maps with the Postponed state without regenerating protobufs.
	SuperNodeState_name[int32(SuperNodeStatePostponed)] = "SuperNodeStatePostponed"
	SuperNodeState_value["SuperNodeStatePostponed"] = int32(SuperNodeStatePostponed)
}

const (
	// SuperNodeStatePostponed represents a temporary pause due to failed compliance checks.
	SuperNodeStatePostponed SuperNodeState = 5
)
