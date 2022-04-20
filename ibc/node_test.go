package ibc_test

import (
	"strings"
	"testing"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/stretchr/testify/require"
)

func TestCondenseMoniker_MiddleDetail(t *testing.T) {
	start := strings.Repeat("a", stakingtypes.MaxMonikerLength)
	end := strings.Repeat("z", stakingtypes.MaxMonikerLength)

	// Two monikers that have the same start and end but only differ in the middle.
	// The different piece will be truncated, but the condensed moniker should still differ.
	m1 := start + "1" + end
	m2 := start + "2" + end

	require.NotEqual(t, ibc.CondenseMoniker(m1), ibc.CondenseMoniker(m2))

	require.LessOrEqual(t, len(ibc.CondenseMoniker(m1)), stakingtypes.MaxMonikerLength)
}

func TestCondenseMoniker_Short(t *testing.T) {
	const m = "my_moniker"
	require.Equal(t, m, ibc.CondenseMoniker(m))
}
