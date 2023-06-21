package ibc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/icza/dyno"
	interchaintest "github.com/strangelove-ventures/interchaintest/v5"
	"github.com/strangelove-ventures/interchaintest/v5/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v5/ibc"
	"github.com/strangelove-ventures/interchaintest/v5/internal/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v5/relayer"
	"github.com/strangelove-ventures/interchaintest/v5/testreporter"
	"github.com/strangelove-ventures/interchaintest/v5/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestInterchainQueries is a test case that performs basic simulations and assertions around the packet implementation
// of interchain queries. See: https://github.com/quasar-finance/interchain-query-demo
func TestInterchainQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	dockerImage := ibc.DockerImage{
		Repository: "ghcr.io/strangelove-ventures/heighliner/icqd",
		Version:    "latest",
		UidGid:     dockerutil.GetHeighlinerUserString(),
	}

	// Get both chains
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			ChainName: "sender",
			ChainConfig: ibc.ChainConfig{
				Type:           "cosmos",
				Name:           "sender",
				ChainID:        "sender",
				Images:         []ibc.DockerImage{dockerImage},
				Bin:            "icq",
				Bech32Prefix:   "cosmos",
				Denom:          "atom",
				GasPrices:      "0.00atom",
				TrustingPeriod: "300h",
				GasAdjustment:  1.1,
			}},
		{
			ChainName: "receiver",
			ChainConfig: ibc.ChainConfig{
				Type:           "cosmos",
				Name:           "receiver",
				ChainID:        "receiver",
				Images:         []ibc.DockerImage{dockerImage},
				Bin:            "icq",
				Bech32Prefix:   "cosmos",
				Denom:          "atom",
				GasPrices:      "0.00atom",
				TrustingPeriod: "300h",
				GasAdjustment:  1.1,
				ModifyGenesis:  modifyGenesisAllowICQQueries([]string{"/cosmos.bank.v1beta1.Query/AllBalances"}), // Add the whitelisted queries to the host chain
			}},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain1, chain2 := chains[0], chains[1]

	// Get a relayer instance
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.StartupFlags("-b", "100"),
	).Build(t, client, network)

	// Build the network; spin up the chains and configure the relayer
	const pathName = "test1-test2"
	const relayerName = "relayer"

	ic := interchaintest.NewInterchain().
		AddChain(chain1).
		AddChain(chain2).
		AddRelayer(r, relayerName).
		AddLink(interchaintest.InterchainLink{
			Chain1:  chain1,
			Chain2:  chain2,
			Relayer: r,
			Path:    pathName,
			CreateChannelOpts: ibc.CreateChannelOptions{
				SourcePortName: "interquery",
				DestPortName:   "icqhost",
				Order:          ibc.Unordered,
				Version:        "icq-1",
			},
		})

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Fund user accounts, so we can query balances and make assertions.
	const userFunds = int64(10_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain1, chain2)
	chain1User := users[0]
	chain2User := users[1]

	// Wait a few blocks for user accounts to be created on chain.
	err = testutil.WaitForBlocks(ctx, 5, chain1, chain2)
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
	err = testutil.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	// Query for the balances of an account on the counterparty chain using interchain queries.
	chanID := channels[0].Counterparty.ChannelID
	require.NotEmpty(t, chanID)

	chain1Addr := chain1User.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(chain1.Config().Bech32Prefix)
	require.NotEmpty(t, chain1Addr)

	chain2Addr := chain2User.(*cosmos.CosmosWallet).FormattedAddressWithPrefix(chain2.Config().Bech32Prefix)
	require.NotEmpty(t, chain2Addr)

	cmd := []string{"icq", "tx", "interquery", "send-query-all-balances", chanID, chain2Addr,
		"--node", chain1.GetRPCAddress(),
		"--home", chain1.HomeDir(),
		"--chain-id", chain1.Config().ChainID,
		"--from", chain1Addr,
		"--keyring-dir", chain1.HomeDir(),
		"--keyring-backend", keyring.BackendTest,
		"-y",
	}
	_, _, err = chain1.Exec(ctx, cmd, nil)
	require.NoError(t, err)

	// Wait a few blocks for query to be sent to counterparty.
	err = testutil.WaitForBlocks(ctx, 10, chain1)
	require.NoError(t, err)

	// Check the results from the interchain query above.
	cmd = []string{"icq", "query", "interquery", "query-state", strconv.Itoa(1),
		"--node", chain1.GetRPCAddress(),
		"--home", chain1.HomeDir(),
		"--chain-id", chain1.Config().ChainID,
		"--output", "json",
	}
	stdout, _, err := chain1.Exec(ctx, cmd, nil)
	require.NoError(t, err)

	results := &icqResults{}
	err = json.Unmarshal(stdout, results)
	require.NoError(t, err)
	require.NotEmpty(t, results.Request)
	require.NotEmpty(t, results.Response)
}

type icqResults struct {
	Request struct {
		Type       string `json:"@type"`
		Address    string `json:"address"`
		Pagination struct {
			Key        interface{} `json:"key"`
			Offset     string      `json:"offset"`
			Limit      string      `json:"limit"`
			CountTotal bool        `json:"count_total"`
			Reverse    bool        `json:"reverse"`
		} `json:"pagination"`
	} `json:"request"`
	Response struct {
		Type     string `json:"@type"`
		Balances []struct {
			Amount string `json:"amount"`
			Denom  string `json:"denom"`
		} `json:"balances"`
		Pagination struct {
			NextKey interface{} `json:"next_key"`
			Total   string      `json:"total"`
		} `json:"pagination"`
	} `json:"response"`
}

func modifyGenesisAllowICQQueries(allowQueries []string) func(ibc.ChainConfig, []byte) ([]byte, error) {
	return func(chainConfig ibc.ChainConfig, genbz []byte) ([]byte, error) {
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
