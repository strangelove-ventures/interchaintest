package tendermint

import (
	"bytes"
	"context"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
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

// PrettyPrintTxs pretty prints tendermint transactions and events
func PrettyPrintTxs(ctx context.Context, txs types.Txs, getTx func(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error)) (string, error) {
	buf := new(bytes.Buffer)
	for i, tx := range txs {
		buf.WriteString(fmt.Sprintf("TX %d: ", i))

		// Tx data may contain useful information such as protobuf message type but will be
		// mixed in with non-human readable data.
		buf.WriteString("DATA:\n")
		buf.Write(tx)
		buf.WriteString("\n")

		resTx, err := getTx(ctx, tx.Hash(), false)
		if err != nil {
			return "", fmt.Errorf("tendermint rpc get tx: %w", err)
		}
		buf.WriteString("EVENTS:\n")
		spew.Fdump(buf, resTx.TxResult.Events)
	}

	return buf.String(), nil
}
