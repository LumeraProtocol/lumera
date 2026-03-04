package text

import (
	"os"
	"strconv"
	"strings"
)

// ParseAppOptionBool coerces an interface{} value (typically from
// servertypes.AppOptions.Get) into a boolean. Nil, unrecognised types, and
// unparseable strings all evaluate to false.
func ParseAppOptionBool(raw interface{}) bool {
	switch value := raw.(type) {
	case nil:
		return false
	case bool:
		return value
	case string:
		parsed, err := strconv.ParseBool(value)
		return err == nil && parsed
	case int:
		return value != 0
	case int8:
		return value != 0
	case int16:
		return value != 0
	case int32:
		return value != 0
	case int64:
		return value != 0
	case uint:
		return value != 0
	case uint8:
		return value != 0
	case uint16:
		return value != 0
	case uint32:
		return value != 0
	case uint64:
		return value != 0
	default:
		return false
	}
}

// EnvBool reads the environment variable identified by key and returns its
// boolean value. Missing, blank, or unparseable values evaluate to false.
func EnvBool(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return parsed
}

// EnvOrDefault returns the value of the environment variable identified by key,
// or fallback if the variable is empty or unset.
func EnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
