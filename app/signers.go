package app

import (
	"encoding/hex"
	"fmt"
	"strings"

    "cosmossdk.io/depinject"
	"cosmossdk.io/core/address"
	"cosmossdk.io/x/tx/signing"
	"google.golang.org/protobuf/proto"
)

// Create a global variable for the signing options
var defaultSigningOptions *signing.Options

func init() {
    // Initialize with nil codecs - these will be replaced when actual codecs are available
    defaultSigningOptions = &signing.Options{
        AddressCodec:          nil,
        ValidatorAddressCodec: nil,
    }
    
    // Define custom signers
    defaultSigningOptions.DefineCustomGetSigners(
        "ethermint.evm.v1.MsgEthereumTx", 
        handleEthereumTxSigner,
    )
}

// handleEthereumTxSigner handles extracting signers from MsgEthereumTx
// Note: This function won't be directly called with your MsgEthereumTx,
// but instead with a protobuf message that has the same structure.
// We'll need to extract the 'from' field from it
func handleEthereumTxSigner(msg proto.Message) ([][]byte, error) {
	// Access the From field directly using protoreflect
	msgReflect := msg.ProtoReflect()

    fromField := msgReflect.Descriptor().Fields().ByName("from")
    
    if fromField == nil {
        return nil, fmt.Errorf("from field not found in message")
    }
    
    fromValue := msgReflect.Get(fromField).String()
    if fromValue == "" {
        return nil, fmt.Errorf("sender address not defined for message")
    }
    
    // Convert the hex address to bytes
    // Ethereum addresses might be prefixed with 0x
    fromAddress := strings.TrimPrefix(fromValue, "0x")
    addrBytes, err := hex.DecodeString(fromAddress)
    if err != nil {
        return nil, fmt.Errorf("failed to decode from address: %w", err)
    }
    
    return [][]byte{addrBytes}, nil
}

// Wrap in depinject constructor
func ProvideSigningOptions(
    in struct {
        depinject.In
        AddressCodec          address.Codec
        ValidatorAddressCodec address.Codec
    },
) []signing.CustomGetSigner {
    return []signing.CustomGetSigner{
        {
            MsgType: "ethermint.evm.v1.MsgEthereumTx",
            Fn: func(msg proto.Message) ([][]byte, error) {
                return handleEthereumTxSigner(msg)
            },
        },
    }
}
