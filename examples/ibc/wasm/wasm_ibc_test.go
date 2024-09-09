package wasm_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v9"
	"github.com/strangelove-ventures/interchaintest/v9/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v9/chain/cosmos/wasm"
	"github.com/strangelove-ventures/interchaintest/v9/ibc"
	"github.com/strangelove-ventures/interchaintest/v9/testreporter"
	"github.com/strangelove-ventures/interchaintest/v9/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestWasmIbc(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	ctx := context.Background()

	// Chain Factory
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{Name: "juno", ChainName: "juno1", Version: "latest", ChainConfig: ibc.ChainConfig{
			GasPrices:      "0.00ujuno",
			EncodingConfig: wasm.WasmEncoding(),
		}},
		{Name: "juno", ChainName: "juno2", Version: "latest", ChainConfig: ibc.ChainConfig{
			GasPrices:      "0.00ujuno",
			EncodingConfig: wasm.WasmEncoding(),
		}},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	juno1, juno2 := chains[0], chains[1]

	// Relayer Factory
	client, network := interchaintest.DockerSetup(t)
	r := interchaintest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
		t, client, network)

	// Prep Interchain
	const ibcPath = "wasmpath"
	ic := interchaintest.NewInterchain().
		AddChain(juno1).
		AddChain(juno2).
		AddRelayer(r, "relayer").
		AddLink(interchaintest.InterchainLink{
			Chain1:  juno1,
			Chain2:  juno2,
			Relayer: r,
			Path:    ibcPath,
		})

	// Log location
	f, err := interchaintest.CreateLogFile(fmt.Sprintf("wasm_ibc_test_%d.json", time.Now().Unix()))
	require.NoError(t, err)
	// Reporter/logs
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation: false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create and Fund User Wallets
	initBal := math.NewInt(100_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", initBal, juno1, juno2)
	juno1User := users[0]
	juno2User := users[1]

	err = testutil.WaitForBlocks(ctx, 2, juno1, juno2)
	require.NoError(t, err)

	juno1UserBalInitial, err := juno1.GetBalance(ctx, juno1User.FormattedAddress(), juno1.Config().Denom)
	require.NoError(t, err)
	require.True(t, juno1UserBalInitial.Equal(initBal))

	juno2UserBalInitial, err := juno2.GetBalance(ctx, juno2User.FormattedAddress(), juno2.Config().Denom)
	require.NoError(t, err)
	require.True(t, juno2UserBalInitial.Equal(initBal))

	// Start the relayer
	err = r.StartRelayer(ctx, eRep, ibcPath)
	require.NoError(t, err)

	t.Cleanup(
		func() {
			err := r.StopRelayer(ctx, eRep)
			if err != nil {
				t.Logf("an error occurred while stopping the relayer: %s", err)
			}
		},
	)

	juno1Chain := juno1.(*cosmos.CosmosChain)
	juno2Chain := juno2.(*cosmos.CosmosChain)

	// Store ibc_reflect_send.wasm contract on juno1
	juno1ContractCodeId, err := juno1Chain.StoreContract(
		ctx, juno1User.KeyName(), "sample_contracts/cw_ibc_example.wasm")
	require.NoError(t, err)

	// Instantiate the contract on juno1
	juno1ContractAddr, err := juno1Chain.InstantiateContract(
		ctx, juno1User.KeyName(), juno1ContractCodeId, "{}", true)
	require.NoError(t, err)

	// Store ibc_reflect_send.wasm on juno2
	juno2ContractCodeId, err := juno2Chain.StoreContract(
		ctx, juno2User.KeyName(), "sample_contracts/cw_ibc_example.wasm")
	require.NoError(t, err)

	// Instantiate contract on juno2
	juno2ContractAddr, err := juno2Chain.InstantiateContract(
		ctx, juno2User.KeyName(), juno2ContractCodeId, "{}", true)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 1, juno1, juno2)
	require.NoError(t, err)

	// Query the reflect sender contract on Juno1 for it's port id
	juno1ContractInfo, err := juno1Chain.QueryContractInfo(ctx, juno1ContractAddr)
	require.NoError(t, err)
	juno1ContractPortId := juno1ContractInfo.ContractInfo.IbcPortID

	// Query the reflect contract on Juno2 for it's port id
	juno2ContractInfo, err := juno2Chain.QueryContractInfo(ctx, juno2ContractAddr)
	require.NoError(t, err)
	juno2ContractPortId := juno2ContractInfo.ContractInfo.IbcPortID

	// Create channel between Juno1 and Juno2
	err = r.CreateChannel(ctx, eRep, ibcPath, ibc.CreateChannelOptions{
		SourcePortName: juno1ContractPortId,
		DestPortName:   juno2ContractPortId,
		Order:          ibc.Unordered,
		Version:        "counter-1",
	})
	require.NoError(t, err)

	// Wait for the channel to get set up and whoami message to exchange
	err = testutil.WaitForBlocks(ctx, 10, juno1, juno2)
	require.NoError(t, err)

	// Get contract channel
	juno1ChannelInfo, err := r.GetChannels(ctx, eRep, juno1.Config().ChainID)
	require.NoError(t, err)
	juno1ChannelID := juno1ChannelInfo[len(juno1ChannelInfo)-1].ChannelID

	// Get contract channel
	juno2ChannelInfo, err := r.GetChannels(ctx, eRep, juno1.Config().ChainID)
	require.NoError(t, err)
	juno2ChannelID := juno2ChannelInfo[len(juno2ChannelInfo)-1].ChannelID

	// Prepare the query and execute messages to interact with the contracts
	queryJuno1CountMsg := fmt.Sprintf(`{"get_count":{"channel":"%s"}}`, juno1ChannelID)
	queryJuno2CountMsg := fmt.Sprintf(`{"get_count":{"channel":"%s"}}`, juno2ChannelID)
	juno1IncrementMsg := fmt.Sprintf(`{"increment": {"channel":"%s"}}`, juno1ChannelID)
	juno2IncrementMsg := fmt.Sprintf(`{"increment": {"channel":"%s"}}`, juno2ChannelID)

	_, err = juno1.Height(ctx)
	require.NoError(t, err)

	// Query the count of the contract on juno1- should be 0 as no packets have been sent through
	var juno1InitialCountResponse CwIbcCountResponse
	err = juno1Chain.QueryContract(ctx, juno1ContractAddr, queryJuno1CountMsg, &juno1InitialCountResponse)
	require.NoError(t, err)
	require.Equal(t, 0, juno1InitialCountResponse.Data.Count)

	// Query the count of the contract on juno1- should be 0 as no packets have been sent through
	var juno2InitialCountResponse CwIbcCountResponse
	err = juno2Chain.QueryContract(ctx, juno2ContractAddr, queryJuno2CountMsg, &juno2InitialCountResponse)
	require.NoError(t, err)
	require.Equal(t, 0, juno2InitialCountResponse.Data.Count)

	// Send packet from juno1 to juno2 and increment the juno2 contract count
	juno1Increment, err := juno1Chain.ExecuteContract(ctx, juno1User.KeyName(), juno1ContractAddr, juno1IncrementMsg)
	require.NoError(t, err)
	// Check if the transaction was successful
	require.Equal(t, uint32(0), juno1Increment.Code)

	// Wait for the ibc packet to be delivered
	err = testutil.WaitForBlocks(ctx, 2, juno1, juno2)
	require.NoError(t, err)

	// Query the count of the contract on juno2- should be 1 as a single packet has been sent through
	var juno2IncrementedCountResponse CwIbcCountResponse
	err = juno2Chain.QueryContract(ctx, juno2ContractAddr, queryJuno2CountMsg, &juno2IncrementedCountResponse)
	require.NoError(t, err)
	require.Equal(t, 1, juno2IncrementedCountResponse.Data.Count)

	// Query the count of the contract on juno1- should still be 0 as no packets have been sent through
	var juno1PreIncrementedCountResponse CwIbcCountResponse
	err = juno1Chain.QueryContract(ctx, juno1ContractAddr, queryJuno1CountMsg, &juno1PreIncrementedCountResponse)
	require.NoError(t, err)
	require.Equal(t, 0, juno1PreIncrementedCountResponse.Data.Count)

	// send packet from juno2 to juno1 and increment the juno1 contract count
	juno2Increment, err := juno2Chain.ExecuteContract(ctx, juno2User.KeyName(), juno2ContractAddr, juno2IncrementMsg)
	require.NoError(t, err)
	require.Equal(t, uint32(0), juno2Increment.Code)

	// Wait for the ibc packet to be delivered
	err = testutil.WaitForBlocks(ctx, 2, juno1, juno2)
	require.NoError(t, err)

	// Query the count of the contract on juno1- should still be 1 as a single packet has been sent through
	var juno1IncrementedCountResponse CwIbcCountResponse
	err = juno1Chain.QueryContract(ctx, juno1ContractAddr, queryJuno1CountMsg, &juno1IncrementedCountResponse)
	require.NoError(t, err)
	require.Equal(t, 1, juno1IncrementedCountResponse.Data.Count)

	// Query the count of the contract on juno2- should be 1 as a single packet has now been sent through from juno1 to juno2
	var juno2PreIncrementedCountResponse CwIbcCountResponse
	err = juno2Chain.QueryContract(ctx, juno2ContractAddr, queryJuno2CountMsg, &juno2PreIncrementedCountResponse)
	require.NoError(t, err)
	require.Equal(t, 1, juno2PreIncrementedCountResponse.Data.Count)

}

// cw_ibc_example response data
type CwIbcCountResponse struct {
	Data struct {
		Count int `json:"count"`
	} `json:"data"`
}
