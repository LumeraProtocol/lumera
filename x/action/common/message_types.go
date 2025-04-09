package common

// MessageType enum for different action message contexts
type MessageType int

const (
	MsgRequestAction MessageType = iota
	MsgFinalizeAction
	MsgApproveAction
)

func (messageType MessageType) String() string {
	switch messageType {
	case MsgRequestAction:
		return "MsgRequestAction"
	case MsgFinalizeAction:
		return "MsgFinalizeAction"
	case MsgApproveAction:
		return "MsgApproveAction"
	default:
		return "UnknownMessageType"
	}
}
