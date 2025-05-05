package keeper

import (
	"context"
	"crypto/sha256"
	"fmt"
	types2 "github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"math/big"
	"sort"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const DefaultLimit = 25

// GetTopSuperNodesForBlock implements the logic to:
//   - Validate blockHeight
//   - Optionally limit the result to a certain number of supernodes
//   - Filter supernodes by original registration, block presence, and optional request state
//   - Sort by XOR distance
func (k Keeper) GetTopSuperNodesForBlock(
	goCtx context.Context,
	req *types2.QueryGetTopSuperNodesForBlockRequest,
) (*types2.QueryGetTopSuperNodesForBlockResponse, error) {

	if req == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "nil request")
	}

	// 0) Parse the state filter since auto cli doesn't support enum
	var superNodeStateFilter types2.SuperNodeState
	stateValue, ok := types2.SuperNodeState_value[req.State]
	if !ok {
		superNodeStateFilter = types2.SuperNodeStateUnspecified
	} else {
		superNodeStateFilter = types2.SuperNodeState(stateValue)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	blockHeight := int64(req.BlockHeight)

	// 1) Validate block height
	if blockHeight <= 0 {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"invalid block height %d", blockHeight,
		)
	}

	// 2) Determine the limit (default 25 if not provided or <= 0)
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}

	// 3) Retrieve all supernodes from the store
	allSns, err := k.GetAllSuperNodes(ctx)
	if err != nil {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrNotFound,
			"could not fetch supernodes: %v", err,
		)
	}

	// 4) Filter supernodes
	validSns := make([]types2.SuperNode, 0)
	for _, sn := range allSns {
		// 4.1) Must have at least one state record
		if len(sn.States) == 0 {
			continue
		}

		// 4.2) Must have a state record at or before the requested block height
		if sn.States[0].Height > blockHeight {
			continue
		}

		// 4.3) Determine supernode's state at blockHeight
		stateAtBlock, ok := DetermineStateAtBlock(sn.States, blockHeight)
		if !ok {
			continue
		}

		// 4.4) State must not be Unspecified
		if stateAtBlock == types2.SuperNodeStateUnspecified {
			continue
		}

		// 4.5) Must match requested state if specified
		if superNodeStateFilter != types2.SuperNodeStateUnspecified && stateAtBlock != superNodeStateFilter {
			continue
		}

		// This node qualifies for distance calc
		validSns = append(validSns, sn)
	}

	// 5) Compute XOR distances and rank
	blockHash, err := k.GetBlockHashForHeight(ctx, blockHeight)
	if err != nil {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"could not retrieve block hash for height %d: %v", blockHeight, err,
		)
	}

	// 6) Rank supernodes by distance
	topNodes := k.rankSuperNodesByDistance(blockHash, validSns, int(limit))

	// 7) Build the response
	topPointers := make([]*types2.SuperNode, len(topNodes))
	for i := range topNodes {
		topPointers[i] = &topNodes[i]
	}
	return &types2.QueryGetTopSuperNodesForBlockResponse{
		Supernodes: topPointers,
	}, nil
}

// rankSuperNodesByDistance calculates XOR distance for each supernode to the given block hash,
// sorts them in ascending order of distance, and returns up to topN supernodes.
func (k Keeper) rankSuperNodesByDistance(
	blockHash []byte,
	supernodes []types2.SuperNode,
	topN int,
) []types2.SuperNode {

	type supernodeDistance struct {
		sn       *types2.SuperNode
		distance *big.Int
	}

	distances := make([]supernodeDistance, 0, len(supernodes))
	for i := range supernodes {
		sn := &supernodes[i]
		dist := k.calcDistance(blockHash, sn)
		distances = append(distances, supernodeDistance{sn, dist})
	}

	sort.Slice(distances, func(i, j int) bool {
		return distances[i].distance.Cmp(distances[j].distance) < 0
	})

	if len(distances) < topN {
		topN = len(distances)
	}

	result := make([]types2.SuperNode, topN)
	for i := 0; i < topN; i++ {
		result[i] = *distances[i].sn
	}
	return result
}

// DetermineStateAtBlock sorts the records by ascending height, then picks
// the last record whose height <= blockHeight, if any.
func DetermineStateAtBlock(states []*types2.SuperNodeStateRecord, blockHeight int64) (types2.SuperNodeState, bool) {
	if len(states) == 0 {
		return types2.SuperNodeStateUnspecified, false
	}
	// Defensive: sort ascending
	sort.Slice(states, func(i, j int) bool {
		return states[i].Height < states[j].Height
	})

	foundState := types2.SuperNodeStateUnspecified
	found := false
	for _, sRecord := range states {
		if sRecord.Height <= blockHeight {
			foundState = sRecord.State
			found = true
		} else {
			break
		}
	}
	return foundState, found
}

func (k Keeper) calcDistance(blockHash []byte, sn *types2.SuperNode) *big.Int {
	valHash := hashValidatorAddress(sn.ValidatorAddress)
	return xorDistance(blockHash, valHash)
}

// hashValidatorAddress hashes a validator address (in bech32) into a 32-byte sha256 digest.
func hashValidatorAddress(valAddr string) []byte {
	addrBytes, _ := sdk.ValAddressFromBech32(valAddr)
	h := sha256.Sum256(addrBytes)
	return h[:]
}

// xorDistance computes the XOR distance between two byte slices as a big.Int.
func xorDistance(a, b []byte) *big.Int {
	xorBytes := make([]byte, len(a))
	for i := 0; i < len(a) && i < len(b); i++ {
		xorBytes[i] = a[i] ^ b[i]
	}
	return new(big.Int).SetBytes(xorBytes)
}

func (k Keeper) GetBlockHashForHeight(ctx sdk.Context, height int64) ([]byte, error) {
	if height <= 0 {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "invalid height %d", height)
	}
	h := sha256.Sum256([]byte(fmt.Sprintf("%d", height)))
	return h[:], nil
}
