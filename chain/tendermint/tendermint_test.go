package tendermint

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

func TestPrettyPrintTxs(t *testing.T) {
	ctx := context.Background()

	t.Run("with transactions", func(t *testing.T) {
		got, err := PrettyPrintTxs(ctx, types.Txs{types.Tx("test")}, func(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) {
			require.NotNil(t, ctx)
			require.Equal(t, types.Tx("test").Hash(), hash)

			return &coretypes.ResultTx{
				TxResult: abcitypes.ResponseDeliverTx{
					Data: []byte("test data"),
					Events: []abcitypes.Event{
						{Type: "event1.type", Attributes: []abcitypes.EventAttribute{
							{Key: []byte("event1.key"), Value: []byte("event1.value")},
						}},
					},
				},
			}, nil
		})

		require.NoError(t, err)

		require.NotEmpty(t, got)
		require.Contains(t, got, "test")
		require.Contains(t, got, "event1.type")
		require.Contains(t, got, "event1.key")
		require.Contains(t, got, "event1.value")
	})
}
