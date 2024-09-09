package ibc

import (
	"testing"

	chantypes "github.com/cosmos/ibc-go/v9/modules/core/04-channel/types"
	"github.com/stretchr/testify/require"
)

func TestChannelOptsConfigured(t *testing.T) {
	// Test the default channel opts
	opts := DefaultChannelOpts()
	require.NoError(t, opts.Validate())

	// Test empty struct channel opts
	opts = CreateChannelOptions{}
	require.Error(t, opts.Validate())

	// Test invalid Order type in channel opts
	opts = CreateChannelOptions{
		SourcePortName: "transfer",
		DestPortName:   "transfer",
		Order:          3,
		Version:        "123",
	}
	require.Error(t, opts.Validate())
	require.Equal(t, chantypes.ErrInvalidChannelOrdering, opts.Order.Validate())

	// Test partial channel opts
	opts = CreateChannelOptions{
		SourcePortName: "",
		DestPortName:   "",
		Order:          0,
	}
	require.Error(t, opts.Validate())
}

func TestClientOptsConfigured(t *testing.T) {
	// Test the default client opts
	opts := DefaultClientOpts()
	require.NoError(t, opts.Validate())

	// Test empty struct client opts
	opts = CreateClientOptions{}
	require.NoError(t, opts.Validate())

	// Test partial client opts
	opts = CreateClientOptions{
		MaxClockDrift: "5m",
	}
	require.NoError(t, opts.Validate())

	// Test invalid MaxClockDrift
	opts = CreateClientOptions{
		MaxClockDrift: "invalid duration",
	}
	require.Error(t, opts.Validate())

	// Test invalid TrustingPeriod
	opts = CreateClientOptions{
		TrustingPeriod: "invalid duration",
	}
	require.Error(t, opts.Validate())
}
