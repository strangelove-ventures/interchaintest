package ibc

import (
	"testing"

	"github.com/stretchr/testify/require"
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
		for _, tt := range []struct {
			Tx      Tx
			WantErr string
		}{
			{
				Tx{},
				"tx height cannot be 0",
			},
			{
				Tx{Height: 1},
				"tx hash cannot be empty",
			},
			{
				Tx{Height: 1, TxHash: "123"},
				"tx gas spent cannot be 0",
			},
		} {
			err := tt.Tx.Validate()

			require.Error(t, err, tt)
			require.EqualError(t, err, tt.WantErr, tt)
		}

		tx := Tx{
			Height:   1,
			TxHash:   "abc",
			GasSpent: 10,
		}
		require.Error(t, tx.Validate())
	})
}
