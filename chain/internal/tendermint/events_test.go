package tendermint

import (
	"testing"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/require"
)

func TestAttributeValue(t *testing.T) {
	events := []abcitypes.Event{
		{Type: "1", Attributes: []abcitypes.EventAttribute{
			{Key: "ignore", Value: "should not see me"},
			{Key: "key1", Value: "found1"},
		}},
		{Type: "2", Attributes: []abcitypes.EventAttribute{
			{Key: "key2", Value: "found2"},
			{Key: "ignore", Value: "should not see me"},
		}},
	}

	_, ok := AttributeValue(nil, "test", "")
	require.False(t, ok)

	_, ok = AttributeValue(events, "key_not_there", "ignored")
	require.False(t, ok)

	_, ok = AttributeValue(events, "1", "attribute not there")
	require.False(t, ok)

	found, ok := AttributeValue(events, "1", "key1")
	require.True(t, ok)
	require.Equal(t, "found1", found)

	found, ok = AttributeValue(events, "2", "key2")
	require.True(t, ok)
	require.Equal(t, "found2", found)
}
