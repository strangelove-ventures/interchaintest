package cosmos

import (
	"context"
	"fmt"

	tmtypes "github.com/cometbft/cometbft/rpc/core/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type blockClient interface {
	Block(ctx context.Context, height *int64) (*tmtypes.ResultBlock, error)
}

// rangeBlockMessages iterates through all a block's transactions and each transaction's messages yielding to f.
// Return true from f to stop iteration.
func rangeBlockMessages(ctx context.Context, interfaceRegistry codectypes.InterfaceRegistry, client blockClient, height uint64, done func(sdk.Msg) bool) error {
	h := int64(height)
	block, err := client.Block(ctx, &h)
	if err != nil {
		return fmt.Errorf("tendermint rpc get block: %w", err)
	}
	for _, txbz := range block.Block.Txs {
		tx, err := decodeTX(interfaceRegistry, txbz)
		if err != nil {
			return fmt.Errorf("decode tendermint tx: %w", err)
		}
		for _, m := range tx.GetMsgs() {
			if ok := done(m); ok {
				return nil
			}
		}
	}
	return nil
}
