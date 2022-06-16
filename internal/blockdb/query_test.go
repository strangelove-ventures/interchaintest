package blockdb

import (
	"context"
	_ "embed"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/sample_txs.json
	txsFixture []byte
)

func TestQuery_CurrentSchemaVersion(t *testing.T) {
	t.Parallel()

	db := emptyDB()
	defer db.Close()

	require.NoError(t, Migrate(db, "first-sha"))
	require.NoError(t, Migrate(db, "second-sha"))

	res, err := NewQuery(db).CurrentSchemaVersion(context.Background())

	require.NoError(t, err)
	require.Equal(t, "second-sha", res.GitSha)
	require.WithinDuration(t, res.CreatedAt, time.Now(), 10*time.Second)
}

func TestQuery_RecentTestCases(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "test1", "sha1")
		require.NoError(t, err)
		c, err := tc.AddChain(ctx, "chain-b", "cosmos")
		require.NoError(t, err)
		require.NoError(t, c.SaveBlock(ctx, 10, []Tx{{Data: []byte("tx1")}, {Data: []byte("tx2")}}))
		require.NoError(t, c.SaveBlock(ctx, 11, []Tx{{Data: []byte("tx3")}}))

		_, err = tc.AddChain(ctx, "chain-a", "cosmos")
		require.NoError(t, err)

		_, err = CreateTestCase(ctx, db, "empty", "empty-test")
		require.NoError(t, err)

		results, err := NewQuery(db).RecentTestCases(ctx, 10)
		require.NoError(t, err)

		require.Len(t, results, 2)

		// No blocks or txs.
		got := results[0]
		require.EqualValues(t, 1, got.ID)
		require.Equal(t, "test1", got.Name)
		require.Equal(t, "sha1", got.GitSha)
		require.WithinDuration(t, time.Now(), got.CreatedAt, 10*time.Second)
		require.Equal(t, "chain-a", got.ChainID)
		require.Equal(t, "cosmos", got.ChainType)
		require.EqualValues(t, 2, got.ChainPKey)
		require.Zero(t, got.ChainHeight.Int64)
		require.Zero(t, got.TxTotal.Int64)

		// With blocks and txs.
		got = results[1]
		require.EqualValues(t, 1, got.ID)
		require.Equal(t, "test1", got.Name)
		require.WithinDuration(t, time.Now(), got.CreatedAt, 10*time.Second)
		require.Equal(t, "chain-b", got.ChainID)
		require.Equal(t, "cosmos", got.ChainType)
		require.EqualValues(t, 1, got.ChainPKey)
		require.EqualValues(t, 11, got.ChainHeight.Int64)
		require.EqualValues(t, 3, got.TxTotal.Int64)
	})

	t.Run("limit", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "1", "1")
		require.NoError(t, err)
		_, err = tc.AddChain(ctx, "chain1", "cosmos")
		require.NoError(t, err)
		_, err = tc.AddChain(ctx, "chain2", "cosmos")
		require.NoError(t, err)

		got, err := NewQuery(db).RecentTestCases(ctx, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
	})

	t.Run("no test cases", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		got, err := NewQuery(db).RecentTestCases(ctx, 1)

		require.NoError(t, err)
		require.Empty(t, got)
	})
}

func TestQuery_CosmosMessages(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	db := migratedDB()
	defer db.Close()

	tc, err := CreateTestCase(ctx, db, "test", "sha")
	require.NoError(t, err)
	chain, err := tc.AddChain(ctx, "chain1", "cosmos")
	require.NoError(t, err)

	var txs []struct {
		Raw string `json:"tx"`
	}

	err = json.Unmarshal(txsFixture, &txs)
	require.NoError(t, err)
	require.NotEmpty(t, txs)

	for i, tx := range txs {
		require.NotEmpty(t, tx.Raw)
		err = chain.SaveBlock(ctx, uint64(i+1), []Tx{{Data: []byte(tx.Raw)}})
		require.NoError(t, err)
	}

	results, err := NewQuery(db).CosmosMessages(ctx, chain.id)
	require.NoError(t, err)
	require.NotEmpty(t, results)

	first := results[0]
	require.EqualValues(t, 1, first.Height)
	require.EqualValues(t, 0, first.Index)
	require.Equal(t, "/ibc.core.client.v1.MsgCreateClient", first.Type)

	second := results[1]
	require.EqualValues(t, 2, second.Height)
	require.EqualValues(t, 0, second.Index)
	require.Equal(t, "/ibc.core.client.v1.MsgUpdateClient", second.Type)

	third := results[2]
	require.EqualValues(t, 2, third.Height)
	require.EqualValues(t, 1, third.Index)
	require.Equal(t, "/ibc.core.connection.v1.MsgConnectionOpenInit", third.Type)

	for _, res := range results {
		if !strings.HasPrefix(res.Type, "/ibc") {
			continue
		}
		atLeastOnePresent := res.ClientChainID.Valid ||
			res.ClientID.Valid || res.CounterpartyClientID.Valid ||
			res.ConnID.Valid || res.CounterpartyConnID.Valid ||
			res.PortID.Valid || res.CounterpartyPortID.Valid ||
			res.ChannelID.Valid || res.CounterpartyChannelID.Valid
		require.Truef(t, atLeastOnePresent, "IBC messages must contain valid IBC info for %+v", res)
	}
}

func TestQuery_Transactions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("happy path", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "test", "abc123")
		require.NoError(t, err)
		chain, err := tc.AddChain(ctx, "chain-a", "cosmos")
		require.NoError(t, err)

		require.NoError(t, chain.SaveBlock(ctx, 12, []Tx{{Data: []byte(`1`)}}))
		require.NoError(t, chain.SaveBlock(ctx, 14, []Tx{{Data: []byte(`2`)}, {Data: []byte(`3`)}}))

		results, err := NewQuery(db).Transactions(ctx, chain.id)
		require.NoError(t, err)

		require.Len(t, results, 3)

		require.EqualValues(t, 12, results[0].Height)
		require.Equal(t, "1", string(results[0].Tx))

		require.EqualValues(t, 14, results[1].Height)
		require.Equal(t, "2", string(results[1].Tx))

		require.EqualValues(t, 14, results[2].Height)
		require.Equal(t, "3", string(results[2].Tx))
	})

	t.Run("no txs", func(t *testing.T) {
		db := migratedDB()
		defer db.Close()

		tc, err := CreateTestCase(ctx, db, "test", "abc123")
		require.NoError(t, err)
		chain, err := tc.AddChain(ctx, "chain-a", "cosmos")
		require.NoError(t, err)

		results, err := NewQuery(db).Transactions(ctx, chain.id)
		require.NoError(t, err)

		require.Len(t, results, 0)
	})
}
