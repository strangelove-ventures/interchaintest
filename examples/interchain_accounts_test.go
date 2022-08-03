package ibctest

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	transfertypes "github.com/cosmos/ibc-go/v4/modules/apps/transfer/types"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/test"
	"github.com/strangelove-ventures/ibctest/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestInterchainAccounts(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	home := ibctest.TempDir(t)
	client, network := ibctest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	// Get both chains
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "icad", Version: "master"},
		{Name: "icad", Version: "master"},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain1, chain2 := chains[0], chains[1]

	r := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
		t, client, network,
	)

	// Build the network; spin up the chains and configure the relayer
	const pathName = "test-path"

	ic := ibctest.NewInterchain().
		AddChain(chain1).
		AddChain(chain2).
		AddRelayer(r, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  chain1,
			Chain2:  chain2,
			Relayer: r,
			Path:    pathName,
		})

	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		HomeDir:   home,
		Client:    client,
		NetworkID: network,

		SkipPathCreation: true,
	}))

	// Fund a user account on chain1
	const userFunds = int64(10_000_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain1)
	chain1User := users[0]

	// Generate a new path
	err = r.GeneratePath(ctx, eRep, chain1.Config().ChainID, chain2.Config().ChainID, pathName)
	require.NoError(t, err)

	// Create new clients
	err = r.CreateClients(ctx, eRep, pathName)
	require.NoError(t, err)

	err = test.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	// Create a new connection
	err = r.CreateConnections(ctx, eRep, pathName)
	require.NoError(t, err)

	err = test.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	// Query for the newly created connection
	connections, err := r.GetConnections(ctx, eRep, chain1.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, 1, len(connections))

	// Not really great, consider exposing a function for retrieving a nodes addr like below
	host := strings.Split(chain1.GetRPCAddress(), "//")[1]
	nodeAddr := fmt.Sprintf("tcp://%s", host)

	chain1Addr := chain1User.Bech32Address(chain1.Config().Bech32Prefix)

	// Register a new interchain account
	registerICA := []string{
		chain1.Config().Bin, "tx", "intertx", "register",
		"--from", chain1Addr,
		"--connection-id", connections[0].ID,
		"--chain-id", chain1.Config().ChainID,
		"--home", chain1.HomeDir(),
		"--node", nodeAddr,
		"--keyring-backend", keyring.BackendTest,
		"-y",
	}
	_, _, err = chain1.Exec(ctx, registerICA, nil)
	require.NoError(t, err)

	// Start the relayer
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

	// Wait for relayer to start up and finish channel handshake
	err = test.WaitForBlocks(ctx, 15, chain1, chain2)
	require.NoError(t, err)

	// Query for the newly registered interchain account
	queryICA := []string{
		chain1.Config().Bin, "query", "intertx", "interchainaccounts", connections[0].ID, chain1Addr,
		"--chain-id", chain1.Config().ChainID,
		"--home", chain1.HomeDir(),
		"--node", nodeAddr,
	}
	stdout, _, err := chain1.Exec(ctx, queryICA, nil)
	require.NoError(t, err)

	// At this point stdout should look like this
	// interchain_account_address: cosmos1p76n3mnanllea4d3av0v0e42tjj03cae06xq8fwn9at587rqp23qvxsv0j
	// we split the string at the : and then grab the address.
	parts := strings.SplitN(string(stdout), ":", 2)
	require.Equal(t, 2, len(parts))

	icaAddr := strings.TrimSpace(parts[1])
	require.NotEqual(t, "", icaAddr)

	// Get the balance of the account on chain1
	chain1OrigBal, err := chain1.GetBalance(ctx, chain1Addr, chain1.Config().Denom)
	require.NoError(t, err)

	// Build a bank transfer msg to send from the controller account to the ICA
	const transferAmount = 10000
	jsonMsg := fmt.Sprintf(`{"@type":"/cosmos.bank.v1beta1.MsgSend","from_address":"%s","to_address":"%s","amount":[{"denom":"%s","amount":"%s"}]}`, chain1Addr, icaAddr, chain1.Config().Denom, strconv.Itoa(transferAmount))
	//msg, err := json.Marshal(map[string]any{
	//	"@type":        "/cosmos.bank.v1beta1.MsgSend",
	//	"from_address": chain1Addr,
	//	"to_address":   icaAddr,
	//	"amount": []map[string]any{
	//		{
	//			"denom":  chain1.Config().Denom,
	//			"amount": strconv.Itoa(transferAmount),
	//		},
	//	},
	//})
	rawMsg := json.RawMessage(jsonMsg)
	msg, err := rawMsg.MarshalJSON()
	require.NoError(t, err)

	t.Log()
	t.Log(string(msg))
	t.Log()

	// Send ICA transfer
	t.Log("Before sending transfer")
	sendICATransfer := []string{
		chain1.Config().Bin, "tx", "intertx", "submit", string(msg),
		"--connection-id", connections[0].ID,
		"--from", chain1Addr,
		"--chain-id", chain1.Config().ChainID,
		"--home", chain1.HomeDir(),
		"--node", nodeAddr,
		"--keyring-backend", keyring.BackendTest,
		"-y",
	}
	stdout, stderr, err := chain1.Exec(ctx, sendICATransfer, nil)
	require.NoError(t, err)

	t.Logf("stdout: %s\n", stdout)
	t.Logf("stderr: %s\n", stderr)

	t.Log("After sending transfer")

	// Wait for tx to be relayed
	err = test.WaitForBlocks(ctx, 10, chain2)
	require.NoError(t, err)

	t.Log("After waiting for blocks")

	// Compose the ibc denom for the tokens sent to the ICA from chain1
	channels, err := r.GetChannels(ctx, eRep, chain1.Config().ChainID)
	require.NoError(t, err)

	t.Log("After get channels")

	prefixedDenom := transfertypes.GetPrefixedDenom(channels[0].Counterparty.PortID, channels[0].Counterparty.ChannelID, chain1.Config().Denom)
	ibcDenom := transfertypes.ParseDenomTrace(prefixedDenom)

	t.Log("Before get balance")
	// Assert that the account on chain1 has subtracted the funds just sent
	chain1Bal, err := chain1.GetBalance(ctx, chain1Addr, chain1.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, chain1OrigBal-transferAmount, chain1Bal)

	// Assert that the interchain account has received the funds
	icaBal, err := chain2.GetBalance(ctx, icaAddr, ibcDenom.IBCDenom())
	require.NoError(t, err)
	require.Equal(t, transferAmount, icaBal)
}
