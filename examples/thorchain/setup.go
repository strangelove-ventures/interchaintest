package thorchain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/docker/docker/client"
	ethcommon "github.com/ethereum/go-ethereum/common"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/chain/ethereum"
	tc "github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/chain/utxo"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
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

		if name == "GAIA" {
			exoChains[name].genWallets = BuildGaiaWallets(t, 5, chain.Config())
		}
	}

	ic := interchaintest.NewInterchain()
	for _, chain := range chains {
		name := chain.Config().Name
		var additionalGenesisWallets []ibc.WalletAmount
		for _, wallet := range exoChains[name].genWallets {
			additionalGenesisWallets = append(additionalGenesisWallets, ibc.WalletAmount{
				Address: wallet.FormattedAddress(),
				Amount: sdkmath.NewInt(100_000_000),
				Denom: chain.Config().Denom,
			})
		}
		if name == "GAIA" {
			// this wallet just stops bifrost complaining about it not existing
			additionalGenesisWallets = append(additionalGenesisWallets, ibc.WalletAmount{
				Address: "cosmos1zf3gsk7edzwl9syyefvfhle37cjtql35427vcp",
				Amount: sdkmath.NewInt(1_000_000),
				Denom: chain.Config().Denom,
			})
		}
		ic.AddChain(chain, additionalGenesisWallets...)
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

func StartThorchain(t *testing.T, ctx context.Context, client *client.Client, network string, exoChains ExoChains, ethRouterContractAddress string) *tc.Thorchain {
	numThorchainValidators := 1
	numThorchainFullNodes := 0

	bifrostEnvOverrides := map[string]string{
		"BIFROST_CHAINS_GAIA_BLOCK_SCANNER_START_BLOCK_HEIGHT": "2",
	}
	for _, exoChain := range exoChains {
		name := exoChain.chain.Config().Name
		hostKey := fmt.Sprintf("%s_HOST", name)
		bifrostEnvOverrides[hostKey] = exoChain.chain.GetRPCAddress()
		if name == "GAIA" {
			hostGRPCKey := fmt.Sprintf("%s_GRPC_HOST", name)
			bifrostEnvOverrides[hostGRPCKey] = exoChain.chain.GetGRPCAddress()
		}
		disableChainKey := fmt.Sprintf("BIFROST_CHAINS_%s_DISABLED", name)
		bifrostEnvOverrides[disableChainKey] = "false"
	}
	thorchainChainSpec := ThorchainDefaultChainSpec(t.Name(), numThorchainValidators, numThorchainFullNodes, ethRouterContractAddress, nil, bifrostEnvOverrides)

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

	err = thorchain.StartAllValSidecars(ctx)
	require.NoError(t, err, "failed starting validator sidecars")

	// Give some time for bifrost to initialize before any tests start
	err = testutil.WaitForBlocks(ctx, 5, thorchain)
	require.NoError(t, err)

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

	return ethRouterContractAddress
}

func SetupGaia(t *testing.T, ctx context.Context, exoChain *ExoChain) *errgroup.Group {
	gaia := exoChain.chain.(*cosmos.CosmosChain)
	
	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		for _, genWallet := range exoChain.genWallets {
			err := gaia.RecoverKey(egCtx, genWallet.KeyName(), genWallet.Mnemonic())
			if err != nil {
				return err
			}
		}
	
		amount := ibc.WalletAmount{
			Denom:  gaia.Config().Denom,
			Amount: sdkmath.NewInt(1_000_000),
		}
	
		// Send 100 txs on gaia so that bifrost can automatically set the network fee
		// Sim testing can directly use bifrost to do this, right now, we can't, but may in the future
		val0 := gaia.GetNode()
		for i := 0; i < 100/len(exoChain.genWallets) + 1; i++ {
			for j, genWallet := range exoChain.genWallets {
				toUser := exoChain.genWallets[(j+1)%len(exoChain.genWallets)]
				go sendFunds(ctx, genWallet.KeyName(), toUser.FormattedAddress(), amount, val0)
			}
			err := testutil.WaitForBlocks(ctx, 1, gaia)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return eg
}

func BuildGaiaWallets(t *testing.T, numWallets int, cfg ibc.ChainConfig) []ibc.Wallet {
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	kr := keyring.NewInMemory(cdc)

	gaiaWallets := make([]ibc.Wallet, numWallets)
	for i := 0; i < numWallets; i++ {
		keyName := fmt.Sprintf("tx100_%d", i)
		record, mnemonic, err := kr.NewMnemonic(
			keyName,
			keyring.English,
			hd.CreateHDPath(118, 0, 0).String(),
			"", // Empty passphrase.
			hd.Secp256k1,
		)
		require.NoError(t, err)

		addrBytes, err := record.GetAddress()
		require.NoError(t, err)

		gaiaWallets[i] = cosmos.NewWallet(keyName, addrBytes, mnemonic, cfg)
	}

	return gaiaWallets
}