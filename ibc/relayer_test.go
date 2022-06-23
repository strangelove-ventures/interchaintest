package ibc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelOptsConfigured(t *testing.T) {
	// Test the default channel opts
	opts := DefaultChannelOpts()
	require.True(t, opts.IsFullyConfigured())

	// Test empty struct channel opts
	opts = CreateChannelOptions{}
	require.False(t, opts.IsFullyConfigured())

	// Test invalid Order type in channel opts
	opts = CreateChannelOptions{
		SourcePortName: "transfer",
		DestPortName:   "transfer",
		Order:          3,
		Version:        "123",
	}
	require.True(t, opts.IsFullyConfigured())
	require.Equal(t, ErrInvalidOrderType, opts.Order.Validate())

	// Test partial channel opts
	opts = CreateChannelOptions{
		SourcePortName: "",
		DestPortName:   "",
		Order:          0,
	}
	require.False(t, opts.IsFullyConfigured())
}
