package stride_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v3/modules/apps/transfer/types"
	"github.com/strangelove-ventures/ibctest/v3"
	"github.com/strangelove-ventures/ibctest/v3/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v3/examples/stride"
	"github.com/strangelove-ventures/ibctest/v3/ibc"
	"github.com/strangelove-ventures/ibctest/v3/internal/dockerutil"
	"github.com/strangelove-ventures/ibctest/v3/relayer"
	"github.com/strangelove-ventures/ibctest/v3/test"
	"github.com/strangelove-ventures/ibctest/v3/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"
)

// TestStrideICAandICQ is a test case that performs simulations and assertions around interchain accounts
// and the client implementation of interchain queries. See: https://github.com/Stride-Labs/interchain-queries
func TestStrideRedemptions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := ibctest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	// Define chains involved in test
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			Name:      "stride",
			ChainName: "stride",
			ChainConfig: ibc.ChainConfig{
				Type:    "cosmos",
				Name:    "stride",
				ChainID: "stride-1",
				Images: []ibc.DockerImage{{
					Repository: "ghcr.io/strangelove-ventures/heighliner/stride",
					Version:    "andrew-test_admin",
					UidGid:     dockerutil.GetHeighlinerUserString(),
				}},
				Bin:            "strided",
				Bech32Prefix:   "stride",
				Denom:          "ustrd",
				GasPrices:      "0.0ustrd",
				TrustingPeriod: TrustingPeriod,
				GasAdjustment:  1.1,
				ModifyGenesis:  ModifyGenesisStride(),
				EncodingConfig: stride.StrideEncoding(),
			}},
		{
			Name:      "gaia",
			ChainName: "gaia",
			Version:   "v7.0.3",
			ChainConfig: ibc.ChainConfig{
				ModifyGenesis:  ModifyGenesisStrideCounterparty(),
				TrustingPeriod: TrustingPeriod,
			},
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	stride, gaia := chains[0].(*cosmos.CosmosChain), chains[1].(*cosmos.CosmosChain)
	strideCfg, gaiaCfg := stride.Config(), gaia.Config()

	// Get a relayer instance
	r := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.CustomDockerImage("ghcr.io/cosmos/relayer", "andrew-client_icq", "100:1000"),
		relayer.StartupFlags("-p", "events"),
	).Build(t, client, network)

	// Build the network; spin up the chains and configure the relayer
	const pathStrideGaia = "stride-gaia"
	const relayerName = "relayer"

	clientOpts := ibc.DefaultClientOpts()
	clientOpts.TrustingPeriod = TrustingPeriod

	ic := ibctest.NewInterchain().
		AddChain(stride).
		AddChain(gaia).
		AddRelayer(r, relayerName).
		AddLink(ibctest.InterchainLink{
			Chain1:           stride,
			Chain2:           gaia,
			Relayer:          r,
			Path:             pathStrideGaia,
			CreateClientOpts: clientOpts,
		})

	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Fund user accounts, so we can query balances and make assertions.
	const userFunds = int64(10_000_000_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, stride, gaia)
	strideUser, gaiaUser := users[0], users[1]

	strideFullNode := stride.FullNodes[0]

	// Wait a few blocks for user accounts to be created on chain.
	err = test.WaitForBlocks(ctx, 2, stride, gaia)
	require.NoError(t, err)

	// Start the relayers
	err = r.StartRelayer(ctx, eRep, pathStrideGaia)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occured while stopping the relayer: %s", err)
			}
		},
	)

	// Wait a few blocks for the relayer to start.
	err = test.WaitForBlocks(ctx, 2, stride, gaia)
	require.NoError(t, err)

	// Recover stride admin key
	err = stride.RecoverKey(ctx, StrideAdminAccount, StrideAdminMnemonic)
	require.NoError(t, err)

	strideAdminAddrBytes, err := stride.GetAddress(ctx, StrideAdminAccount)
	require.NoError(t, err)

	strideAdminAddr, err := types.Bech32ifyAddressBytes(strideCfg.Bech32Prefix, strideAdminAddrBytes)
	require.NoError(t, err)

	err = stride.SendFunds(ctx, ibctest.FaucetAccountKeyName, ibc.WalletAmount{
		Address: strideAdminAddr,
		Amount:  userFunds,
		Denom:   strideCfg.Denom,
	})
	require.NoError(t, err, "failed to fund stride admin account")

	// get native chain user addresses
	strideAddr := strideUser.Bech32Address(strideCfg.Bech32Prefix)
	require.NotEmpty(t, strideAddr)

	gaiaAddress := gaiaUser.Bech32Address(gaiaCfg.Bech32Prefix)
	require.NotEmpty(t, gaiaAddress)

	// get ibc paths
	gaiaConns, err := r.GetConnections(ctx, eRep, gaiaCfg.ChainID)
	require.NoError(t, err)

	gaiaChans, err := r.GetChannels(ctx, eRep, gaiaCfg.ChainID)
	require.NoError(t, err)

	atomIBCDenom := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(gaiaChans[0].Counterparty.PortID, gaiaChans[0].Counterparty.ChannelID, gaiaCfg.Denom)).IBCDenom()

	var eg errgroup.Group

	// Fund stride user with ibc transfers in parallel
	eg.Go(func() error {
		gaiaHeight, err := gaia.Height(ctx)
		if err != nil {
			return err
		}
		// Fund stride user with ibc denom atom
		tx, err := gaia.SendIBCTransfer(ctx, gaiaChans[0].ChannelID, gaiaUser.KeyName, ibc.WalletAmount{
			Amount:  1_000_000_000_000,
			Denom:   gaiaCfg.Denom,
			Address: strideAddr,
		}, nil)
		if err != nil {
			return err
		}
		_, err = test.PollForAck(ctx, gaia, gaiaHeight, gaiaHeight+10, tx.Packet)
		return err
	})

	require.NoError(t, eg.Wait())

	// Register gaia host zone
	_, err = strideFullNode.ExecTx(ctx, StrideAdminAccount,
		"stakeibc", "register-host-zone",
		gaiaConns[0].Counterparty.ConnectionId, gaiaCfg.Denom, gaiaCfg.Bech32Prefix,
		atomIBCDenom, gaiaChans[0].Counterparty.ChannelID, "1",
		"--gas", "1000000",
	)
	require.NoError(t, err)

	// TODO: replace with poll for channel open confirm messages
	// Wait a few blocks for the ICA accounts to be setup
	err = test.WaitForBlocks(ctx, 15, stride, gaia)
	require.NoError(t, err)

	// Get validator addresses
	gaiaVal1Address, err := gaia.Validators[0].KeyBech32(ctx, "validator", "val")
	require.NoError(t, err)

	gaiaVal2Address, err := gaia.Validators[1].KeyBech32(ctx, "validator", "val")
	require.NoError(t, err)

	// Add gaia validator 1
	_, err = strideFullNode.ExecTx(ctx, StrideAdminAccount,
		"stakeibc", "add-validator",
		gaiaCfg.ChainID, "gval1", gaiaVal1Address,
		"10", "5",
	)
	require.NoError(t, err)

	// Add gaia validator 2
	_, err = strideFullNode.ExecTx(ctx, StrideAdminAccount,
		"stakeibc", "add-validator",
		gaiaCfg.ChainID, "gval2", gaiaVal2Address,
		"10", "10",
	)
	require.NoError(t, err)

	var gaiaHostZone HostZoneWrapper

	// query gaia host zone
	stdout, _, err := strideFullNode.ExecQuery(ctx,
		"stakeibc", "show-host-zone", gaiaCfg.ChainID,
	)
	require.NoError(t, err)
	err = json.Unmarshal(stdout, &gaiaHostZone)
	require.NoError(t, err)

	// Liquid stake some atom
	res, err := strideFullNode.ExecTx(ctx, strideUser.KeyName,
		"stakeibc", "liquid-stake",
		"10000", gaiaCfg.Denom,
	)
	require.NoError(t, err)
	fmt.Println("RESPONSE FROM LIQUID STAKE")
	fmt.Println(res)

	// query deposit records
	stdout, _, err = strideFullNode.ExecQuery(ctx,
		"records", "list-deposit-record",
	)
	require.NoError(t, err)
	fmt.Print(string(stdout))

	// err = json.Unmarshal(stdout, &depositRecords)
	// require.NoError(t, err)
}
