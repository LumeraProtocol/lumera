package keeper

import (
	"context"
	"strings"

	"github.com/LumeraProtocol/lumera/x/action/v1/types"
	actiontypes "github.com/LumeraProtocol/lumera/x/action/v1/types"

	"cosmossdk.io/store/prefix"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	gogoproto "github.com/gogo/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// QueryActionByMetadata returns actions filtered by metadata field and value
func (q queryServer) QueryActionByMetadata(goCtx context.Context, req *types.QueryActionByMetadataRequest) (*types.QueryActionByMetadataResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	if req.ActionType == types.ActionTypeUnspecified || req.MetadataQuery == "" {
		return nil, status.Error(codes.InvalidArgument, "action type and metadata query required")
	}

	metadataParts := strings.SplitN(req.MetadataQuery, "=", 2)
	if len(metadataParts) != 2 {
		return nil, status.Error(codes.InvalidArgument, "invalid metadata query format, expected 'field=value'")
	}
	metadataKey := metadataParts[0]
	metadataValue := metadataParts[1]

	ctx := sdk.UnwrapSDKContext(goCtx)

	store := q.k.storeService.OpenKVStore(ctx)
	storeAdapter := runtime.KVStoreAdapter(store)
	actionStore := prefix.NewStore(storeAdapter, []byte(ActionKeyPrefix))

	var actions []*types.Action

	appendAction := func(act *actiontypes.Action, price string) {
		actions = append(actions, &types.Action{
			Creator:        act.Creator,
			ActionID:       act.ActionID,
			ActionType:     types.ActionType(act.ActionType),
			Metadata:       act.Metadata,
			Price:          price,
			ExpirationTime: act.ExpirationTime,
			State:          types.ActionState(act.State),
			BlockHeight:    act.BlockHeight,
			SuperNodes:     act.SuperNodes,
		})
	}

	onResult := func(key, value []byte, accumulate bool) (bool, error) {
		var act actiontypes.Action
		if err := q.k.cdc.Unmarshal(value, &act); err != nil {
			return false, err
		}

		if act.ActionType != actiontypes.ActionType(req.ActionType) {
			return false, nil
		}

		price := act.Price

		switch req.ActionType {
		case types.ActionTypeSense:
			var senseMetadata actiontypes.SenseMetadata
			err := gogoproto.Unmarshal(act.Metadata, &senseMetadata)
			if err != nil {
				q.k.Logger().Error("failed to unmarshal sense metadata", "action_id", act.ActionID, "error", err)
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

		case types.ActionTypeCascade:
			var cascadeMetadata actiontypes.CascadeMetadata
			err := gogoproto.Unmarshal(act.Metadata, &cascadeMetadata)
			if err != nil {
				q.k.Logger().Error("failed to unmarshal cascade metadata", "action_id", act.ActionID, "error", err)
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

	return &types.QueryActionByMetadataResponse{
		Actions:    actions,
		Pagination: pageRes,
	}, nil
}
