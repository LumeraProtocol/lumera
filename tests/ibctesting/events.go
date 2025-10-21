package ibctesting

import (
	"encoding/hex"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"

	gogoproto "github.com/cosmos/gogoproto/proto"

	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
)

// TODO: Remove this once it's implemented in the `ibc-go`.
// https://github.com/cosmos/ibc-go/issues/8284
//
// ParsePacketsFromEventsV2 parses events emitted from a MsgRecvPacket and returns
// all the packets found.
// Returns an error if no packet is found.
func ParsePacketsFromEventsV2(eventType string, events []abci.Event) ([]channeltypesv2.Packet, error) {
	ferr := func(err error) ([]channeltypesv2.Packet, error) {
		return nil, fmt.Errorf("ParsePacketsFromEventsV2: %w", err)
	}
	var packets []channeltypesv2.Packet
	for _, ev := range events {
		if ev.Type == eventType {
			for _, attr := range ev.Attributes {
				switch attr.Key {
				case channeltypesv2.AttributeKeyEncodedPacketHex:
					data, err := hex.DecodeString(attr.Value)
					if err != nil {
						return ferr(err)
					}
					var packet channeltypesv2.Packet
					err = gogoproto.Unmarshal(data, &packet)
					if err != nil {
						return ferr(err)
					}
					packets = append(packets, packet)

				default:
					continue
				}
			}
		}
	}
	return packets, nil
}


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
