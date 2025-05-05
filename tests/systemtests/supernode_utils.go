package system

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/LumeraProtocol/lumera/x/supernode/v1/types"
)

// GetSuperNodeResponse queries and returns a supernode response
func GetSuperNodeResponse(t *testing.T, cli *LumeradCli, validatorAddr string) *types.SuperNode {
	queryCmd := []string{
		"q", "supernode", "get-super-node",
		validatorAddr,
	}
	queryResp := cli.CustomQuery(queryCmd...)
	t.Logf("Raw Response: %s", queryResp)

	// First unmarshal into a map to handle string conversions
	var rawResponse map[string]interface{}
	err := json.Unmarshal([]byte(queryResp), &rawResponse)
	if err != nil {
		t.Fatal(err)
	}

	supernodeData := rawResponse["supernode"].(map[string]interface{})

	// Convert state enum and height in states
	states := supernodeData["states"].([]interface{})
	for _, state := range states {
		stateMap := state.(map[string]interface{})
		// Convert state enum
		stateStr := stateMap["state"].(string)
		if enumVal, ok := types.SuperNodeState_value[stateStr]; ok {
			stateMap["state"] = enumVal
		}
		// Convert height to number
		if heightStr, ok := stateMap["height"].(string); ok {
			height, err := strconv.ParseInt(heightStr, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			stateMap["height"] = height
		}
	}

	// Convert height in prev_ip_addresses
	if ipAddresses, ok := supernodeData["prev_ip_addresses"].([]interface{}); ok {
		for _, addr := range ipAddresses {
			addrMap := addr.(map[string]interface{})
			if heightStr, ok := addrMap["height"].(string); ok {
				height, err := strconv.ParseInt(heightStr, 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				addrMap["height"] = height
			}
		}
	}

	// Marshal back to JSON
	jsonBytes, err := json.Marshal(rawResponse)
	if err != nil {
		t.Fatal(err)
	}

	// Finally unmarshal into our response type
	var response types.QueryGetSuperNodeResponse
	err = json.Unmarshal(jsonBytes, &response)
	if err != nil {
		t.Fatal(err)
	}

	return response.Supernode
}
