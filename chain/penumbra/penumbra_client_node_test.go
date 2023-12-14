package penumbra

import (
	"math/big"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
)

// TestBigIntDecoding tests the decoding of big integers.
func TestBigIntDecoding(t *testing.T) {
	bigInt := math.NewInt(11235813)
	hi, lo := translateBigInt(bigInt)
	converted := translateHiAndLo(hi, lo)
	require.True(t, bigInt.Equal(converted))

	b := big.NewInt(0)
	b.SetString("18446744073709551620", 10) // use a number that is bigger than the maximum value an uint64 can hold
	bInt := math.NewIntFromBigInt(b)
	hi, lo = translateBigInt(bInt)
	converted = translateHiAndLo(hi, lo)
	require.True(t, converted.Equal(bInt))
}

// TestIbcTransferTimeout tests the function ibcTransferTimeouts
// in order to verify that it behaves correctly under different scenarios.
// Scenario 1: both timeout values equal zero - return default timeout values
// Scenario 2: options has nil timeout value - return default timeout values
// Scenario 3: both timeout values equal non-zero values - use specified timeout values
// Scenario 4: only nanoseconds equals non-zero value - use specified value for timestamp and zero for height
// Scenario 5: only height equals non-zero value - use specified value for height and zero for timestamp
func TestIbcTransferTimeout(t *testing.T) {
	defaultHeight, defaultTimestamp := defaultTransferTimeouts()
	zero := uint64(0)

	t.Run("both timeout values equal zero - return default timeout values", func(t *testing.T) {
		opts := ibc.TransferOptions{
			Timeout: &ibc.IBCTimeout{
				NanoSeconds: 0,
				Height:      0,
			},
		}

		height, timestamp := ibcTransferTimeouts(opts)
		require.Equal(t, defaultHeight, height)
		require.Equal(t, defaultTimestamp, timestamp)
	})

	t.Run("options has nil timeout value - return default timeout values", func(t *testing.T) {
		var opts ibc.TransferOptions

		height, timestamp := ibcTransferTimeouts(opts)
		require.Equal(t, defaultHeight, height)
		require.Equal(t, defaultTimestamp, timestamp)
	})

	t.Run("both timeout values equal non-zero values - use specified timeout values", func(t *testing.T) {
		opts := ibc.TransferOptions{
			Timeout: &ibc.IBCTimeout{
				NanoSeconds: 12345,
				Height:      12345,
			},
		}

		height, timestamp := ibcTransferTimeouts(opts)
		require.Equal(t, opts.Timeout.Height, height.RevisionHeight)
		require.Equal(t, zero, height.RevisionNumber)
		require.Equal(t, opts.Timeout.NanoSeconds, timestamp)
	})

	t.Run("only nanoseconds equals non-zero value - use specified value for timestamp and zero for height", func(t *testing.T) {
		opts := ibc.TransferOptions{
			Timeout: &ibc.IBCTimeout{
				NanoSeconds: 12345,
			},
		}

		height, timestamp := ibcTransferTimeouts(opts)
		require.Equal(t, zero, height.RevisionHeight)
		require.Equal(t, zero, height.RevisionNumber)
		require.Equal(t, opts.Timeout.NanoSeconds, timestamp)
	})

	t.Run("only height equals non-zero value - use specified value for height and zero for timestamp", func(t *testing.T) {
		opts := ibc.TransferOptions{
			Timeout: &ibc.IBCTimeout{
				Height: 12345,
			},
		}

		height, timestamp := ibcTransferTimeouts(opts)
		require.Equal(t, opts.Timeout.Height, height.RevisionHeight)
		require.Equal(t, zero, height.RevisionNumber)
		require.Equal(t, zero, timestamp)
	})
}
