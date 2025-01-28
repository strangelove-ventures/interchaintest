package thorchain

import (
	"context"
	"errors"
	"fmt"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"

	"github.com/strangelove-ventures/interchaintest/v9/ibc"
	"github.com/strangelove-ventures/interchaintest/v9/testutil"
)

// PollForMessage searches every transaction for a message. Must pass a coded registry capable of decoding the cosmos transaction.
// fn is optional. Return true from the fn to stop polling and return the found message. If fn is nil, returns the first message to match type T.
func PollForMessage[T any](ctx context.Context, chain *Thorchain, registry codectypes.InterfaceRegistry, startHeight, maxHeight int64, fn func(found T) bool) (T, error) {
	var zero T
	if fn == nil {
		fn = func(T) bool { return true }
	}
	doPoll := func(ctx context.Context, height int64) (T, error) {
		h := height
		block, err := chain.getFullNode().Client.Block(ctx, &h)
		if err != nil {
			return zero, err
		}
		for _, tx := range block.Block.Txs {
			sdkTx, err := decodeTX(registry, tx)
			if err != nil {
				return zero, err
			}
			for _, msg := range sdkTx.GetMsgs() {
				if found, ok := msg.(T); ok {
					if fn(found) {
						return found, nil
					}
				}
			}
		}
		return zero, errors.New("not found")
	}

	bp := testutil.BlockPoller[T]{CurrentHeight: chain.Height, PollFunc: doPoll}
	return bp.DoPoll(ctx, startHeight, maxHeight)
}

// PollForBalance polls until the balance matches.
func PollForBalance(ctx context.Context, chain *Thorchain, deltaBlocks int64, balance ibc.WalletAmount) error {
	h, err := chain.Height(ctx)
	if err != nil {
		return fmt.Errorf("failed to get height: %w", err)
	}
	doPoll := func(ctx context.Context, height int64) (any, error) {
		bal, err := chain.GetBalance(ctx, balance.Address, balance.Denom)
		if err != nil {
			return nil, err
		}
		if !balance.Amount.Equal(bal) {
			return nil, fmt.Errorf("balance (%s) does not match expected: (%s)", bal.String(), balance.Amount.String())
		}
		return nil, nil
	}
	bp := testutil.BlockPoller[any]{CurrentHeight: chain.Height, PollFunc: doPoll}
	_, err = bp.DoPoll(ctx, h, h+deltaBlocks)
	return err
}
