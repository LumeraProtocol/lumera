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
	"github.com/pastelnetwork/pastel/x/supernode/types"
)

const DefaultTopN = 10

func (k Keeper) GetTopSuperNodesForBlock(goCtx context.Context, req *types.QueryGetTopSuperNodesForBlockRequest) (*types.QueryGetTopSuperNodesForBlockResponse, error) {
	if req == nil {
		return nil, errorsmod.Wrap(sdkerrors.ErrInvalidRequest, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(goCtx)

	blockHash, err := k.GetBlockHashForHeight(ctx, int64(req.BlockHeight))
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "could not retrieve block hash for height %d: %v", req.BlockHeight, err)

	}

	snList, err := k.GetAllSuperNodes(ctx, types.SuperNodeStateActive)
	if err != nil {
		return nil, errorsmod.Wrapf(sdkerrors.ErrNotFound, "could not retrieve block hash for height %d: %v", req.BlockHeight, err)
	}

	type supernodeDistance struct {
		sn       *types.SuperNode
		distance *big.Int
	}

	var distances []supernodeDistance

	for i := range snList {
		sn := &snList[i]
		valHash := hashValidatorAddress(sn.ValidatorAddress)
		dist := xorDistance(blockHash, valHash)
		distances = append(distances, supernodeDistance{sn, dist})
	}

	sort.Slice(distances, func(i, j int) bool {
		return distances[i].distance.Cmp(distances[j].distance) < 0
	})

	n := DefaultTopN
	if len(distances) < n {
		n = len(distances)
	}

	topSupernodes := make([]*types.SuperNode, n)
	for i := 0; i < n; i++ {
		topSupernodes[i] = distances[i].sn
	}

	return &types.QueryGetTopSuperNodesForBlockResponse{
		Supernodes: topSupernodes,
	}, nil
}

// GetBlockHashForHeight is a dummy implementation. In a real scenario, you might fetch from store or external indexer.
func (k Keeper) GetBlockHashForHeight(ctx sdk.Context, height int64) ([]byte, error) {
	if height <= 0 {
		return nil, errorsmod.Wrapf(sdkerrors.ErrInvalidRequest, "invalid height %d", height)
	}

	// Instead of string(height), do:
	h := sha256.Sum256([]byte(fmt.Sprintf("%d", height)))
	return h[:], nil
}

func hashValidatorAddress(valAddr string) []byte {
	addrBytes, _ := sdk.ValAddressFromBech32(valAddr)
	h := sha256.Sum256(addrBytes)
	return h[:]
}

func xorDistance(a, b []byte) *big.Int {
	// XOR the two byte slices and return as big.Int for comparison
	xorBytes := make([]byte, len(a))
	for i := 0; i < len(a) && i < len(b); i++ {
		xorBytes[i] = a[i] ^ b[i]
	}
	return new(big.Int).SetBytes(xorBytes)
}
