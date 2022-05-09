package tendermint

import (
	"bytes"
	"context"
	"fmt"
	"strconv"

	"github.com/davecgh/go-spew/spew"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

// PrettyPrintBlock pretty prints tendermint block, all its transactions, and events
func PrettyPrintBlock(ctx context.Context, client rpcclient.SignClient, height int64) (string, error) {
	blockRes, err := client.Block(ctx, &height)
	if err != nil {
		return "", fmt.Errorf("tendermint rpc get block: %w", err)
	}
	buf := new(bytes.Buffer)
	buf.WriteString("BLOCK " + strconv.FormatInt(height, 10) + "\n")

	for i, tx := range blockRes.Block.Txs {
		buf.WriteString(fmt.Sprintf("TX %d:\n", i))
		buf.WriteString(tx.String() + "\n")

		// Tx data may contain useful information such as protobuf message type but will be
		// mixed in with non-human readable data.
		buf.WriteString("DATA:\n")
		buf.Write(tx)
		buf.WriteString("\n")

		resTx, err := client.Tx(ctx, tx.Hash(), false)
		if err != nil {
			return "", fmt.Errorf("tendermint rpc get tx: %w", err)
		}
		buf.WriteString("EVENTS:\n")
		spew.Fdump(buf, resTx.TxResult.Events)
	}

	return buf.String(), nil
}
