package utxo_test

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/utxo"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"golang.org/x/sync/errgroup"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestUtxo(t *testing.T) {

	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)
	ctx := context.Background()

	// Get default bitcoin chain config
	btcConfig := utxo.DefaultBitcoinChainConfig("btc", "rpcuser", "password")
	bchConfig := utxo.DefaultBitcoinCashChainConfig("bch", "rpcuser", "password")
	liteConfig := utxo.DefaultLitecoinChainConfig("ltc", "rpcuser", "password")
	dogeConfig := utxo.DefaultDogecoinChainConfig("doge", "rpcuser", "password")

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			ChainName:   "btc",
			Name:        "btc",
			Version:     "26.2",
			ChainConfig: btcConfig,
		},
		{
			ChainName:   "bch",
			Name:        "bch",
			Version:     "27.1.0",
			ChainConfig: bchConfig,
		},
		{
			ChainName:   "ltc",
			Name:        "ltc",
			Version:     "0.21",
			ChainConfig: liteConfig,
		},
		{
			ChainName:   "doge",
			Name:        "doge",
			Version:     "dogecoin-daemon-1.14.7",
			ChainConfig: dogeConfig,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	ic := interchaintest.NewInterchain()
	for _, chain := range chains {
		ic.AddChain(chain)
	}

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		SkipPathCreation: true, // Skip path creation, so we can have granular control over the process
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create and fund a user using GetAndFundTestUsers
	eg, egCtx := errgroup.WithContext(ctx)
	for _, chain := range chains {
		chain := chain
		eg.Go(func() error {
			// Fund 2 coins to user1 and user2
			fundAmount := sdkmath.NewInt(200_000_000)
			users := interchaintest.GetAndFundTestUsers(t, egCtx, "user1", fundAmount, chain)
			user1 := users[0]
			users = interchaintest.GetAndFundTestUsers(t, egCtx, "user2", fundAmount, chain)
			user2 := users[0]
	
			// Verify user1 balance
			balanceUser1, err := chain.GetBalance(egCtx, user1.FormattedAddress(), "")
			if err != nil {
				return err
			}
			if !balanceUser1.Equal(fundAmount) {
				return fmt.Errorf("User (%s) balance (%s) is not expected (%s)", user1.KeyName(), balanceUser1, fundAmount)
			}

			// Verify user2 balance
			balanceUser2, err := chain.GetBalance(ctx, user2.FormattedAddress(), "")
			if err != nil {
				return err
			}
			if !balanceUser2.Equal(fundAmount) {
				return fmt.Errorf("User (%s) balance (%s) is not expected (%s)", user2.KeyName(), balanceUser2, fundAmount)
			}
			
			// Send 1 coin from user1 to user2 with a note/memo
			memo := fmt.Sprintf("+:%s:%s", "abc.abc", "bech16sg0fxrdd0vgpl4pkcnqwzjlu5lrs6ymcqldel")
			transferAmount := sdkmath.NewInt(100_000_000)
			_, err = chain.SendFundsWithNote(ctx, user1.KeyName(), ibc.WalletAmount{
				Address: user2.FormattedAddress(),
				Amount: transferAmount,
			}, memo)
			if err != nil {
				return err
			}
			
			// Verify user1 balance
			balanceUser1, err = chain.GetBalance(egCtx, user1.FormattedAddress(), "")
			if err != nil {
				return err
			}
			fees, err := strconv.ParseFloat(chain.Config().GasPrices, 64)
			if err != nil {
				return err
			}
			feeScaled := fees * chain.Config().GasAdjustment * math.Pow10(int(*chain.Config().CoinDecimals))
			expectedBalance := fundAmount.Sub(transferAmount).SubRaw(int64(feeScaled))
			if !balanceUser1.Equal(expectedBalance) {
				return fmt.Errorf("User (%s) balance (%s) is not expected (%s)", user1.KeyName(), balanceUser1, expectedBalance)
			}

			// Verify user2 balance
			balanceUser2, err = chain.GetBalance(ctx, user2.FormattedAddress(), "")
			if err != nil {
				return err
			}
			expectedBalance = fundAmount.Add(transferAmount)
			if !balanceUser2.Equal(expectedBalance) {
				return fmt.Errorf("User (%s) balance (%s) is not expected (%s)", user2.KeyName(), balanceUser2, expectedBalance)
			}

			return nil
		})
	}
	require.NoError(t, eg.Wait())
}
