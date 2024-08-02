package thorchain_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"cosmossdk.io/math"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/chain/ethereum"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v8/examples/thorchain/features"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestThorchain(t *testing.T) {
	numThorchainValidators := 1
	numThorchainFullNodes  := 0

	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	// ------------------------
	// Setup EVM chains first
	// ------------------------
	ethChainName := common.ETHChain.String() // must use this name for test
	cf0 := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			ChainName:   ethChainName,
			Name:        ethChainName,
			Version:     "latest",
			ChainConfig: ethereum.DefaultEthereumAnvilChainConfig(ethChainName),
		},
	})

	chains, err := cf0.Chains(t.Name())
	require.NoError(t, err)
	ethChain := chains[0].(*ethereum.EthereumChain)

	ic0 := interchaintest.NewInterchain().
		AddChain(ethChain)
	
	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	require.NoError(t, ic0.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic0.Close()
	})

	ethUserInitialAmount := math.NewInt(2 * ethereum.ETHER)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "user", ethUserInitialAmount, ethChain)
	ethUser := users[0]

	ethChain.SendFunds(ctx, "faucet", ibc.WalletAmount{
		Address: "0x1804c8ab1f12e6bbf3894d4083f33e07309d1f38",
		Amount: math.NewInt(ethereum.ETHER),
	})

	os.Setenv("ETHFAUCET", "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	stdout, _, err := ethChain.ForgeScript(ctx, ethUser.KeyName(), ethereum.ForgeScriptOpts{
		ContractRootDir: "contracts",
		SolidityContract: "script/Token.s.sol",
		RawOptions:       []string{"--sender", ethUser.FormattedAddress(), "--json"},
	})
	require.NoError(t, err)

	tokenContractAddress, err := GetEthAddressFromStdout(string(stdout))
	require.NoError(t, err)
	require.NotEmpty(t, tokenContractAddress)
	require.True(t, ethcommon.IsHexAddress(tokenContractAddress))

	fmt.Println("Token contract address:", tokenContractAddress)

	stdout, _, err = ethChain.ForgeScript(ctx, ethUser.KeyName(), ethereum.ForgeScriptOpts{
		ContractRootDir: "contracts",
		SolidityContract: "script/Router.s.sol",
		RawOptions:       []string{"--sender", ethUser.FormattedAddress(), "--json"},
	})
	require.NoError(t, err)

	ethRouterContractAddress, err := GetEthAddressFromStdout(string(stdout))
	require.NoError(t, err)
	require.NotEmpty(t, ethRouterContractAddress)
	require.True(t, ethcommon.IsHexAddress(ethRouterContractAddress))

	fmt.Println("Router contract address:", ethRouterContractAddress)


	// ----------------------------
	// Set up thorchain and others
	// ----------------------------
	thorchainChainSpec := ThorchainDefaultChainSpec(t.Name(), numThorchainValidators, numThorchainFullNodes, ethRouterContractAddress)
	// TODO: add router contracts to thorchain
	// Set ethereum RPC
	// Move other chains to above for setup too?
	//thorchainChainSpec.
	chainSpecs := []*interchaintest.ChainSpec{
		thorchainChainSpec,
		GaiaChainSpec(),
	}

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), chainSpecs)

	chains, err = cf.Chains(t.Name())
	require.NoError(t, err)

	thorchain := chains[0].(*tc.Thorchain)
	gaia := chains[1].(*cosmos.CosmosChain)

	ic := interchaintest.NewInterchain().
		AddChain(thorchain).
		AddChain(gaia)
	
	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	err = thorchain.StartAllValSidecars(ctx)
	require.NoError(t, err, "failed starting validator sidecars")

	doTxs(t, ctx, gaia) // Do 100 transactions

	defaultFundAmount := math.NewInt(100_000_000)
	users = interchaintest.GetAndFundTestUsers(t, ctx, "default", defaultFundAmount, thorchain)
	thorchainUser := users[0]
	err = testutil.WaitForBlocks(ctx, 2, thorchain)
	require.NoError(t, err, "thorchain failed to make blocks")

	// --------------------------------------------------------
	// Check balances are correct
	// --------------------------------------------------------
	thorchainUserAmount, err := thorchain.GetBalance(ctx, thorchainUser.FormattedAddress(), thorchain.Config().Denom)
	require.NoError(t, err)
	require.True(t, thorchainUserAmount.Equal(defaultFundAmount), "Initial thorchain user amount not expected")
	
	val0 := thorchain.GetNode()
	faucetAddr, err := val0.AccountKeyBech32(ctx, "faucet")
	require.NoError(t, err)
	faucetAmount, err := thorchain.GetBalance(ctx, faucetAddr, thorchain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, InitialFaucetAmount.Sub(defaultFundAmount).Sub(StaticGas), faucetAmount)

	// TODO: Run each stage in parallel

	// --------------------------------------------------------
	// Bootstrap pool
	// --------------------------------------------------------
	_, gaiaLper := features.DualLp(t, ctx, thorchain, gaia)
	_, _ = features.DualLp(t, ctx, thorchain, ethChain)

	// --------------------------------------------------------
	// Savers
	// --------------------------------------------------------
	gaiaSaver := features.Saver(t, ctx, thorchain, gaia)
	_ = features.Saver(t, ctx, thorchain, ethChain)
	
	// --------------------------------------------------------
	// Arb
	// --------------------------------------------------------
	_, err = features.Arb(t, ctx, thorchain, gaia, ethChain) // Must add all active chains
	require.NoError(t, err)
	
	// --------------------------------------------------------
	// Swap
	// --------------------------------------------------------
	err = features.SingleSwap(t, ctx, thorchain, gaia, thorchain)
	require.NoError(t, err)
	
	// --------------------------------------------------------
	// Saver Eject
	// --------------------------------------------------------
	gaiaSaverBalance, err := gaia.GetBalance(ctx, gaiaSaver.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	
	_ = features.SaverEject(t, ctx, thorchain, gaia)
	
	gaiaSaverAfterBalance, err := gaia.GetBalance(ctx, gaiaSaver.FormattedAddress(), gaia.Config().Denom)
	require.NoError(t, err)
	require.True(t, gaiaSaverBalance.Equal(gaiaSaverAfterBalance), fmt.Sprintf("Balance (%s) should be the same (%s)", gaiaSaverAfterBalance, gaiaSaverBalance))

	// --------------------------------------------------------
	// Ragnarok gaia
	// --------------------------------------------------------
	err = features.Ragnarok(t, ctx, thorchain, gaia, gaiaLper, gaiaSaver)
	
	//err = gaia.StopAllNodes(ctx)
	//require.NoError(t, err)

	//state, err := gaia.ExportState(ctx, -1)
	//require.NoError(t, err)
	//fmt.Println("State: ", state)


	//err = testutil.WaitForBlocks(ctx, 300, thorchain)
	//require.NoError(t, err, "thorchain failed to make blocks")
}
