package ibc

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
)

func TestTx_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		tx := Tx{
			Height:   1,
			TxHash:   "abc",
			GasSpent: 10,
			Packet:   validPacket(),
		}

		require.NoError(t, tx.Validate())
	})

	t.Run("invalid", func(t *testing.T) {
		var empty Tx
		err := empty.Validate()
		require.Greater(t, len(multierr.Errors(err)), 1)

		require.Error(t, err)
		require.Contains(t, err.Error(), "tx height cannot be 0")
		require.Contains(t, err.Error(), "tx hash cannot be empty")
		require.Contains(t, err.Error(), "tx gas spent cannot be 0")

		tx := Tx{
			Height:   1,
			TxHash:   "abc",
			GasSpent: 10,
		}
		require.Error(t, tx.Validate())
	})
}
