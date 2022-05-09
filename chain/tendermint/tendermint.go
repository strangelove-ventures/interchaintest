package tendermint

import (
	"bytes"
	"context"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// PrettyPrintTxs pretty prints tendermint transactions and events
func PrettyPrintTxs(ctx context.Context, txs types.Txs, getTx func(ctx context.Context, hash []byte, prove bool) (*coretypes.ResultTx, error)) (string, error) {
	buf := new(bytes.Buffer)
	for i, tx := range txs {
		buf.WriteString(fmt.Sprintf("TX %d: ", i))
		buf.WriteString(tx.String() + "\n")

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
