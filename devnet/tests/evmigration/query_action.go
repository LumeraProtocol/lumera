package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// FullAction holds all on-chain action fields for validation.
type FullAction struct {
	ActionID    string   `json:"actionID"`
	Creator     string   `json:"creator"`
	ActionType  string   `json:"actionType"`
	Metadata    string   `json:"metadata"`
	Price       string   `json:"price"`
	State       string   `json:"state"`
	SuperNodes  []string `json:"superNodes"`
	BlockHeight string   `json:"blockHeight"`
	Expiration  string   `json:"expirationTime"`
	RqIdsIc     uint64   `json:"rqIdsIc,string"`
	RqIdsMax    uint64   `json:"rqIdsMax,string"`
}

// queryActionsByCreator returns the action IDs owned by the given creator address.
func queryActionsByCreator(creator string) ([]string, error) {
	out, err := run("query", "action", "list-actions-by-creator", creator)
	if err != nil {
		return nil, fmt.Errorf("query list-actions-by-creator %s: %s\n%w", creator, truncate(out, 300), err)
	}

	var resp struct {
		Actions []struct {
			ActionID string `json:"actionID"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse list-actions-by-creator %s: %s\n%w", creator, truncate(out, 300), err)
	}

	ids := make([]string, 0, len(resp.Actions))
	for _, a := range resp.Actions {
		ids = append(ids, a.ActionID)
	}
	return ids, nil
}

func queryActionsBySupernode(supernode string) ([]string, error) {
	out, err := run("query", "action", "list-actions-by-supernode", supernode)
	if err != nil {
		return nil, fmt.Errorf("query list-actions-by-supernode %s: %s\n%w", supernode, truncate(out, 300), err)
	}

	var resp struct {
		Actions []struct {
			ActionID string `json:"actionID"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse list-actions-by-supernode %s: %s\n%w", supernode, truncate(out, 300), err)
	}

	ids := make([]string, 0, len(resp.Actions))
	for _, a := range resp.Actions {
		ids = append(ids, a.ActionID)
	}
	return ids, nil
}

// queryActionCreator returns the creator field of a single action by ID.
func queryActionCreator(actionID string) (string, error) {
	out, err := run("query", "action", "action", actionID)
	if err != nil {
		return "", fmt.Errorf("query action %s: %s\n%w", actionID, truncate(out, 300), err)
	}

	var resp struct {
		Action struct {
			Creator string `json:"creator"`
		} `json:"action"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return "", fmt.Errorf("parse action %s: %s\n%w", actionID, truncate(out, 300), err)
	}
	return resp.Action.Creator, nil
}

func queryActionSupernodes(actionID string) ([]string, error) {
	out, err := run("query", "action", "action", actionID)
	if err != nil {
		return nil, fmt.Errorf("query action %s: %s\n%w", actionID, truncate(out, 300), err)
	}

	var resp struct {
		Action struct {
			SuperNodes []string `json:"superNodes"`
		} `json:"action"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse action %s supernodes: %s\n%w", actionID, truncate(out, 300), err)
	}
	return resp.Action.SuperNodes, nil
}

// queryFullAction returns all fields of an on-chain action.
func queryFullAction(actionID string) (*FullAction, error) {
	out, err := run("query", "action", "action", actionID)
	if err != nil {
		return nil, fmt.Errorf("query action %s: %s\n%w", actionID, truncate(out, 300), err)
	}
	var resp struct {
		Action FullAction `json:"action"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("parse action %s: %s\n%w", actionID, truncate(out, 300), err)
	}
	return &resp.Action, nil
}

// extractActionIDFromTxOutput parses the action_id from a request-action tx event log.
func extractActionIDFromTxOutput(txOutput string) string {
	// Try JSON log first (events array).
	var resp struct {
		Events []struct {
			Type       string `json:"type"`
			Attributes []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"attributes"`
		} `json:"events"`
	}
	if err := json.Unmarshal([]byte(txOutput), &resp); err == nil {
		for _, ev := range resp.Events {
			if ev.Type == "action_registered" || ev.Type == "lumera.action.v1.EventActionRegistered" {
				for _, attr := range ev.Attributes {
					if attr.Key == "action_id" || attr.Key == "actionID" {
						return strings.Trim(attr.Value, "\"")
					}
				}
			}
		}
	}

	// Fallback: search for action_id in the raw output.
	re := regexp.MustCompile(`"action_id"\s*:\s*"?(\d+)"?`)
	if m := re.FindStringSubmatch(txOutput); len(m) > 1 {
		return m[1]
	}
	return ""
}
