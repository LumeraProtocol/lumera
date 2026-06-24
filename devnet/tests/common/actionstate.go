package common

import (
	"fmt"
	"strings"
)

// ActionState is a target CASCADE action lifecycle state the generator can aim
// to produce.
type ActionState string

const (
	ActionStatePending  ActionState = "pending"
	ActionStateDone     ActionState = "done"
	ActionStateApproved ActionState = "approved"
)

// knownActionStates is the set of states ParseActionStates accepts.
var knownActionStates = map[string]ActionState{
	string(ActionStatePending):  ActionStatePending,
	string(ActionStateDone):     ActionStateDone,
	string(ActionStateApproved): ActionStateApproved,
}

// ParseActionStates parses the -action-states flag (a comma-separated list)
// into a validated, de-duplicated slice preserving first-seen order. Values are
// trimmed and matched case-insensitively. It errors on unknown or empty input.
func ParseActionStates(s string) ([]ActionState, error) {
	var out []ActionState
	seen := make(map[ActionState]bool)
	for raw := range strings.SplitSeq(s, ",") {
		token := strings.ToLower(strings.TrimSpace(raw))
		if token == "" {
			continue
		}
		state, ok := knownActionStates[token]
		if !ok {
			return nil, fmt.Errorf("unknown action state %q (valid: pending, done, approved)", strings.TrimSpace(raw))
		}
		if seen[state] {
			continue
		}
		seen[state] = true
		out = append(out, state)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no action states specified")
	}
	return out, nil
}
