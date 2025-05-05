package keeper

import (
	"context"
	types2 "github.com/LumeraProtocol/lumera/x/action/v1/types"
	"strings"

	"cosmossdk.io/store/prefix"
	actionapi "github.com/LumeraProtocol/lumera/api/lumera/action"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// QueryActionByMetadata returns actions filtered by metadata field and value
func (k Keeper) QueryActionByMetadata(goCtx context.Context, req *types2.QueryActionByMetadataRequest) (*types2.QueryListActionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.ActionType == types2.ActionTypeUnspecified || req.MetadataQuery == "" {
		return nil, status.Error(codes.InvalidArgument, "action type and metadata query required")
	}

	metadataParts := strings.SplitN(req.MetadataQuery, "=", 2)
	if len(metadataParts) != 2 {
		return nil, status.Error(codes.InvalidArgument, "invalid metadata query format, expected 'field=value'")
	}
	metadataKey := metadataParts[0]
	metadataValue := metadataParts[1]

	ctx := sdk.UnwrapSDKContext(goCtx)

	store := k.storeService.OpenKVStore(ctx)
	storeAdapter := runtime.KVStoreAdapter(store)
	actionStore := prefix.NewStore(storeAdapter, []byte(ActionKeyPrefix))

	var actions []*types2.Action

	appendAction := func(act *actionapi.Action, price sdk.Coin) {
		actions = append(actions, &types2.Action{
			Creator:        act.Creator,
			ActionID:       act.ActionID,
			ActionType:     types2.ActionType(act.ActionType),
			Metadata:       act.Metadata,
			Price:          &price,
			ExpirationTime: act.ExpirationTime,
			State:          types2.ActionState(act.State),
			BlockHeight:    act.BlockHeight,
			SuperNodes:     act.SuperNodes,
		})
	}

	onResult := func(key, value []byte, accumulate bool) (bool, error) {
		var act actionapi.Action
		if err := k.cdc.Unmarshal(value, &act); err != nil {
			return false, err
		}

		if act.ActionType != actionapi.ActionType(req.ActionType) {
			return false, nil
		}

		price, err := sdk.ParseCoinNormalized(act.Price)
		if err != nil {
			k.Logger().Error("failed to parse price", "action_id", act.ActionID, "price", act.Price, "error", err)
			return false, err
		}

		switch req.ActionType {
		case types2.ActionTypeSense:
			var senseMetadata actionapi.SenseMetadata
			err := proto.Unmarshal(act.Metadata, &senseMetadata)
			if err != nil {
				k.Logger().Error("failed to unmarshal sense metadata", "action_id", act.ActionID, "error", err)
				return false, err
			}

			switch metadataKey {
			case "collection_id":
				if senseMetadata.CollectionId == metadataValue {
					if accumulate {
						appendAction(&act, price)
					}
					return true, nil
				}
			case "group_id":
				if senseMetadata.GroupId == metadataValue {
					if accumulate {
						appendAction(&act, price)
					}
					return true, nil
				}
			case "data_hash":
				if senseMetadata.DataHash == metadataValue {
					if accumulate {
						appendAction(&act, price)
					}
					return true, nil
				}
			}

		case types2.ActionTypeCascade:
			var cascadeMetadata actionapi.CascadeMetadata
			err := proto.Unmarshal(act.Metadata, &cascadeMetadata)
			if err != nil {
				k.Logger().Error("failed to unmarshal cascade metadata", "action_id", act.ActionID, "error", err)
				return false, err
			}

			switch metadataKey {
			case "file_name":
				if cascadeMetadata.FileName == metadataValue {
					if accumulate {
						appendAction(&act, price)
					}
					return true, nil
				}
			case "data_hash":
				if cascadeMetadata.DataHash == metadataValue {
					if accumulate {
						appendAction(&act, price)
					}
					return true, nil
				}
			}
		}

		return false, nil
	}

	pageRes, err := query.FilteredPaginate(actionStore, req.Pagination, onResult)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to paginate actions: %v", err)
	}

	return &types2.QueryListActionsResponse{
		Actions:    actions,
		Pagination: pageRes,
	}, nil
}
