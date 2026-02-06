package keeper

import (
	"fmt"

	"github.com/LumeraProtocol/lumera/x/audit/v1/types"
)

type epochInfo struct {
	EpochID     uint64
	StartHeight int64
	EndHeight   int64
}

func deriveEpochAtHeight(height int64, params types.Params) (epochInfo, error) {
	// Epoch math is consensus-critical. Epoch boundaries must be derived deterministically
	// from params (epoch_zero_height, epoch_length_blocks) and height, with no state reads.
	params = params.WithDefaults()
	if params.EpochLengthBlocks == 0 {
		return epochInfo{}, fmt.Errorf("epoch_length_blocks must be > 0")
	}
	if params.EpochZeroHeight == 0 {
		return epochInfo{}, fmt.Errorf("epoch_zero_height must be > 0")
	}

	epochZero := int64(params.EpochZeroHeight)
	epochLen := int64(params.EpochLengthBlocks)

	// If height is before epoch_zero_height (should not happen on a normal chain), clamp to epoch 0.
	var epochID uint64
	var start int64
	if height < epochZero {
		epochID = 0
		start = epochZero
	} else {
		rel := height - epochZero
		epochID = uint64(rel / epochLen)
		start = epochZero + int64(epochID)*epochLen
	}

	end := start + epochLen - 1
	return epochInfo{EpochID: epochID, StartHeight: start, EndHeight: end}, nil
}

func deriveEpochByID(epochID uint64, params types.Params) (epochInfo, error) {
	// Used by queries and validation to reconstruct boundaries from an epoch ID.
	params = params.WithDefaults()
	if params.EpochLengthBlocks == 0 {
		return epochInfo{}, fmt.Errorf("epoch_length_blocks must be > 0")
	}
	if params.EpochZeroHeight == 0 {
		return epochInfo{}, fmt.Errorf("epoch_zero_height must be > 0")
	}

	epochZero := int64(params.EpochZeroHeight)
	epochLen := int64(params.EpochLengthBlocks)

	start := epochZero + int64(epochID)*epochLen
	end := start + epochLen - 1
	return epochInfo{EpochID: epochID, StartHeight: start, EndHeight: end}, nil
}
