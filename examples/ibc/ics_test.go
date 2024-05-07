package ibc_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	ibcconntypes "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	interchaintest "github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
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
		relayer.StartupFlags("--block-history", "100"),
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

	// Reporter/logs
	f, err := interchaintest.CreateLogFile(fmt.Sprintf("%s_%d.json", t.Name(), time.Now().Unix()))
	require.NoError(t, err)
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

	// ------------------ ICS Setup ------------------

	// Finish the ICS provider chain initialization.
	// - Restarts the relayer to connect ics20-1 transfer channel
	// - Delegates tokens to the provider to update consensus value
	// - Flushes the IBC state to the consumer
	provider.FinishICSProviderSetup(t, ctx, r, eRep, ibcPath)

	// ------------------ Test Begins ------------------

	// Fund users
	// NOTE: this has to be done after the provider delegation & IBC update to the consumer.
	amt := math.NewInt(10_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", amt, consumer, provider)
	consumerUser, providerUser := users[0], users[1]

	t.Run("validate consumer action executed", func(t *testing.T) {
		bal, err := consumer.BankQueryBalance(ctx, consumerUser.FormattedAddress(), consumer.Config().Denom)
		require.NoError(t, err)
		require.EqualValues(t, amt, bal)
	})

	t.Run("provider -> consumer IBC transfer", func(t *testing.T) {
		providerChannelInfo, err := r.GetChannels(ctx, eRep, provider.Config().ChainID)
		require.NoError(t, err)

		channelID, err := getTransferChannel(providerChannelInfo)
		require.NoError(t, err, providerChannelInfo)

		consumerChannelInfo, err := r.GetChannels(ctx, eRep, consumer.Config().ChainID)
		require.NoError(t, err)

		consumerChannelID, err := getTransferChannel(consumerChannelInfo)
		require.NoError(t, err, consumerChannelInfo)

		dstAddress := consumerUser.FormattedAddress()
		sendAmt := math.NewInt(7)
		transfer := ibc.WalletAmount{
			Address: dstAddress,
			Denom:   provider.Config().Denom,
			Amount:  sendAmt,
		}

		tx, err := provider.SendIBCTransfer(ctx, channelID, providerUser.KeyName(), transfer, ibc.TransferOptions{})
		require.NoError(t, err)
		require.NoError(t, tx.Validate())

		require.NoError(t, r.Flush(ctx, eRep, ibcPath, channelID))

		srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom("transfer", consumerChannelID, provider.Config().Denom))
		dstIbcDenom := srcDenomTrace.IBCDenom()

		consumerBal, err := consumer.BankQueryBalance(ctx, consumerUser.FormattedAddress(), dstIbcDenom)
		require.NoError(t, err)
		require.EqualValues(t, sendAmt, consumerBal)
	})
}

func getTransferChannel(channels []ibc.ChannelOutput) (string, error) {
	for _, channel := range channels {
		if channel.PortID == "transfer" && channel.State == ibcconntypes.OPEN.String() {
			return channel.ChannelID, nil
		}
	}

	return "", fmt.Errorf("no open transfer channel found")
}
