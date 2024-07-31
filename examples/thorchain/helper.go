package thorchain_test

import (
	"context"
	"fmt"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

// PollForBalanceChaqnge polls until the balance changes
func PollForBalanceChange(ctx context.Context, chain ibc.Chain, deltaBlocks int64, balance ibc.WalletAmount) error {
	h, err := chain.Height(ctx)
	if err != nil {
		return fmt.Errorf("failed to get height: %w", err)
	}
	doPoll := func(ctx context.Context, height int64) (any, error) {
		bal, err := chain.GetBalance(ctx, balance.Address, balance.Denom)
		if err != nil {
			return nil, err
		}
		if balance.Amount.Equal(bal) {
			return nil, fmt.Errorf("balance (%s) hasn't changed: (%s)", bal.String(), balance.Amount.String())
		}
		return nil, nil
	}
	bp := testutil.BlockPoller[any]{CurrentHeight: chain.Height, PollFunc: doPoll}
	_, err = bp.DoPoll(ctx, h, h+deltaBlocks)
	return err
}