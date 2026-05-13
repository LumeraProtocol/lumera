package types

import (
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgUpdateParams{},
		&MsgSubmitEpochReport{},
		&MsgSubmitEvidence{},
		&MsgSubmitStorageRecheckEvidence{},
		&MsgClaimHealComplete{},
		&MsgSubmitHealVerification{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &Msg_serviceDesc)
}
