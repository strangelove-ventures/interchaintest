package presenter

import (
	"encoding/json"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v5/internal/blockdb"
	"github.com/stretchr/testify/require"
)

func TestTx(t *testing.T) {
	t.Parallel()

	t.Run("json", func(t *testing.T) {
		tx := blockdb.TxResult{
			Height: 13,
			Tx:     []byte(`{"json":{"foo":true}}`),
		}
		require.True(t, json.Valid(tx.Tx)) // sanity check

		pres := Tx{tx}
		require.Equal(t, "13", pres.Height())

		const want = `{
  "json": {
    "foo": true
  }
}`
		require.Equal(t, want, pres.Data())
	})

	t.Run("non-json", func(t *testing.T) {
		tx := blockdb.TxResult{
			Tx: []byte(`some data`),
		}
		pres := Tx{tx}
		require.Equal(t, "some data", pres.Data())
	})
}

func TestTxs_ToJSON(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		txs := Txs{
			{Height: 1, Tx: []byte(`{"num":1}`)},
			{Height: 3, Tx: []byte(`{"num":3}`)},
			{Height: 5, Tx: []byte(`{"num":5}`)},
		}

		const want = `[
{ "Height": 1, "Tx": { "num": 1 } },
{ "Height": 3, "Tx": { "num": 3 } },
{ "Height": 5, "Tx": { "num": 5 } }
]`
		require.JSONEq(t, want, string(txs.ToJSON()))
	})

	t.Run("invalid json", func(t *testing.T) {
		txs := Txs{
			{Height: 1, Tx: []byte(`{"num":1}`)},
			{Height: 2, Tx: []byte(`not valid`)},
		}

		const want = `[
{ "Height": 1, "Tx": { "num": 1 } },
{ "Height": 2, "Tx": "bm90IHZhbGlk" }
]`
		require.JSONEq(t, want, string(txs.ToJSON()))
	})
}
