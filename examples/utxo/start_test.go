package ethereum_test

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/utxo"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"

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
			ChainName:   "bitcoin",
			Name:        "bitcoin",
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
			ChainName:   "litecoin",
			Name:        "litecoin",
			Version:     "0.21",
			ChainConfig: liteConfig,
		},
		{
			ChainName:   "dogecoin",
			Name:        "dogecoin",
			Version:     "dogecoin-daemon-1.14.7",
			ChainConfig: dogeConfig,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	btcChain := chains[0].(*utxo.UtxoChain)
	bchChain := chains[1].(*utxo.UtxoChain)
	liteChain := chains[2].(*utxo.UtxoChain)
	dogeChain := chains[3].(*utxo.UtxoChain)

	btcChain.UnloadWalletAfterUse(true)
	bchChain.UnloadWalletAfterUse(true)
	liteChain.UnloadWalletAfterUse(true)

	ic := interchaintest.NewInterchain().
		AddChain(btcChain).
		AddChain(bchChain).
		AddChain(liteChain).
		AddChain(dogeChain)

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation: true, // Skip path creation, so we can have granular control over the process
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})
	fmt.Println("Interchain built")

	// ------ BTC -------
	// Check faucet balance on start
	faucetAddrBz, err := btcChain.GetAddress(ctx, "faucet")
	require.NoError(t, err)
	faucetAddr := string(faucetAddrBz)
	balance, err := btcChain.GetBalance(ctx, faucetAddr, "")
	require.NoError(t, err)
	fmt.Println("BTC faucet balance:", balance)

	// Create and fund a user using GetAndFundTestUsers
	btcUserInitialAmount := math.NewInt(200_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", btcUserInitialAmount, btcChain)
	btcUser := users[0]
	fmt.Println("Btc user:", btcUser.KeyName())

	balance, err = btcChain.GetBalance(ctx, btcUser.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, btcUserInitialAmount.Equal(balance), fmt.Sprintf("%s user balance (%s) is not expected (%s)", btcUser.KeyName(), balance, btcUserInitialAmount))
	fmt.Println("Btc user balance:", balance)

	btcUserInitialAmount = math.NewInt(100_000)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "user2", btcUserInitialAmount, btcChain)
	btcUser2 := users[0]
	fmt.Println("Btc user2:", btcUser2.KeyName())

	balance, err = btcChain.GetBalance(ctx, btcUser2.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, btcUserInitialAmount.Equal(balance), fmt.Sprintf("%s user balance (%s) is not expected (%s)", btcUser2.KeyName(), balance, btcUserInitialAmount))
	fmt.Println("Btc user2 balance:", balance)

	memo := fmt.Sprintf("+:%s:%s", "BTC.BTC", "tthor16sg0fxrdd0vgpl4pkcnqwzjlu5lrs6ymcqldel")
	txHash, err := btcChain.SendFundsWithNote(ctx, btcUser.KeyName(), ibc.WalletAmount{
		Address: btcUser2.FormattedAddress(),
		Amount: math.NewInt(100_000_000),
	}, memo)
	require.NoError(t, err)
	fmt.Println("txHash:", txHash)

	balance, err = btcChain.GetBalance(ctx, btcUser2.FormattedAddress(), "")
	require.NoError(t, err)
	fmt.Println("Btc user2 balance after memo tx:", balance)

	// TODO: Use SendFundsWithNote

	// ------ BCH -------
	// Check faucet balance on start
	faucetAddrBz, err = bchChain.GetAddress(ctx, "faucet")
	require.NoError(t, err)
	faucetAddr = string(faucetAddrBz)
	balance, err = bchChain.GetBalance(ctx, faucetAddr, "")
	require.NoError(t, err)
	fmt.Println("BCH faucet balance:", balance)

	// Create and fund a user using GetAndFundTestUsers
	bchUserInitialAmount := math.NewInt(250_000_000)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "user", bchUserInitialAmount, bchChain)
	bchUser := users[0]
	fmt.Println("Bch user:", bchUser.KeyName())

	balance, err = bchChain.GetBalance(ctx, bchUser.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, bchUserInitialAmount.Equal(balance), fmt.Sprintf("%s user balance (%s) is not expected (%s)", bchUser.KeyName(), balance, bchUserInitialAmount))
	fmt.Println("Bch user balance:", balance)

	bchUserInitialAmount = math.NewInt(100_000)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "user2", bchUserInitialAmount, bchChain)
	bchUser2 := users[0]
	fmt.Println("Bch user2:", bchUser2.KeyName())

	balance, err = bchChain.GetBalance(ctx, bchUser2.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, bchUserInitialAmount.Equal(balance), fmt.Sprintf("%s user balance (%s) is not expected (%s)", bchUser2.KeyName(), balance, bchUserInitialAmount))
	fmt.Println("Bch user2 balance:", balance)

	memo = fmt.Sprintf("+:%s:%s", "BCH.BCH", "tthor16sg0fxrdd0vgpl4pkcnqwzjlu5lrs6ymcqldel")
	txHash, err = bchChain.SendFundsWithNote(ctx, bchUser.KeyName(), ibc.WalletAmount{
		Address: bchUser2.FormattedAddress(),
		Amount: math.NewInt(100_000_000),
	}, memo)
	require.NoError(t, err)
	fmt.Println("txHash:", txHash)

	balance, err = bchChain.GetBalance(ctx, bchUser2.FormattedAddress(), "")
	require.NoError(t, err)
	fmt.Println("Bch user2 balance after memo tx:", balance)

	// TODO: Use SendFundsWithNote

	// ------ litecoin -------
	// Check faucet balance on start
	faucetAddrBz, err = liteChain.GetAddress(ctx, "faucet")
	require.NoError(t, err)
	faucetAddr = string(faucetAddrBz)
	balance, err = liteChain.GetBalance(ctx, faucetAddr, "")
	require.NoError(t, err)
	fmt.Println("Lite faucet balance:", balance)

	// Create and fund a user using GetAndFundTestUsers
	liteUserInitialAmount := math.NewInt(270_000_000)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "user", liteUserInitialAmount, liteChain)
	liteUser := users[0]
	fmt.Println("Lite user:", liteUser.KeyName())

	balance, err = liteChain.GetBalance(ctx, liteUser.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, liteUserInitialAmount.Equal(balance), fmt.Sprintf("%s user balance (%s) is not expected (%s)", liteUser.KeyName(), balance, liteUserInitialAmount))
	fmt.Println("Lite user balance:", balance)

	liteUserInitialAmount = math.NewInt(100_000)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "user2", liteUserInitialAmount, liteChain)
	liteUser2 := users[0]
	fmt.Println("Lite user2:", liteUser2.KeyName())

	balance, err = liteChain.GetBalance(ctx, liteUser2.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, liteUserInitialAmount.Equal(balance), fmt.Sprintf("%s user balance (%s) is not expected (%s)", liteUser2.KeyName(), balance, liteUserInitialAmount))
	fmt.Println("Bch user2 balance:", balance)

	memo = fmt.Sprintf("+:%s:%s", "LTC.LTC", "tthor16sg0fxrdd0vgpl4pkcnqwzjlu5lrs6ymcqldel")
	txHash, err = liteChain.SendFundsWithNote(ctx, liteUser.KeyName(), ibc.WalletAmount{
		Address: liteUser2.FormattedAddress(),
		Amount: math.NewInt(100_000_000),
	}, memo)
	require.NoError(t, err)
	fmt.Println("txHash:", txHash)

	balance, err = liteChain.GetBalance(ctx, liteUser2.FormattedAddress(), "")
	require.NoError(t, err)
	fmt.Println("Lite user2 balance after memo tx (should be ~1.001e8):", balance)

	// TODO: Use SendFundsWithNote

	// ------ dogecoin -------
	// Check faucet balance on start
	faucetAddrBz, err = dogeChain.GetAddress(ctx, "faucet")
	require.NoError(t, err)
	faucetAddr = string(faucetAddrBz)
	balance, err = dogeChain.GetBalance(ctx, faucetAddr, "")
	require.NoError(t, err)
	fmt.Println("Doge faucet balance:", balance)

	// Create and fund a user using GetAndFundTestUsers
	dogeUserInitialAmount := math.NewInt(20_900_000_000)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "user", dogeUserInitialAmount, dogeChain)
	dogeUser := users[0]
	fmt.Println("Doge user:", dogeUser.KeyName())

	balance, err = dogeChain.GetBalance(ctx, dogeUser.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, dogeUserInitialAmount.Equal(balance), fmt.Sprintf("%s user balance (%s) is not expected (%s)", dogeUser.KeyName(), balance, dogeUserInitialAmount))
	fmt.Println("Doge user balance:", balance)
	// TODO: Use SendFundsWithNote




	users = interchaintest.GetAndFundTestUsers(t, ctx, "user2", dogeUserInitialAmount, dogeChain)
	dogeUser2 := users[0]
	fmt.Println("Doge user2:", dogeUser2.KeyName())

	err = dogeChain.SendFunds(ctx, dogeUser.KeyName(), ibc.WalletAmount{
		Address: dogeUser2.FormattedAddress(),
		Amount: math.NewInt(200_000_000),
	})
	require.NoError(t, err)
	balance1, err := dogeChain.GetBalance(ctx, dogeUser.FormattedAddress(), "")
	require.NoError(t, err)
	balance2, err := dogeChain.GetBalance(ctx, dogeUser2.FormattedAddress(), "")
	require.NoError(t, err)
	fmt.Println("Doge user1 balance:", balance1, "User2 balance:", balance2)




	dogeUserInitialAmount = math.NewInt(100_000_000)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "user3", dogeUserInitialAmount, dogeChain)
	dogeUser3 := users[0]
	fmt.Println("Doge user3:", dogeUser3.KeyName())

	balance, err = dogeChain.GetBalance(ctx, dogeUser3.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, dogeUserInitialAmount.Equal(balance), fmt.Sprintf("%s user balance (%s) is not expected (%s)", dogeUser3.KeyName(), balance, dogeUserInitialAmount))
	fmt.Println("Doge user2 balance:", balance)

	err = testutil.WaitForBlocks(ctx, 7, dogeChain)
	require.NoError(t, err)

	memo = fmt.Sprintf("+:%s:%s", "DOGE.DOGE", "tthor16sg0fxrdd0vgpl4pkcnqwzjlu5lrs6ymcqldel")
	txHash, err = dogeChain.SendFundsWithNote(ctx, dogeUser.KeyName(), ibc.WalletAmount{
		Address: dogeUser3.FormattedAddress(),
		Amount: math.NewInt(1_000_000_000),
	}, memo)
	require.NoError(t, err)
	fmt.Println("txHash:", txHash)

	balance, err = dogeChain.GetBalance(ctx, dogeUser3.FormattedAddress(), "")
	require.NoError(t, err)
	fmt.Println("Doge user2 balance after memo tx (should be ~11e8):", balance)	

}
