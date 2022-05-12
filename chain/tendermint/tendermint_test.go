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
