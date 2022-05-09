package tendermint

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

func TestAttributeValue(t *testing.T) {
	events := []abcitypes.Event{
		{Type: "1", Attributes: []abcitypes.EventAttribute{
			{Key: []byte("ignore"), Value: []byte("should not see me")},
			{Key: []byte("key1"), Value: []byte("found me 1")},
		}},
		{Type: "2", Attributes: []abcitypes.EventAttribute{
			{Key: []byte("key2"), Value: []byte("found me 2")},
			{Key: []byte("ignore"), Value: []byte("should not see me")},
		}},
	}

	_, ok := AttributeValue(nil, "test", nil)
	require.False(t, ok)

	_, ok = AttributeValue(events, "key_not_there", []byte("ignored"))
	require.False(t, ok)

	_, ok = AttributeValue(events, "1", []byte("attribute not there"))
	require.False(t, ok)

	got, ok := AttributeValue(events, "1", []byte("key1"))
	require.True(t, ok)
	require.Equal(t, "found me 1", string(got))

	got, ok = AttributeValue(events, "2", []byte("key2"))
	require.True(t, ok)
	require.Equal(t, "found me 2", string(got))
}

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
