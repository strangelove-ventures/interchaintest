package presenter

import (
	"encoding/json"
	"testing"

	"github.com/strangelove-ventures/ibctest/internal/blockdb"
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
