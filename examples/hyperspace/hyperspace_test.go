package hyperspace_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/strangelove-ventures/ibctest/v6"
	"github.com/strangelove-ventures/ibctest/v6/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v6/chain/polkadot"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/strangelove-ventures/ibctest/v6/relayer"
	"github.com/strangelove-ventures/ibctest/v6/relayer/hyperspace"
	"github.com/strangelove-ventures/ibctest/v6/testreporter"
	"github.com/strangelove-ventures/ibctest/v6/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	//transfertypes "github.com/cosmos/ibc-go/v6/modules/apps/transfer/types"
)

// TestHyperspace setup
// * Uses simd docker image from heighliner built from feat/wasm-client branch (rebuild & publish if changed)
// * Uses "seunlanlege/centauri-polkadot" v0.9.27 and "seunlanlege/centauri-parachain" v0.9.27
// * Build local Hyperspace docker from centauri repo: 
//       "docker build -f scripts/hyperspace.Dockerfile -t hyperspace:local ."

// TestHyperspace features
// * sets up a Polkadot parachain 
// * sets up a Cosmos chain
// * sets up the Hyperspace relayer
// * Funds a user wallet on both chains
// * Pushes a wasm client contract to the Cosmos chain
// * TODO: create client, connection, and channel in relayer
// * TODO: start relayer
// * TODO: send transfer over ibc
func TestHyperspace(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := ibctest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	nv := 5 // Number of validators
	nf := 3 // Number of full nodes

	// Override config files to support an ~2.5MB contract
	configFileOverrides := make(map[string]any)

	appTomlOverrides := make(testutil.Toml)
	configTomlOverrides := make(testutil.Toml)

	apiOverrides := make(testutil.Toml)
	apiOverrides["rpc-max-body-bytes"] = 1_800_000
	appTomlOverrides["api"] = apiOverrides

	rpcOverrides := make(testutil.Toml)
	rpcOverrides["max_body_bytes"] = 1_800_000
	rpcOverrides["max_header_bytes"] = 1_900_000
	configTomlOverrides["rpc"] = rpcOverrides

	//mempoolOverrides := make(testutil.Toml)
	//mempoolOverrides["max_tx_bytes"] = 6000000
	//configTomlOverrides["mempool"] = mempoolOverrides

	configFileOverrides["config/app.toml"] = appTomlOverrides
	configFileOverrides["config/config.toml"] = configTomlOverrides

	// Get both chains
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			ChainConfig: ibc.ChainConfig{
				Type:    "polkadot",
				Name:    "composable",
				ChainID: "rococo-local",
				Images: []ibc.DockerImage{
					{
						Repository: "polkadot-node",
						Version:    "local",
						UidGid:     "1025:1025",
					},
					{
						Repository: "parachain-node",
						Version:    "local",
						//UidGid: "1025:1025",
					},
				},
				Bin:            "polkadot",
				Bech32Prefix:   "composable",
				Denom:          "uDOT",
				GasPrices:      "",
				GasAdjustment:  0,
				TrustingPeriod: "",
				CoinType:		"354",
			},
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
		{
			ChainConfig: ibc.ChainConfig{
				Type:    "cosmos",
				Name:    "ibc-go-simd",
				ChainID: "simd",
				Images: []ibc.DockerImage{
					{
						Repository: "ibc-go-simd",
						Version:    "local",
						//Version:    "feat-wasm-clients",
						UidGid:     "1025:1025",
					},
				},
				Bin:            "simd",
				Bech32Prefix:   "cosmos",
				Denom:          "stake",
				GasPrices:      "0.00stake",
				GasAdjustment:  1.3,
				TrustingPeriod: "504h",
				CoinType:       "118",
				//EncodingConfig: WasmClientEncoding(),
				NoHostMount: true,
				ConfigFileOverrides: configFileOverrides,
			},
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	polkadotChain := chains[0].(*polkadot.PolkadotChain)
	cosmosChain := chains[1].(*cosmos.CosmosChain)

	fmt.Println("About to build relayer factory")
	// Get a relayer instance
	r := ibctest.NewBuiltinRelayerFactory(
		ibc.Hyperspace,
		zaptest.NewLogger(t),
		// These two fields are used to pass in a custom Docker image built locally
		relayer.ImagePull(false),
		relayer.CustomDockerImage("hyperspace", "local", "1000:1000"),
		//relayer.CustomDockerImage("hyperspace", "local", "501:20"),
	).Build(t, client, network)

	// Build the network; spin up the chains and configure the relayer
	const pathName = "composable-simd"
	const relayerName = "hyperspace"

	fmt.Println("About to create interchain")
	ic := ibctest.NewInterchain().
		AddChain(polkadotChain).
		AddChain(cosmosChain, ibc.WalletAmount{
			// Use test keys temporarily
			Address: "cosmos1nnypkcfrvu3e9dhzeggpn4kh622l4cq7wwwrn0",
			Denom: "stake",
			Amount: 10_000_000_000_000,
		}).
		AddRelayer(r, relayerName).
		AddLink(ibctest.InterchainLink{
			Chain1:  polkadotChain,
			Chain2:  cosmosChain,
			Relayer: r,
			Path:    pathName,
		})

	fmt.Println("About to build interchain")
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation: true, // Skip path creation, so we can have granular control over the process
	}))
	fmt.Println("Interchain built")

	t.Cleanup(func() {
		_ = ic.Close()
	})

	err = testutil.WaitForBlocks(ctx, 2, cosmosChain, polkadotChain)
	require.NoError(t, err, "cosmos or polkadot chain failed to make blocks1")

	// Fund user1 on both relay and parachain, must wait a block to fund user2 due to same faucet address
	fundAmount := int64(12_333_000_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, "user1", fundAmount, polkadotChain, cosmosChain)
	polkadotUser, cosmosUser := users[0], users[1]
	err = testutil.WaitForBlocks(ctx, 2, polkadotChain, cosmosChain)
	require.NoError(t, err, "cosmos or polkadot chain failed to make blocks2")

	polkadotUserAmount, err := polkadotChain.GetBalance(ctx, polkadotUser.FormattedAddress(), polkadotChain.Config().Denom)
	require.NoError(t, err)
	fmt.Println("Polkadot user amount: ", polkadotUserAmount)
	require.Equal(t, fundAmount, polkadotUserAmount, "Initial polkadot user amount not expected")
	parachainUserAmount, err := polkadotChain.GetBalance(ctx, polkadotUser.FormattedAddress(), "")
	require.NoError(t, err)
	fmt.Println("Parachain user amount: ", parachainUserAmount)
	require.Equal(t, fundAmount, parachainUserAmount, "Initial parachain user amount not expected")
	cosmosUserAmount, err := cosmosChain.GetBalance(ctx, cosmosUser.FormattedAddress(), cosmosChain.Config().Denom)
	require.NoError(t, err)
	fmt.Println("Cosmos user amount: ", cosmosUserAmount)
	require.Equal(t, fundAmount, cosmosUserAmount, "Initial cosmos user amount not expected")
	
	codeHash, err := cosmosChain.StoreClientContract(ctx, cosmosUser.KeyName(), "../polkadot/ics10_grandpa_cw.wasm")
	t.Logf("Contract codeHash: %s", codeHash)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 2, polkadotChain, cosmosChain)
	require.NoError(t, err)

	var getCodeQueryMsgRsp GetCodeQueryMsgResponse
	err = cosmosChain.QueryClientContractCode(ctx, codeHash, &getCodeQueryMsgRsp)
	codeHashByte32 := sha256.Sum256(getCodeQueryMsgRsp.Code)
	codeHash2 := hex.EncodeToString(codeHashByte32[:])
	t.Logf("Contract codeHash from code: %s", codeHash2)
	require.NoError(t, err)
	require.NotEmpty(t, getCodeQueryMsgRsp.Code)
	require.Equal(t, codeHash, codeHash2)

	r.SetClientContractHash(ctx, eRep, cosmosChain.Config(), codeHash)

	r.(*hyperspace.HyperspaceRelayer).DockerRelayer.PrintConfigs(ctx, eRep, cosmosChain.Config().ChainID)
	r.(*hyperspace.HyperspaceRelayer).DockerRelayer.PrintConfigs(ctx, eRep, polkadotChain.Config().ChainID)
	r.(*hyperspace.HyperspaceRelayer).DockerRelayer.PrintCoreConfig(ctx, eRep)

	// Create new clients
	err = r.CreateClients(ctx, eRep, pathName, ibc.CreateClientOptions{TrustingPeriod: "330h"})
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 1, cosmosChain, polkadotChain)
	require.NoError(t, err)
	
	r.(*hyperspace.HyperspaceRelayer).DockerRelayer.PrintConfigs(ctx, eRep, cosmosChain.Config().ChainID)
	r.(*hyperspace.HyperspaceRelayer).DockerRelayer.PrintConfigs(ctx, eRep, polkadotChain.Config().ChainID)
	
	// Create a new connection
	err = r.CreateConnections(ctx, eRep, pathName)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 1, cosmosChain, polkadotChain)
	require.NoError(t, err)

	r.(*hyperspace.HyperspaceRelayer).DockerRelayer.PrintConfigs(ctx, eRep, cosmosChain.Config().ChainID)
	r.(*hyperspace.HyperspaceRelayer).DockerRelayer.PrintConfigs(ctx, eRep, polkadotChain.Config().ChainID)
	
	// Create a new channel & get channels from each chain
	err = r.CreateChannel(ctx, eRep, pathName, ibc.DefaultChannelOpts())
	require.NoError(t, err)
	
	err = testutil.WaitForBlocks(ctx, 1, cosmosChain, polkadotChain)
	require.NoError(t, err)

	r.(*hyperspace.HyperspaceRelayer).DockerRelayer.PrintConfigs(ctx, eRep, cosmosChain.Config().ChainID)
	r.(*hyperspace.HyperspaceRelayer).DockerRelayer.PrintConfigs(ctx, eRep, polkadotChain.Config().ChainID)
	
	// Hyperspace panics on "hyperspace query channels --config xxx", this is needed.
	//_, err = r.GetChannels(ctx, eRep, cosmosChain.Config().ChainID)
	//require.NoError(t, err)
	//fmt.Println("Cosmos connection: ", cosmosConnections[0].ID)
	//_, err = r.GetChannels(ctx, eRep, polkadotChain.Config().ChainID)
	//require.NoError(t, err)
	//fmt.Println("Polkadot connection: ", polkadotConnections[0].ID)
	//err = testutil.WaitForBlocks(ctx, 2, polkadotChain, cosmosChain)
	//require.NoError(t, err)
	
	// Start relayer
	/*r.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = r.StopRelayer(ctx, eRep)
		if err != nil {
			panic(err)
		}
	})

	// Send Transaction
	amountToSend := int64(177_000_000)
	dstAddress := polkadotUser.FormattedAddress()
	transfer := ibc.WalletAmount{
		Address: dstAddress,
		Denom:   cosmosChain.Config().Denom,
		Amount:  amountToSend,
	}
	tx, err := cosmosChain.SendIBCTransfer(ctx, "channel-0", cosmosUser.KeyName(), transfer, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate())	// test source wallet has decreased funds
	
	err = testutil.WaitForBlocks(ctx, 20, cosmosChain, polkadotChain)
	require.NoError(t, err)

	expectedBal := cosmosUserAmount - amountToSend
	cosmosUserBalNew, err := cosmosChain.GetBalance(ctx, cosmosUser.FormattedAddress(), cosmosChain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, expectedBal, cosmosUserBalNew)*/

	// Trace IBC Denom
	//srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom("transfer", "channel-0", cosmosChain.Config().Denom))
	//dstIbcDenom := srcDenomTrace.IBCDenom()

	// Test destination wallet has increased funds
	//polkadotUserBalNew, err := polkadotChain.GetBalance(ctx, polkadotUser.FormattedAddress(), dstIbcDenom)
	//require.NoError(t, err)
	//require.Equal(t, amountToSend, polkadotUserBalNew)
	// Then send ibc tx from cosmos -> substrate and vice versa
	//polkadotChain.SendIBCTransfer(), verify
	//cosmosChain.SendIBCTransfer(), verify

}

type GetCodeQueryMsgResponse struct {
	Code []byte `json:"code"`
}