package thorchain_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/docker/docker/client"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/chain/ethereum"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/utxo"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func StartExoChains(t *testing.T, ctx context.Context, client *client.Client, network string) ExoChains {
	chainSpecs := []*interchaintest.ChainSpec{
		EthChainSpec(),
		GaiaChainSpec(),
		BtcChainSpec(),
		BchChainSpec(),
		LtcChainSpec(),
		DogeChainSpec(),
	}
	cf0 := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), chainSpecs)

	chains, err := cf0.Chains(t.Name())
	require.NoError(t, err)

	exoChains := make(map[string]*ExoChain, len(chains))
	for _, chain := range chains {
		name := chain.Config().Name
		exoChains[name] = &ExoChain{
			chain: chain,
		}

		if name == "BTC" || name == "BCH" || name == "LTC" {
			utxoChain := chain.(*utxo.UtxoChain)
			utxoChain.UnloadWalletAfterUse(true)
		}
	}

	ic := interchaintest.NewInterchain()
	for _, chain := range chains {
		ic.AddChain(chain)
	}

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	return exoChains
}

func StartThorchain(t *testing.T, ctx context.Context, client *client.Client, network string, ethRouterContractAddress string) *tc.Thorchain {
	numThorchainValidators := 1
	numThorchainFullNodes := 0

	thorchainChainSpec := ThorchainDefaultChainSpec(t.Name(), numThorchainValidators, numThorchainFullNodes, ethRouterContractAddress)
	// TODO: add router contracts to thorchain

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		thorchainChainSpec,
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	thorchain := chains[0].(*tc.Thorchain)

	ic := interchaintest.NewInterchain().
		AddChain(thorchain)

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// TODO: make sure bifrost starts at gaia block 1
	err = thorchain.StartAllValSidecars(ctx)
	require.NoError(t, err, "failed starting validator sidecars")

	return thorchain
}

func SetupEthContracts(t *testing.T, ctx context.Context, exoChain *ExoChain) string {
	ethChain := exoChain.chain.(*ethereum.EthereumChain)

	ethUserInitialAmount := ethereum.ETHER.MulRaw(2)

	ethUser, err := interchaintest.GetAndFundTestUserWithMnemonic(ctx, "user", strings.Repeat("dog ", 23)+"fossil", ethUserInitialAmount, ethChain)
	require.NoError(t, err)

	stdout, _, err := ethChain.ForgeScript(ctx, ethUser.KeyName(), ethereum.ForgeScriptOpts{
		ContractRootDir:  "contracts",
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
		ContractRootDir:  "contracts",
		SolidityContract: "script/Router.s.sol",
		RawOptions:       []string{"--sender", ethUser.FormattedAddress(), "--json"},
	})
	require.NoError(t, err)

	ethRouterContractAddress, err := GetEthAddressFromStdout(string(stdout))
	require.NoError(t, err)
	require.NotEmpty(t, ethRouterContractAddress)
	require.True(t, ethcommon.IsHexAddress(ethRouterContractAddress))

	fmt.Println("Router contract address:", ethRouterContractAddress)

	return ethRouterContractAddress
}

func SetupGaia(t *testing.T, ctx context.Context, exoChain *ExoChain) *sync.WaitGroup {
	gaia := exoChain.chain.(*cosmos.CosmosChain)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		err := gaia.SendFunds(ctx, "faucet", ibc.WalletAmount{
			Address: "cosmos1zf3gsk7edzwl9syyefvfhle37cjtql35427vcp",
			Denom:   gaia.Config().Denom,
			Amount:  sdkmath.NewInt(10000000),
		})
		require.NoError(t, err)

		doTxs(t, ctx, gaia) // Do 100 transactions
		wg.Done()
	}()

	return wg
}
