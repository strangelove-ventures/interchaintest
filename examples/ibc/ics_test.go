package ibc_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/math"
	transfertypes "github.com/cosmos/ibc-go/v7/modules/apps/transfer/types"
	ibcconntypes "github.com/cosmos/ibc-go/v7/modules/core/03-connection/types"
	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// This tests Cosmos Interchain Security, spinning up a provider and a single consumer chain.
func TestICS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	relayers := []relayerImp{
		{
			name:       "Cosmos Relayer",
			relayerImp: ibc.CosmosRly,
		},
		{
			name:       "Hermes",
			relayerImp: ibc.Hermes,
		},
	}

	icsVersions := []string{"v3.1.0", "v3.3.0", "v4.0.0"}

	for _, rly := range relayers {
		rly := rly
		testname := rly.name
		t.Run(testname, func(t *testing.T) {
			// We paralellize the relayers, but not the versions. That would be too many tests running at once, and things can become unstable.
			t.Parallel()
			for _, providerVersion := range icsVersions {
				providerVersion := providerVersion
				for _, consumerVersion := range icsVersions {
					consumerVersion := consumerVersion
					testname := fmt.Sprintf("provider%s-consumer%s", providerVersion, consumerVersion)
					testname = strings.ReplaceAll(testname, ".", "")
					t.Run(testname, func(t *testing.T) {
						fullNodes := 0
						validators := 1
						ctx := context.Background()
						var consumerBechPrefix string
						if consumerVersion == "v4.0.0" {
							consumerBechPrefix = "consumer"
						}

						// Chain Factory
						cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
							{Name: "ics-provider", Version: providerVersion, NumValidators: &validators, NumFullNodes: &fullNodes, ChainConfig: ibc.ChainConfig{GasAdjustment: 1.5}},
							{Name: "ics-consumer", Version: consumerVersion, NumValidators: &validators, NumFullNodes: &fullNodes, ChainConfig: ibc.ChainConfig{Bech32Prefix: consumerBechPrefix}},
						})

						chains, err := cf.Chains(t.Name())
						require.NoError(t, err)
						provider, consumer := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)

						// Relayer Factory
						client, network := interchaintest.DockerSetup(t)

						r := interchaintest.NewBuiltinRelayerFactory(
							rly.relayerImp,
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
							TestName:  t.Name(),
							Client:    client,
							NetworkID: network,

							SkipPathCreation: false,
						})
						require.NoError(t, err, "failed to build interchain")

						// ------------------ ICS Setup ------------------

						// Finish the ICS provider chain initialization.
						// - Restarts the relayer to connect ics20-1 transfer channel
						// - Delegates tokens to the provider to update consensus value
						// - Flushes the IBC state to the consumer
						err = provider.FinishICSProviderSetup(ctx, r, eRep, ibcPath)
						require.NoError(t, err)

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
					})
				}
			}
		})
	}
}

func getTransferChannel(channels []ibc.ChannelOutput) (string, error) {
	for _, channel := range channels {
		if channel.PortID == "transfer" && channel.State == ibcconntypes.OPEN.String() {
			return channel.ChannelID, nil
		}
	}

	return "", fmt.Errorf("no open transfer channel found")
}
