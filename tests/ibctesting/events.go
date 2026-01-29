package ibctesting

import (
	"encoding/hex"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"

	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
)

// ParseAckFromEventsV2 parses events emitted from a MsgRecvPacket and returns the
// acknowledgement.
func ParseAckFromEventsV2(events []abci.Event) ([]byte, error) {
	for _, ev := range events {
		if ev.Type == channeltypes.EventTypeWriteAck {
			for _, attr := range ev.Attributes {
				if attr.Key == channeltypesv2.AttributeKeyEncodedAckHex {
					bz, err := hex.DecodeString(attr.Value)
					if err != nil {
						panic(err)
					}
					return bz, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("acknowledgement event attribute not found")
}

func ParsePortIDFromEvents(events []abci.Event) (string, error) {
	for _, ev := range events {
		if ev.Type == channeltypes.EventTypeChannelOpenInit || ev.Type == channeltypes.EventTypeChannelOpenTry {
			for _, attr := range ev.Attributes {
				if attr.Key == channeltypes.AttributeKeyPortID {
					return attr.Value, nil
				}
			}
		}
	}
	return "", fmt.Errorf("port id event attribute not found")
}

func ParseChannelVersionFromEvents(events []abci.Event) (string, error) {
	for _, ev := range events {
		if ev.Type == channeltypes.EventTypeChannelOpenInit || ev.Type == channeltypes.EventTypeChannelOpenTry {
			for _, attr := range ev.Attributes {
				if attr.Key == channeltypes.AttributeKeyVersion {
					return attr.Value, nil
				}
			}
		}
	}
	return "", fmt.Errorf("version event attribute not found")
}
