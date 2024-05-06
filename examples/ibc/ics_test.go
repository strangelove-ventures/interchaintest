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
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
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

	// err = testutil.WaitForBlocks(ctx, 5, provider, consumer)
	// require.NoError(t, err, "failed to wait for blocks")

	// get provider node
	stakingVals, err := provider.StakingQueryValidators(ctx, stakingttypes.BondStatusBonded)
	require.NoError(t, err)
	valOne := stakingVals[0]
	denom := provider.Config().Denom

	// delegate from the "validator" key
	err = provider.GetNode().StakingDelegate(ctx, "validator", valOne.OperatorAddress, fmt.Sprintf("100%s", denom))
	require.NoError(t, err, "failed to delegate from validator")

	// now wait for the relayer event to be processes

	// relayer flush
	channels, err := r.GetChannels(ctx, eRep, providerChainID)
	require.NoError(t, err)
	// require.Len(t, channels, 2)

	// get the ics20-1 transfer channel
	var transferChannel = ""
	for _, channel := range channels {
		fmt.Printf("channel: %v\n", channel)
		// TODO: this is stuck in TRYOPEN state? channel: {STATE_TRYOPEN ORDER_UNORDERED {transfer channel-1} [connection-0] ics20-1 transfer channel-1}
		if channel.Version == "ics20-1" {
			transferChannel = channel.ChannelID
		}
	}
	require.NotEmpty(t, transferChannel)

	require.NoError(t, r.Flush(ctx, eRep, ibcPath, transferChannel))

	err = testutil.WaitForBlocks(ctx, 15, provider, consumer)
	require.NoError(t, err, "failed to wait for blocks")

	// perform an action on the consumer
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", math.NewInt(10_000_000), consumer)
	fmt.Printf("users: %v\n", users)
	// get balance & print
	for _, user := range users {
		bal, err := consumer.BankQueryBalance(ctx, user.FormattedAddress(), consumer.Config().Denom)
		require.NoError(t, err)
		fmt.Printf("user: %v, balance: %v\n", user.FormattedAddress(), bal)
	}

	err = testutil.WaitForBlocks(ctx, 1000, provider, consumer)
	require.NoError(t, err, "failed to wait for blocks")

	// perform ibc transfer here from provider -> consumer
}
