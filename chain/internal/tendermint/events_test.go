package tendermint

import (
	"testing"

	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

func TestAttributeValue(t *testing.T) {
	events := []abcitypes.Event{
		{Type: "1", Attributes: []abcitypes.EventAttribute{
			{Key: []byte("ignore"), Value: []byte("should not see me")},
			{Key: []byte("key1"), Value: []byte("found1")},
		}},
		{Type: "2", Attributes: []abcitypes.EventAttribute{
			{Key: []byte("key2"), Value: []byte("found2")},
			{Key: []byte("ignore"), Value: []byte("should not see me")},
		}},
	}

	_, ok := AttributeValue(nil, "testutil", "")
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
