package ibctest

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
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

	client, network := ibctest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	// Get both chains
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "icad", Version: "damian-fix-non-determinism-e2e-tests"},
		{Name: "icad", Version: "damian-fix-non-determinism-e2e-tests"},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain1, chain2 := chains[0], chains[1]

	// Get a relayer instance
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
		Client:    client,
		NetworkID: network,

		SkipPathCreation: true,
	}))

	// chain1
	// "banner spread envelope side kite person disagree path silver will brother under couch edit food venture squirrel civil budget number acquire point work mass"
	// chain2
	// "veteran try aware erosion drink dance decade comic dawn museum release episode original list ability owner size tuition surface ceiling depth seminar capable only"

	// Fund a user account on chain1 and chain2
	const userFunds = int64(10_000_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain1, chain2)
	chain1User := users[0]
	chain2User := users[1]

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

	// Register a new interchain account on behalf of the user acc on chain1
	chain1Addr := chain1User.Bech32Address(chain1.Config().Bech32Prefix)

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

	// Get initial account balances
	chain2Addr := chain2User.Bech32Address(chain2.Config().Bech32Prefix)

	chain2OrigBal, err := chain2.GetBalance(ctx, chain2Addr, chain2.Config().Denom)
	require.NoError(t, err)

	icaOrigBal, err := chain2.GetBalance(ctx, icaAddr, chain2.Config().Denom)
	require.NoError(t, err)

	// Send funds to ICA from user account on chain2
	const transferAmount = 10000
	transfer := ibc.WalletAmount{
		Address: icaAddr,
		Denom:   chain2.Config().Denom,
		Amount:  transferAmount,
	}
	err = chain2.SendFunds(ctx, chain2User.KeyName, transfer)

	// Wait for transfer to be complete and assert balances
	err = test.WaitForBlocks(ctx, 5, chain2)
	require.NoError(t, err)

	chain2Bal, err := chain2.GetBalance(ctx, chain2Addr, chain2.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, chain2OrigBal-transferAmount, chain2Bal)

	icaBal, err := chain2.GetBalance(ctx, icaAddr, chain2.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, icaOrigBal+transferAmount, icaBal)

	t.Logf("User OG Bal: %d \n", chain2OrigBal)
	t.Logf("ICA OG Bal: %d \n", icaOrigBal)
	t.Logf("User Bal: %d \n", chain2Bal)
	t.Logf("ICA Bal: %d \n", icaBal)

	// Build bank transfer msg
	rawMsg, err := json.Marshal(map[string]any{
		"@type":        "/cosmos.bank.v1beta1.MsgSend",
		"from_address": icaAddr,
		"to_address":   chain2Addr,
		"amount": []map[string]any{
			{
				"denom":  chain2.Config().Denom,
				"amount": strconv.Itoa(transferAmount),
			},
		},
	})
	require.NoError(t, err)

	// Send bank transfer msg to ICA on chain2 from the user account on chain1
	sendICATransfer := []string{
		chain1.Config().Bin, "tx", "intertx", "submit", string(rawMsg),
		"--connection-id", connections[0].ID,
		"--from", chain1Addr,
		"--chain-id", chain1.Config().ChainID,
		"--home", chain1.HomeDir(),
		"--node", nodeAddr,
		"--keyring-backend", keyring.BackendTest,
		"-y",
	}
	_, _, err = chain1.Exec(ctx, sendICATransfer, nil)
	require.NoError(t, err)

	// Wait for tx to be relayed
	err = test.WaitForBlocks(ctx, 10, chain2)
	require.NoError(t, err)

	// Assert that the funds have been received by the user account on chain2
	t.Logf("User OG Bal: %d \n", chain2Bal)
	t.Logf("ICA OG Bal: %d \n", icaBal)

	chain2Bal, err = chain2.GetBalance(ctx, chain2Addr, chain2.Config().Denom)
	require.NoError(t, err)

	// Assert that the funds have been removed from the ICA on chain2
	icaBal, err = chain2.GetBalance(ctx, icaAddr, chain2.Config().Denom)
	require.NoError(t, err)

	t.Logf("User Bal: %d \n", chain2Bal)
	t.Logf("ICA Bal: %d \n", icaBal)

	require.Equal(t, chain2OrigBal, chain2Bal)
	require.Equal(t, icaOrigBal, icaBal)
}
