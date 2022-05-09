package tendermint

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

type mockClient struct {
	GotHeight       int64
	StubResultBlock *coretypes.ResultBlock
	StubResultTx    *coretypes.ResultTx
}

func (m *mockClient) Block(ctx context.Context, height *int64) (*coretypes.ResultBlock, error) {
	m.GotHeight = *height
	return m.StubResultBlock, nil
}

func (m *mockClient) Tx(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error) {
	return m.StubResultTx, nil
}

func (m *mockClient) BlockByHash(ctx context.Context, hash []byte) (*coretypes.ResultBlock, error) {
	panic("implement me")
}

func (m *mockClient) BlockResults(ctx context.Context, height *int64) (*coretypes.ResultBlockResults, error) {
	panic("implement me")
}

func (m *mockClient) Commit(ctx context.Context, height *int64) (*coretypes.ResultCommit, error) {
	panic("implement me")
}

func (m *mockClient) Validators(ctx context.Context, height *int64, page, perPage *int) (*coretypes.ResultValidators, error) {
	panic("implement me")
}

func (m *mockClient) TxSearch(ctx context.Context, query string, prove bool, page, perPage *int, orderBy string) (*coretypes.ResultTxSearch, error) {
	panic("implement me")
}

func (m *mockClient) BlockSearch(ctx context.Context, query string, page, perPage *int, orderBy string) (*coretypes.ResultBlockSearch, error) {
	panic("implement me")
}

func TestPrettyPrintBlock(t *testing.T) {
	client := &mockClient{
		StubResultBlock: &coretypes.ResultBlock{
			Block: &types.Block{
				Data: types.Data{
					Txs: types.Txs{types.Tx("test")},
				},
			},
		},
		StubResultTx: &coretypes.ResultTx{
			TxResult: abcitypes.ResponseDeliverTx{
				Data: []byte("test data"),
				Events: []abcitypes.Event{
					{Type: "event1.type", Attributes: []abcitypes.EventAttribute{
						{Key: []byte("event1.key"), Value: []byte("event1.value")},
					}},
				},
			},
		},
	}

	got, err := PrettyPrintBlock(context.Background(), client, 3)
	require.NoError(t, err)

	require.EqualValues(t, 3, client.GotHeight)
	require.NotEmpty(t, got)
	require.Contains(t, got, "BLOCK 3")
	require.Contains(t, got, "test")
	require.Contains(t, got, "event1.type")
	require.Contains(t, got, "event1.key")
	require.Contains(t, got, "event1.value")
}
