package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSNDistStateKeyDoesNotMutatePrefix(t *testing.T) {
	originalPrefix := append([]byte(nil), SNDistStatePrefix...)

	key := SNDistStateKey("validator-address")
	expected := append(append([]byte(nil), originalPrefix...), []byte("validator-address")...)

	require.Equal(t, expected, key)
	require.Equal(t, originalPrefix, SNDistStatePrefix)

	key[0] = 'x'
	require.Equal(t, originalPrefix, SNDistStatePrefix)
}
