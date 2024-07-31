package thorchain_test

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/math"
	
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

// Send 100 txs on gaia so that bifrost can automatically set the network fee
// Sim testing can directly use bifrost to do this, right now, we can't, but may in the future
func doTxs(t *testing.T, ctx context.Context, gaia *cosmos.CosmosChain) {
	fundAmount := math.NewInt(100_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "temp", fundAmount, gaia, gaia, gaia, gaia, gaia, gaia, gaia, gaia)

	amount := ibc.WalletAmount{
		Denom: gaia.Config().Denom,
		Amount: math.NewInt(1_000_000),
	}

	val0 := gaia.GetNode()
	for i := 0; i < 12; i++ {
		go sendFunds(ctx, users[0].KeyName(), users[1].FormattedAddress(), amount, val0)
		go sendFunds(ctx, users[1].KeyName(), users[0].FormattedAddress(), amount, val0)
		go sendFunds(ctx, users[2].KeyName(), users[3].FormattedAddress(), amount, val0)
		go sendFunds(ctx, users[3].KeyName(), users[2].FormattedAddress(), amount, val0)
		go sendFunds(ctx, users[4].KeyName(), users[5].FormattedAddress(), amount, val0)
		go sendFunds(ctx, users[5].KeyName(), users[4].FormattedAddress(), amount, val0)
		go sendFunds(ctx, users[6].KeyName(), users[7].FormattedAddress(), amount, val0)
		go sendFunds(ctx, users[7].KeyName(), users[6].FormattedAddress(), amount, val0)
		_ = testutil.WaitForBlocks(ctx, 1, gaia)
	}

}

func sendFunds(ctx context.Context, keyName string, toAddr string, amount ibc.WalletAmount, val0 *cosmos.ChainNode) {
	command := []string{"bank", "send", keyName, toAddr, fmt.Sprintf("%s%s", amount.Amount.String(), amount.Denom),}
	_, _, _ = val0.Exec(ctx, val0.TxCommand(keyName, command...), val0.Chain.Config().Env)
}