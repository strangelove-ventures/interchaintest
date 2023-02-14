package hyperspace_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"

	transfertypes "github.com/cosmos/ibc-go/v6/modules/apps/transfer/types"
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
)

// TestHyperspace setup
// Must build local docker images of hyperspace, parachain, and polkadot
// ###### hyperspace ######
// * Repo: ComposableFi/centauri
// * Branch: edjroz/add-query-channels
// * Commit: 2996275ce29a2d7411af7bf3b1d65bc66a719da8
// * Build local Hyperspace docker from centauri repo:
//    amd64: "docker build -f scripts/hyperspace.Dockerfile -t hyperspace:local ."
//    arm64: "docker build -f scripts/hyperspace.aarch64.Dockerfile -t hyperspace:latest --platform=linux/arm64/v8 .
// ###### parachain ######
// * Repo: ComposableFi/centauri
// * Branch: edjroz/add-query-channels
// * Commit: 043470ce1932c418d15df635480da8efb61d66d7
// * Build local parachain docker from centauri repo:
//     ./scripts/build-parachain-node-docker.sh (you can change the script to compile for ARM arch if needed)
// ###### polkadot ######
// * Repo: paritytech/polkadot
// * Branch: release-v0.9.33
// * Commit: c7d6c21242fc654f6f069e12c00951484dff334d
// * Build local polkadot docker from  polkadot repo
//     amd64: docker build -f scripts/ci/dockerfiles/polkadot/polkadot_builder.Dockerfile . -t polkadot-node:local
//     arm64: docker build --platform linux/arm64 -f scripts/ci/dockerfiles/polkadot/polkadot_builder.aarch64.Dockerfile . -t polkadot-node:local

// TestHyperspace features
// * sets up a Polkadot parachain
// * sets up a Cosmos chain
// * sets up the Hyperspace relayer
// * Funds a user wallet on both chains
// * Pushes a wasm client contract to the Cosmos chain
// * create client, connection, and channel in relayer
// * start relayer
// * send transfer over ibc
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
			ChainName: "composable", // Set ChainName so that a suffix with a "dash" is not appended (required for hyperspace)
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
				CoinType:       "354",
			},
			NumValidators: &nv,
			NumFullNodes:  &nf,
		},
		{
			ChainName: "simd", // Set chain name so that a suffix with a "dash" is not appended (required for hyperspace)
			ChainConfig: ibc.ChainConfig{
				Type:    "cosmos",
				Name:    "simd",
				ChainID: "simd",
				Images: []ibc.DockerImage{
					{
						Repository: "ghcr.io/strangelove-ventures/heighliner/ibc-go-simd",
						Version:    "feat-wasm-client-230118",
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
				NoHostMount:         true,
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
		AddChain(cosmosChain).
		AddRelayer(r, relayerName).
		AddLink(ibctest.InterchainLink{
			Chain1:  polkadotChain,
			Chain2:  cosmosChain,
			Relayer: r,
			Path:    pathName,
		})

	fmt.Println("About to build interchain")
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  true, // Skip path creation, so we can have granular control over the process
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

	// Store grandpa contract
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

	// Set client contract hash in cosmos chain config
	err = r.SetClientContractHash(ctx, eRep, cosmosChain.Config(), codeHash)
	require.NoError(t, err)

	r.(*hyperspace.HyperspaceRelayer).DockerRelayer.PrintConfigs(ctx, eRep, cosmosChain.Config().ChainID)
	r.(*hyperspace.HyperspaceRelayer).DockerRelayer.PrintConfigs(ctx, eRep, polkadotChain.Config().ChainID)
	r.(*hyperspace.HyperspaceRelayer).DockerRelayer.PrintCoreConfig(ctx, eRep)

	// Create new clients
	err = r.CreateClients(ctx, eRep, pathName, ibc.CreateClientOptions{TrustingPeriod: "330h"})
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 2, cosmosChain, polkadotChain)
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
	cosmosChannelOutput, err := r.GetChannels(ctx, eRep, cosmosChain.Config().ChainID)
	require.NoError(t, err)
	require.Equal(t, len(cosmosChannelOutput), 1)

	require.Equal(t, cosmosChannelOutput[0].ChannelID, "channel-0")
	require.Equal(t, cosmosChannelOutput[0].PortID, "transfer")

	//fmt.Println("Cosmos connection: ", cosmosConnections[0].ID)
	polkadotChannelOutput, err := r.GetChannels(ctx, eRep, polkadotChain.Config().ChainID)
	require.NoError(t, err)

	require.Equal(t, polkadotChannelOutput[0].ChannelID, "channel-0")
	require.Equal(t, polkadotChannelOutput[0].PortID, "transfer")

	require.Equal(t, len(polkadotChannelOutput), 1)

	//fmt.Println("Polkadot connection: ", polkadotConnections[0].ID)
	//	err = testutil.WaitForBlocks(ctx, 2, polkadotChain, cosmosChain)
	//	require.NoError(t, err)

	err = polkadotChain.EnableIbcTransfers()
	require.NoError(t, err)

	// Start relayer
	r.StartRelayer(ctx, eRep, pathName)
	require.NoError(t, err)
	t.Cleanup(func() {
		err = r.StopRelayer(ctx, eRep)
		if err != nil {
			panic(err)
		}
	})
	err = testutil.WaitForBlocks(ctx, 2, cosmosChain, polkadotChain)
	require.NoError(t, err)

	// Mint 100 UNIT for alice and "polkadotUser", not sure why the ~1.5M UNIT from balance/genesis doesn't work
	mint := ibc.WalletAmount{
		Address: polkadotUser.FormattedAddress(),
		Denom:   "1",
		Amount:  int64(100_000_000_000_000), // 100 UNITS, not 100T
	}
	err = polkadotChain.MintFunds("alice", mint)
	require.NoError(t, err)
	err = testutil.WaitForBlocks(ctx, 2, cosmosChain, polkadotChain)
	require.NoError(t, err)
	mint2 := ibc.WalletAmount{
		Address: "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY", // Alice
		Denom:   "1",
		Amount:  int64(100_000_000_000_000), // 100 UNITS, not 100T
	}
	err = polkadotChain.MintFunds("alice", mint2)
	require.NoError(t, err)

	// Send IBC Transaction, from Cosmos to Parachain (stake)
	amountToSend := int64(1_770_000)
	transfer := ibc.WalletAmount{
		Address: polkadotUser.FormattedAddress(),
		Denom:   cosmosChain.Config().Denom,
		Amount:  amountToSend,
	}
	tx, err := cosmosChain.SendIBCTransfer(ctx, cosmosChannelOutput[0].ChannelID, cosmosUser.KeyName(), transfer, ibc.TransferOptions{})
	require.NoError(t, err)
	require.NoError(t, tx.Validate()) // test source wallet has decreased funds

	err = testutil.WaitForBlocks(ctx, 10, cosmosChain, polkadotChain)
	require.NoError(t, err)

	// Verify cosmosUser balance went down 1.77M
	expectedBal := cosmosUserAmount - amountToSend
	cosmosUserBalNew, err := cosmosChain.GetBalance(ctx, cosmosUser.FormattedAddress(), cosmosChain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, expectedBal, cosmosUserBalNew)

	// Trace IBC Denom of stake on parachain
	srcDenomTrace := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(cosmosChannelOutput[0].PortID, cosmosChannelOutput[0].ChannelID, cosmosChain.Config().Denom))
	dstIbcDenom := srcDenomTrace.IBCDenom()
	fmt.Println("Dst Ibc denom: ", dstIbcDenom)

	// Test destination wallet has increased funds, this is not working
	//pubKey, err := polkadot.DecodeAddressSS58(polkadotUser.FormattedAddress())
	//polkadotUserIbcCoins2, err := polkadotChain.GetIbcBalance(ctx, pubKey)
	//polkadotUserIbcCoins3, err := polkadotChain.GetIbcBalance(ctx, []byte(hex.EncodeToString(pubKey)))
	polkadotUserIbcCoins, err := polkadotChain.GetIbcBalance(ctx, polkadotUser.Address())
	fmt.Println("IbcCoins: ", polkadotUserIbcCoins.String(), "  -- this probably doesn't work, Error: ", err)

	// Send 1.18M stake from ParachainUser to CosmosUser
	amountToSend2 := int64(1_180_000)
	transfer2 := ibc.WalletAmount{
		Address: cosmosUser.FormattedAddress(),
		Denom:   "2", // stake
		Amount:  amountToSend2,
	}
	_, err = polkadotChain.SendIBCTransfer(ctx, polkadotChannelOutput[0].ChannelID, polkadotUser.KeyName(), transfer2, ibc.TransferOptions{})
	require.NoError(t, err)

	// Send 1.88T "UNIT" from Alice to CosmosUser
	amountToSend1 := int64(1_880_000_000_000)
	transfer1 := ibc.WalletAmount{
		Address: cosmosUser.FormattedAddress(),
		Denom:   "1", // UNIT
		Amount:  amountToSend1,
	}
	_, err = polkadotChain.SendIBCTransfer(ctx, polkadotChannelOutput[0].ChannelID, "alice", transfer1, ibc.TransferOptions{})
	require.NoError(t, err)

	// Wait for MsgRecvPacket
	pollForBalance := ibc.WalletAmount{
		Address: cosmosUser.FormattedAddress(),
		Denom:   cosmosChain.Config().Denom,
		Amount:  expectedBal + amountToSend2,
	}
	err = cosmos.PollForBalance(ctx, cosmosChain, 30, pollForBalance)
	require.NoError(t, err)

	// Verify final balances
	cosmosUserNativeBal, err := cosmosChain.GetBalance(ctx, cosmosUser.FormattedAddress(), cosmosChain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, pollForBalance.Amount, cosmosUserNativeBal)
	fmt.Println("Initial: ", cosmosUserAmount, "   Middle:", cosmosUserBalNew, "  Final: ", cosmosUserNativeBal)
	// Trace IBC Denom
	srcDenomTrace2 := transfertypes.ParseDenomTrace(transfertypes.GetPrefixedDenom(polkadotChannelOutput[0].PortID, polkadotChannelOutputChannelOutput[0].ChannelID, "UNIT"))
	dstIbcDenom2 := srcDenomTrace2.IBCDenom()
	fmt.Println("Dst Ibc denom:2 ", dstIbcDenom2)
	cosmosUserIbcBal2, err := cosmosChain.GetBalance(ctx, cosmosUser.FormattedAddress(), dstIbcDenom2)
	require.NoError(t, err)
	require.Equal(t, amountToSend1, cosmosUserIbcBal2)
	fmt.Println("CosmosUserIbcBal2: ", cosmosUserIbcBal2)
}

type GetCodeQueryMsgResponse struct {
	Code []byte `json:"code"`
}
