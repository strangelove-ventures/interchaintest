package cosmos

import (
	"context"
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
)

type blockClient interface {
	Block(ctx context.Context, height *int64) (*tmtypes.ResultBlock, error)
}

func findMsgFromBlock[T sdk.Msg](ctx context.Context, client blockClient, height uint64) (zero T, _ error) {
	h := int64(height)
	block, err := client.Block(ctx, &h)
	if err != nil {
		return zero, fmt.Errorf("tendermint rpc get block: %w", err)
	}
	for _, txbz := range block.Block.Txs {
		tx, err := decodeTX(txbz)
		if err != nil {
			return zero, fmt.Errorf("decode tendermint tx: %w", err)
		}
		found, ok := findMsgFromTx[T](tx)
		if ok {
			return found, nil
		}
	}
	return zero, errors.New("msg not found")
}

func findMsgFromTx[T sdk.Msg](tx sdk.Tx) (zero T, _ bool) {
	for _, m := range tx.GetMsgs() {
		found, ok := m.(T)
		if !ok {
			continue
		}
		return found, true
	}
	return zero, false
}
