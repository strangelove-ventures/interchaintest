package tendermint

import (
	"bytes"

	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// AttributeValue returns an event attribute value given the eventType and attribute key tuple.
// In the event of duplicate types and keys, returns the first attribute value found.
// If not found, ok is false.
func AttributeValue(events []abcitypes.Event, eventType string, attrKey []byte) (found []byte, ok bool) {
	for _, event := range events {
		if event.Type != eventType {
			continue
		}
		for _, attr := range event.Attributes {
			if bytes.Equal(attr.Key, attrKey) {
				return attr.Value, true
			}
		}
	}
	return nil, false
}
