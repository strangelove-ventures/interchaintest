package ibctest

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/icza/dyno"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/chain/tendermint"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/relayer"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestInterchainQueries(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := ibctest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	dockerImage := ibc.DockerImage{
		Repository: "ghcr.io/strangelove-ventures/heighliner/icqd",
		Version:    "latest",
	}

	// Get both chains
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			ChainName: "test-1",
			ChainConfig: ibc.ChainConfig{
				Type:           "cosmos",
				Name:           "icq",
				ChainID:        "test-1",
				Images:         []ibc.DockerImage{dockerImage},
				Bin:            "icq",
				Bech32Prefix:   "cosmos",
				Denom:          "uatom",
				GasPrices:      "0.00stake",
				TrustingPeriod: "300h",
				GasAdjustment:  1.1,
			}},
		{
			ChainName: "test-2",
			ChainConfig: ibc.ChainConfig{
				Type:           "cosmos",
				Name:           "icq",
				ChainID:        "test-2",
				Images:         []ibc.DockerImage{dockerImage},
				Bin:            "icq",
				Bech32Prefix:   "cosmos",
				Denom:          "uatom",
				GasPrices:      "0.00stake",
				TrustingPeriod: "300h",
				GasAdjustment:  1.1,
				ModifyGenesis:  modifyGenesisAllowICQQueries([]string{"/cosmos.bank.v1beta1.Query/AllBalances"}),
			}},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain1, chain2 := chains[0], chains[1]

	// Get a relayer instance
	r := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.RelayerOptionExtraStartFlags{Flags: []string{"-p", "events", "-b", "100"}},
	).Build(t, client, network)

	// Build the network; spin up the chains and configure the relayer
	const pathName = "test1-test2"
	const relayerName = "relayer"

	ic := ibctest.NewInterchain().
		AddChain(chain1).
		AddChain(chain2).
		AddRelayer(r, relayerName).
		AddLink(ibctest.InterchainLink{
			Chain1:  chain1,
			Chain2:  chain2,
			Relayer: r,
			Path:    pathName,
		})

	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation:  false,
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(),
		CreateChannelOpts: ibc.CreateChannelOptions{
			SourcePortName: "interquery",
			DestPortName:   "icqhost",
			Order:          ibc.Unordered,
			Version:        "icq-1",
		},
	}))

	// Fund user accounts, so we can query balances and make assertions.
	const userFunds = int64(10_000_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain1, chain2)
	chain1User := users[0]
	chain2User := users[1]

	// Wait a few blocks for user accounts to be created on chain.
	err = test.WaitForBlocks(ctx, 10, chain1, chain2)
	require.NoError(t, err)

	// Query for the recently created channel-id.
	channels, err := r.GetChannels(ctx, eRep, chain1.Config().ChainID)
	require.NoError(t, err)

	// Start the relayer and set the cleanup function.
	err = r.StartRelayer(ctx, eRep, pathName)
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
	err = test.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	// Query for the balances of an account on the counterparty chain using IBC queries.
	chanID := channels[0].ChannelID
	require.NotEqual(t, "", chanID)

	chain1Addr := chain1User.Bech32Address(chain1.Config().Bech32Prefix)
	require.NotEqual(t, "", chain1Addr)

	chain2Addr := chain2User.Bech32Address(chain2.Config().Bech32Prefix)
	require.NotEqual(t, "", chain2Addr)

	chain1Height, err := chain1.Height(ctx)
	require.NoError(t, err)

	cmd := []string{"icq", "tx", "interquery", "send-query-all-balances", chanID, chain2Addr,
		"--node", chain1.GetRPCAddress(),
		"--home", chain1.HomeDir(),
		"--chain-id", chain1.Config().ChainID,
		"--from", chain1Addr,
		"--keyring-dir", chain1.HomeDir(),
		"--output", "json",
		"--keyring-backend", keyring.BackendTest,
		"-y",
	}
	stdout, stderr, err := chain1.Exec(ctx, cmd, nil)
	require.NoError(t, err)

	// TODO remove debug logging
	t.Logf("stdout: %s \n", stdout)
	t.Logf("stderr: %s \n", stderr)

	var icqSendTxRes cosmos.CosmosTx
	err = json.Unmarshal(stdout, &icqSendTxRes)
	require.NoError(t, err)

	// Wait a few blocks for tx to be available
	err = test.WaitForBlocks(ctx, 2, chain1)
	require.NoError(t, err)

	icqSendTx, err := chain1.(*cosmos.CosmosChain).GetTransaction(icqSendTxRes.TxHash)
	require.NoError(t, err)

	t.Logf("icqSendTx: +%v\n", icqSendTx)

	const evType = "send_packet"
	events := icqSendTx.Events
	var (
		seq, _     = tendermint.AttributeValue(events, evType, "packet_sequence")
		srcPort, _ = tendermint.AttributeValue(events, evType, "packet_src_port")
		srcChan, _ = tendermint.AttributeValue(events, evType, "packet_src_channel")
	)

	sequence, err := strconv.ParseUint(seq, 10, 64)
	require.NoError(t, err)

	ack, err := test.PollForAck(ctx, chain1, chain1Height, chain1Height+15, ibc.Packet{
		Sequence:      sequence,
		SourceChannel: srcChan,
		SourcePort:    srcPort,
	})
	require.NoError(t, err)
	require.NoError(t, ack.Validate())

	// // Wait a few blocks for query to be sent to counterparty.
	// err = test.WaitForBlocks(ctx, 10, chain1)
	// require.NoError(t, err)

	// Check the results from the IBC query above.
	cmd = []string{"icq", "query", "interquery", "query-state", seq,
		"--node", chain1.GetRPCAddress(),
		"--home", chain1.HomeDir(),
		"--chain-id", chain1.Config().ChainID,
	}
	stdout, stderr, err = chain1.Exec(ctx, cmd, nil)
	require.NoError(t, err)

	// TODO remove debug logging
	t.Logf("stdout: %s \n", stdout)
	t.Logf("stderr: %s \n", stderr)
}

func modifyGenesisAllowICQQueries(allowQueries []string) func([]byte) ([]byte, error) {
	return func(genbz []byte) ([]byte, error) {
		g := make(map[string]interface{})
		if err := json.Unmarshal(genbz, &g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis file: %w", err)
		}
		if err := dyno.Set(g, allowQueries, "app_state", "interchainquery", "params", "allow_queries"); err != nil {
			return nil, fmt.Errorf("failed to set allowed interchain queries in genesis json: %w", err)
		}
		out, err := json.Marshal(g)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
		}
		return out, nil
	}
}
