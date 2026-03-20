package evmigration

import (
	protov2 "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	txsigning "cosmossdk.io/x/tx/signing"
)

func emptyMsgSigners(protov2.Message) ([][]byte, error) {
	return nil, nil
}

// ProvideCustomGetSigners registers evmigration messages as unsigned at the
// Cosmos tx layer. These messages authenticate both parties inside the message
// payload itself, so the SDK signer extraction must return an empty set.
func ProvideCustomGetSigners() []txsigning.CustomGetSigner {
	return []txsigning.CustomGetSigner{
		{
			MsgType: protoreflect.FullName("lumera.evmigration.MsgClaimLegacyAccount"),
			Fn:      emptyMsgSigners,
		},
		{
			MsgType: protoreflect.FullName("lumera.evmigration.MsgMigrateValidator"),
			Fn:      emptyMsgSigners,
		},
	}
}
