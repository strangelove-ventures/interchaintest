package ethereum_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/ethereum"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"

	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestEthereum(t *testing.T) {

	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)

	// Log location
	f, err := interchaintest.CreateLogFile(fmt.Sprintf("%d.json", time.Now().Unix()))
	require.NoError(t, err)
	// Reporter/logs
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	// Get default ethereum chain config for anvil
	anvilConfig := ethereum.DefaultEthereumAnvilChainConfig("ethereum")

	// add --load-state config (this step is not required for tests that don't require an existing state)
	configFileOverrides := make(map[string]any)
	configFileOverrides["--load-state"] = "eigenlayer-deployed-anvil-state.json" // Relative path of state.json
	anvilConfig.ConfigFileOverrides = configFileOverrides

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			ChainName: "ethereum", 
			Name: "ethereum",
			Version: "latest",
			ChainConfig: anvilConfig,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	ethereumChain := chains[0].(*ethereum.EthereumChain)

	ic := interchaintest.NewInterchain().
		AddChain(ethereumChain)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  true, // Skip path creation, so we can have granular control over the process
	}))
	fmt.Println("Interchain built")

	// Check faucet balance on start
	faucetAddr := "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	balance, err := ethereumChain.GetBalance(ctx, faucetAddr, "")
	require.NoError(t, err)
	expectedFaucetInitialBalance, ok := math.NewIntFromString("9999910437528191621644") // Balance from the state.json
	require.True(t, ok)
	require.True(t, expectedFaucetInitialBalance.Equal(balance))

	// Create and fund a user using GetAndFundTestUsers
	ethUserInitialAmount := math.NewInt(2 * ethereum.ETHER)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", ethUserInitialAmount, ethereumChain)
	ethUser := users[0]

	// Check balances of faucet and then user1
	balance, err = ethereumChain.GetBalance(ctx, faucetAddr, "")
	require.NoError(t, err)
	require.True(t, balance.LTE(expectedFaucetInitialBalance.Sub(ethUserInitialAmount)))

	balance, err = ethereumChain.GetBalance(ctx, ethUser.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, ethUserInitialAmount.Equal(balance))

	// Create user2 wallet and check balance
	ethUser2, err := ethereumChain.BuildWallet(ctx, "ethUser2", "")
	require.NoError(t, err)

	balance, err = ethereumChain.GetBalance(ctx, ethUser2.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, math.ZeroInt().Equal(balance))

	
	// Fund user2 wallet using SendFunds() from user1 wallet
	ethUser2InitialAmount := math.NewInt(ethereum.ETHER)
	ethereumChain.SendFunds(ctx, ethUser.KeyName(), ibc.WalletAmount{
		Address: ethUser2.FormattedAddress(),
		Denom: ethereumChain.Config().Denom,
		Amount: ethUser2InitialAmount,
	})


	// Final check of balances
	balance, err = ethereumChain.GetBalance(ctx, faucetAddr, "")
	require.NoError(t, err)
	require.True(t, balance.LTE(expectedFaucetInitialBalance.Sub(ethUserInitialAmount)))

	balance, err = ethereumChain.GetBalance(ctx, ethUser.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, balance.LTE(ethUserInitialAmount.Sub(ethUser2InitialAmount)))

	balance, err = ethereumChain.GetBalance(ctx, ethUser2.FormattedAddress(), "")
	require.NoError(t, err)
	require.True(t, balance.Equal(ethUser2InitialAmount))

	// Sleep for an additional testing
	time.Sleep(10 * time.Second)

}