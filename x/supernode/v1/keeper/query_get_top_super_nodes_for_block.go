package keeper

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"

	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
	"lukechampine.com/blake3"
)

const DefaultLimit = 25

// GetTopSuperNodesForBlock implements the logic to:
//   - Validate blockHeight
//   - Optionally limit the result to a certain number of supernodes
//   - Filter supernodes by original registration, block presence, and optional request state
//   - Sort by XOR distance
func (q queryServer) GetTopSuperNodesForBlock(
	goCtx context.Context,
	req *types.QueryGetTopSuperNodesForBlockRequest,
) (*types.QueryGetTopSuperNodesForBlockResponse, error) {

	if req == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "nil request")
	}

	// 0) Parse the state filter since auto cli doesn't support enum
	// Accept both full enum names (e.g., "SUPERNODE_STATE_ACTIVE") and
	// short names (e.g., "ACTIVE", case-insensitive).
	var superNodeStateFilter types.SuperNodeState
	normalized := strings.ToUpper(strings.TrimSpace(req.State))
	switch normalized {
	case "", "SUPERNODE_STATE_UNSPECIFIED", "UNSPECIFIED":
		superNodeStateFilter = types.SuperNodeStateUnspecified
	case "SUPERNODE_STATE_ACTIVE", "ACTIVE":
		superNodeStateFilter = types.SuperNodeStateActive
	case "SUPERNODE_STATE_DISABLED", "DISABLED":
		superNodeStateFilter = types.SuperNodeStateDisabled
	case "SUPERNODE_STATE_STOPPED", "STOPPED":
		superNodeStateFilter = types.SuperNodeStateStopped
	case "SUPERNODE_STATE_PENALIZED", "PENALIZED":
		superNodeStateFilter = types.SuperNodeStatePenalized
	case "SUPERNODE_STATE_POSTPONED", "POSTPONED":
		superNodeStateFilter = types.SuperNodeStatePostponed
	default:
		if v, ok := types.SuperNodeState_value[normalized]; ok {
			superNodeStateFilter = types.SuperNodeState(v)
		} else {
			superNodeStateFilter = types.SuperNodeStateUnspecified
		}
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
	allSns, err := q.k.GetAllSuperNodes(ctx)
	if err != nil {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrNotFound,
			"could not fetch supernodes: %v", err,
		)
	}

	// 4) Filter supernodes
	validSns := make([]types.SuperNode, 0)
	for _, sn := range allSns {
		// 4.1) Must have at least one state record
		if len(sn.States) == 0 {
			continue
		}

		// 4.2) Determine supernode's state at blockHeight
		stateAtBlock, ok := DetermineStateAtBlock(sn.States, blockHeight)
		if !ok {
			continue
		}

		// 4.3) State must not be Unspecified or POSTPONED unless explicitly requested
		if stateAtBlock == types.SuperNodeStateUnspecified {
			continue
		}
		if superNodeStateFilter == types.SuperNodeStateUnspecified && stateAtBlock == types.SuperNodeStatePostponed {
			continue
		}

		// 4.4) Must match requested state if specified
		if superNodeStateFilter != types.SuperNodeStateUnspecified && stateAtBlock != superNodeStateFilter {
			continue
		}

		// This node qualifies for distance calc
		validSns = append(validSns, sn)
	}

	// 5) Compute XOR distances and rank
	blockHash, err := q.k.GetBlockHashForHeight(ctx, blockHeight)
	if err != nil {
		return nil, errorsmod.Wrapf(
			sdkerrors.ErrInvalidRequest,
			"could not retrieve block hash for height %d: %v", blockHeight, err,
		)
	}

	// 6) Rank supernodes by distance
	topNodes := q.k.RankSuperNodesByDistance(blockHash, validSns, int(limit))

	// 7) Build the response
	topPointers := make([]*types.SuperNode, len(topNodes))
	for i := range topNodes {
		topPointers[i] = &topNodes[i]
	}
	return &types.QueryGetTopSuperNodesForBlockResponse{
		Supernodes: topPointers,
	}, nil
}

// RankSuperNodesByDistance calculates XOR distance for each supernode to the given block hash,
// sorts them in ascending order of distance, and returns up to topN supernodes.
func (k Keeper) RankSuperNodesByDistance(
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
		if dist, ok := k.calcDistance(blockHash, sn); ok {
			distances = append(distances, supernodeDistance{sn, dist})
		}
	}

	sort.SliceStable(distances, func(i, j int) bool {
		cmp := distances[i].distance.Cmp(distances[j].distance)
		if cmp != 0 {
			return cmp < 0
		}
		// Tie-breaker to ensure deterministic order
		return distances[i].sn.ValidatorAddress < distances[j].sn.ValidatorAddress
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

// DetermineStateAtBlock returns the supernode state at the given block height.
//
// Invariant: the state history is append-only and therefore sorted by Height
// (non-decreasing). This function relies on that invariant and uses binary
// search to find the last record with Height <= blockHeight.
//
// If multiple records share the same Height, the last record at that height
// wins ("last write wins") because we search for the first Height > blockHeight
// and step back one slot.
func DetermineStateAtBlock(states []*types.SuperNodeStateRecord, blockHeight int64) (types.SuperNodeState, bool) {
	if len(states) == 0 {
		return types.SuperNodeStateUnspecified, false
	}

	pos := sort.Search(len(states), func(i int) bool {
		return states[i].Height > blockHeight
	})
	idx := pos - 1
	if idx < 0 {
		return types.SuperNodeStateUnspecified, false
	}
	return states[idx].State, true
}

func (k Keeper) calcDistance(blockHash []byte, sn *types.SuperNode) (*big.Int, bool) {
	valHash, ok := hashValidatorAddress(sn.ValidatorAddress)
	if !ok {
		return nil, false
	}
	return xorDistance(blockHash, valHash), true
}

// hashValidatorAddress hashes a validator address (bech32) into a 32-byte BLAKE3 digest.
// Returns false if the address cannot be decoded.
func hashValidatorAddress(valAddr string) ([]byte, bool) {
	addrBytes, err := sdk.ValAddressFromBech32(valAddr)
	if err != nil {
		return nil, false
	}
	h := blake3.Sum256(addrBytes)
	return h[:], true
}

// xorDistance computes the XOR distance between two byte slices as a big.Int.
func xorDistance(a, b []byte) *big.Int {
	// XOR over the max length, treating missing bytes as zero
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	xorBytes := make([]byte, n)
	for i := 0; i < n; i++ {
		var av, bv byte
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		xorBytes[i] = av ^ bv
	}
	return new(big.Int).SetBytes(xorBytes)
}

func (k Keeper) GetBlockHashForHeight(ctx sdk.Context, height int64) ([]byte, error) {
	if height <= 0 {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "invalid height %d", height)
	}
	h := blake3.Sum256([]byte(fmt.Sprintf("%d", height)))
	return h[:], nil
}
