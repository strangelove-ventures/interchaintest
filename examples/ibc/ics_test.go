package ibc_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	stakingttypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	interchaintest "github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// This tests Cosmos Interchain Security, spinning up a provider and a single consumer chain.
func TestICS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	ctx := context.Background()

	vals := 1
	fNodes := 0
	providerChainID := "provider-1"

	// Chain Factory
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name: "ics-provider", Version: "v3.3.0",
			NumValidators: &vals, NumFullNodes: &fNodes,
			ChainConfig: ibc.ChainConfig{GasAdjustment: 1.5, ChainID: providerChainID, TrustingPeriod: "336h"},
		},
		{
			Name: "ics-consumer", Version: "v3.1.0",
			NumValidators: &vals, NumFullNodes: &fNodes,
			ChainConfig: ibc.ChainConfig{GasAdjustment: 1.5, ChainID: "consumer-1"},
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	provider, consumer := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

	// Relayer Factory
	client, network := interchaintest.DockerSetup(t)

	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
	).Build(t, client, network)

	// Prep Interchain
	const ibcPath = "ics-path"
	ic := interchaintest.NewInterchain().
		AddChain(provider).
		AddChain(consumer).
		AddRelayer(r, "relayer").
		AddProviderConsumerLink(interchaintest.ProviderConsumerLink{
			Provider: provider,
			Consumer: consumer,
			Relayer:  r,
			Path:     ibcPath,
		})

	// Log location
	f, err := interchaintest.CreateLogFile(fmt.Sprintf("%d.json", time.Now().Unix()))
	require.NoError(t, err)
	// Reporter/logs
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)

	// Build interchain
	err = ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: false,
	})
	require.NoError(t, err, "failed to build interchain")

	// get provider node
	stakingVals, err := provider.StakingQueryValidators(ctx, stakingttypes.BondStatusBonded)
	require.NoError(t, err)

	providerVal := stakingVals[0]
	denom := provider.Config().Denom

	// Perform validator delegation
	// The delegation must be >1,000,000 utoken as this is = 1 power in tendermint.
	t.Run("perform provider delegation to complete channel to the consumer", func(t *testing.T) {
		beforeDel, err := provider.StakingQueryDelegationsTo(ctx, providerVal.OperatorAddress)
		require.NoError(t, err)

		err = provider.GetNode().StakingDelegate(ctx, "validator", providerVal.OperatorAddress, fmt.Sprintf("1000000%s", denom))
		require.NoError(t, err, "failed to delegate from validator")

		afterDel, err := provider.StakingQueryDelegationsTo(ctx, providerVal.OperatorAddress)
		require.NoError(t, err)
		require.Greater(t, afterDel[0].Balance.Amount.Int64(), beforeDel[0].Balance.Amount.Int64())
	})

	// Flush pending ICS packet
	channels, err := r.GetChannels(ctx, eRep, providerChainID)
	require.NoError(t, err)

	var ICSChannel = ""
	for _, channel := range channels {
		if channel.PortID == "provider" {
			ICSChannel = channel.ChannelID
		}
	}
	require.NoError(t, r.Flush(ctx, eRep, ibcPath, ICSChannel))

	t.Run("validate consumer actions now execute", func(t *testing.T) {
		t.Parallel()
		amt := math.NewInt(1_000_000)
		users := interchaintest.GetAndFundTestUsers(t, ctx, "default", amt, consumer)

		for _, user := range users {
			bal, err := consumer.BankQueryBalance(ctx, user.FormattedAddress(), consumer.Config().Denom)
			require.NoError(t, err)
			require.EqualValues(t, amt, bal)
		}
	})
}
