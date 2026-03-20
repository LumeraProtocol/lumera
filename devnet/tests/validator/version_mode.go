package validator

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

const firstEVMVersion = "v1.12.0"

var (
	lumeraVersionOnce  sync.Once
	lumeraVersionCache string
	lumeraVersionErr   error
)

type lumeraVersionJSON struct {
	Version string `json:"version"`
}

func resolveLumeraBinaryVersion(bin string) (string, error) {
	lumeraVersionOnce.Do(func() {
		cmd := exec.Command(bin, "version", "--long", "--output", "json")
		out, err := cmd.Output()
		if err != nil {
			lumeraVersionErr = fmt.Errorf("query %s version: %w", bin, err)
			return
		}

		var parsed lumeraVersionJSON
		if err := json.Unmarshal(out, &parsed); err != nil {
			lumeraVersionErr = fmt.Errorf("parse %s version json: %w", bin, err)
			return
		}
		lumeraVersionCache = strings.TrimSpace(parsed.Version)
		if lumeraVersionCache == "" {
			lumeraVersionErr = fmt.Errorf("empty %s version in output", bin)
		}
	})

	if lumeraVersionErr != nil {
		return "", lumeraVersionErr
	}
	return lumeraVersionCache, nil
}
