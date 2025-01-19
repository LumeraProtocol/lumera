package keeper

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/big"
	"sort"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/LumeraProtocol/lumera/x/supernode/types"
)

const DefaultLimit = 25

// GetTopSuperNodesForBlock implements the logic to:
//   - Validate blockHeight
//   - Optionally limit the result to a certain number of supernodes
//   - Filter supernodes by original registration, block presence, and optional request state
//   - Sort by XOR distance
func (k Keeper) GetTopSuperNodesForBlock(
	goCtx context.Context,
	req *types.QueryGetTopSuperNodesForBlockRequest,
) (*types.QueryGetTopSuperNodesForBlockResponse, error) {

	if req == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "nil request")
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

	fmt.Println("allSNs", allSns)

	// 4) Filter supernodes
	validSns := make([]types.SuperNode, 0)
	for _, sn := range allSns {
		// 4.1) Must have at least one state record
		if len(sn.States) == 0 {
			continue
		}
		// Must be originally registered as Active at a block <= blockHeight
		if sn.States[0].Height > blockHeight {
			continue
		}
		if sn.States[0].State != types.SuperNodeStateActive {
			continue
		}

		// 4.2) Determine supernode's state at blockHeight
		stateAtBlock, ok := DetermineStateAtBlock(sn.States, blockHeight)
		if !ok {
			continue
		}

		// 4.3) State must not be Unspecified
		if stateAtBlock == types.SuperNodeStateUnspecified {
			continue
		}

		// 5) If request.State != Unspecified, we must match
		if req.State != types.SuperNodeStateUnspecified && stateAtBlock != req.State {
			continue
		}

		// This node qualifies for distance calc
		validSns = append(validSns, sn)
	}

	// 6) Compute XOR distances
	blockHash, err := k.GetBlockHashForHeight(ctx, blockHeight)
	if err != nil {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"could not retrieve block hash for height %d: %v", blockHeight, err,
		)
	}

	// 7) Rank supernodes by distance
	topNodes := k.rankSuperNodesByDistance(blockHash, validSns, int(limit))

	// 8) Build the response
	topPointers := make([]*types.SuperNode, len(topNodes))
	for i := range topNodes {
		topPointers[i] = &topNodes[i]
	}
	return &types.QueryGetTopSuperNodesForBlockResponse{
		Supernodes: topPointers,
	}, nil
}

// rankSuperNodesByDistance calculates XOR distance for each supernode to the given block hash,
// sorts them in ascending order of distance, and returns up to topN supernodes.
func (k Keeper) rankSuperNodesByDistance(
	blockHash []byte,
	supernodes []types.SuperNode,
	topN int,
) []types.SuperNode {

	type supernodeDistance struct {
		sn       *types.SuperNode
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

	result := make([]types.SuperNode, topN)
	for i := 0; i < topN; i++ {
		result[i] = *distances[i].sn
	}
	return result
}

// DetermineStateAtBlock sorts the records by ascending height, then picks
// the last record whose height <= blockHeight, if any.
func DetermineStateAtBlock(states []*types.SuperNodeStateRecord, blockHeight int64) (types.SuperNodeState, bool) {
	if len(states) == 0 {
		return types.SuperNodeStateUnspecified, false
	}
	// Defensive: sort ascending
	sort.Slice(states, func(i, j int) bool {
		return states[i].Height < states[j].Height
	})

	foundState := types.SuperNodeStateUnspecified
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

func (k Keeper) calcDistance(blockHash []byte, sn *types.SuperNode) *big.Int {
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
