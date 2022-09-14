package osmosis_test

import (
	"context"
	"fmt"
	"testing"

	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	ibctest "github.com/strangelove-ventures/ibctest/v3"
	"github.com/strangelove-ventures/ibctest/v3/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v3/examples/osmosis"
	"github.com/strangelove-ventures/ibctest/v3/ibc"
	"github.com/strangelove-ventures/ibctest/v3/relayer"
	"github.com/strangelove-ventures/ibctest/v3/test"
	"github.com/strangelove-ventures/ibctest/v3/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestOsmosisGammPool(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			Name:      "osmosis",
			ChainName: "osmosis",
			Version:   "main",
			ChainConfig: ibc.ChainConfig{
				ChainID:        "osmosis-1001", // hardcoded handling in osmosis binary for osmosis-1, so need to override to something different.
				EncodingConfig: osmosis.OsmosisEncoding(),
			},
		},
		{
			Name:      "gaia",
			ChainName: "gaia",
			Version:   "v7.0.3",
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	client, network := ibctest.DockerSetup(t)

	chain, counterpartyChain := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	const (
		path        = "create-pool-test-path"
		relayerName = "relayer"
	)

	// Get a relayer instance
	rf := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.StartupFlags("-p", "events"),
	)

	r := rf.Build(t, client, network)

	ic := ibctest.NewInterchain().
		AddChain(chain).
		AddChain(counterpartyChain).
		AddRelayer(r, relayerName).
		AddLink(ibctest.InterchainLink{
			Chain1:  chain,
			Chain2:  counterpartyChain,
			Relayer: r,
			Path:    path,
		})

	ctx := context.Background()

	rep := testreporter.NewNopReporter().RelayerExecReporter(t)

	require.NoError(t, ic.Build(ctx, rep, ibctest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	channels, err := r.GetChannels(ctx, rep, chain.Config().ChainID)
	require.NoError(t, err)

	require.NoError(t, r.StartRelayer(ctx, rep, path))
	t.Cleanup(func() {
		_ = r.StopRelayer(ctx, rep)
	})

	const userFunds = int64(10_000_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain, counterpartyChain)
	chainUser, counterpartyChainUser := users[0], users[1]

	counterpartyHeight, err := counterpartyChain.Height(ctx)
	require.NoError(t, err)

	chainUserBech32 := chainUser.Bech32Address(chain.Config().Bech32Prefix)
	counterpartyUserBech32 := counterpartyChainUser.Bech32Address(counterpartyChain.Config().Bech32Prefix)
	chainDenom := chain.Config().Denom
	counterpartyDenom := counterpartyChain.Config().Denom

	// send an IBC transfer from counterparty chain to chain user.
	counterpartyAmountSent := int64(1_000_000)
	tx, err := counterpartyChain.SendIBCTransfer(ctx, channels[0].Counterparty.ChannelID, counterpartyChainUser.KeyName, ibc.WalletAmount{
		Address: chainUserBech32,
		Amount:  counterpartyAmountSent,
		Denom:   counterpartyDenom,
	}, nil)
	require.NoError(t, err)

	// will use this later for balance assertions
	counterpartyIBCTransferFees := counterpartyChain.GetGasFeesInNativeDenom(tx.GasSpent)

	// wait for packet flow to complete.
	_, err = test.PollForAck(ctx, counterpartyChain, counterpartyHeight, counterpartyHeight+10, tx.Packet)
	require.NoError(t, err)

	// get ibc denom for counterparty denom on chain.
	denomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(channels[0].PortID, channels[0].ChannelID, counterpartyDenom))
	ibcDenom := denomTrace.IBCDenom()

	// create liquidity pool for pair of counterparty denom (as IBC denom on chain) and native chain denom.
	poolID, err := chain.CreatePool(ctx, chainUser.KeyName, ibc.PoolParams{
		Weights:        fmt.Sprintf("5%s,5%s", ibcDenom, chainDenom),
		InitialDeposit: fmt.Sprintf("499404%s,500000%s", ibcDenom, chainDenom),
		SwapFee:        "0.01",
		ExitFee:        "0.01",
		FutureGovernor: "",
	})
	require.NoError(t, err)
	require.Equal(t, poolID, "1")

	nativeDenomBalancePostCreatePool, err := chain.GetBalance(ctx, chainUserBech32, chainDenom)
	require.NoError(t, err)

	// 10_000_000_000 initial - 1_000_000_000 create pool cost - 500_000 deposit.
	require.Equal(t, int64(8_999_500_000), nativeDenomBalancePostCreatePool)

	// execute swap in liquidity pool, exchange IBC denom for native denom.
	_, err = chain.SwapExactAmountIn(ctx, chainUser.KeyName, fmt.Sprintf("50000%s", ibcDenom), "45000", []string{poolID}, []string{chainDenom})
	require.NoError(t, err, "failed to swap ibc denom for native denom")

	nativeDenomBalance, err := chain.GetBalance(ctx, chainUserBech32, chainDenom)
	require.NoError(t, err)

	// plus 45_089 swap_amount
	require.Equal(t, nativeDenomBalancePostCreatePool+45_089, nativeDenomBalance)

	ibcDenomBalance, err := chain.GetBalance(ctx, chainUserBech32, ibcDenom)
	require.NoError(t, err, "failed to get balance of ibc denom")

	// 1_000_000 initial IBC transfer minus 499_404 initial deposit minus 50_000 swapped for native denom
	require.Equal(t, int64(450596), ibcDenomBalance)

	chainHeight, err := chain.Height(ctx)
	require.NoError(t, err)

	// send leftover back to counterparty chain
	tx, err = chain.SendIBCTransfer(ctx, channels[0].ChannelID, chainUser.KeyName, ibc.WalletAmount{
		Address: counterpartyUserBech32,
		Amount:  ibcDenomBalance,
		Denom:   ibcDenom,
	}, nil)
	require.NoError(t, err)

	_, err = test.PollForAck(ctx, chain, chainHeight, chainHeight+10, tx.Packet)
	require.NoError(t, err)

	ibcDenomBalancePostReturn, err := chain.GetBalance(ctx, chainUserBech32, ibcDenom)
	require.NoError(t, err, "failed to get balance of ibc denom")

	// should have sent it all to back to counterparty
	require.Equal(t, int64(0), ibcDenomBalancePostReturn)

	counterpartyNativeBalance, err := counterpartyChain.GetBalance(ctx, counterpartyUserBech32, counterpartyDenom)
	require.NoError(t, err, "failed to get balance of native denom on counterparty")

	require.Equal(t, userFunds-counterpartyAmountSent-counterpartyIBCTransferFees+ibcDenomBalance, counterpartyNativeBalance)
}
