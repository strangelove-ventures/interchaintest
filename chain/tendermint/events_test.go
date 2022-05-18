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
			{Key: []byte("key1"), Value: []byte("found me 1")},
		}},
		{Type: "2", Attributes: []abcitypes.EventAttribute{
			{Key: []byte("key2"), Value: []byte("found me 2")},
			{Key: []byte("ignore"), Value: []byte("should not see me")},
		}},
	}

	found := AttributeValue(nil, "test", "")
	require.Empty(t, found)

	found = AttributeValue(events, "key_not_there", "ignored")
	require.Empty(t, found)

	found = AttributeValue(events, "1", "attribute not there")
	require.Empty(t, found)

	found = AttributeValue(events, "1", "key1")
	require.Equal(t, "found me 1", found)

	found = AttributeValue(events, "2", "key2")
	require.Equal(t, "found me 2", found)
}
