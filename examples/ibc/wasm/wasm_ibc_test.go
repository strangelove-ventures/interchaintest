package wasm_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos/wasm"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
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
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create and Fund User Wallets
	initBal := math.NewInt(100_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", initBal.Int64(), juno1, juno2)
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
				t.Logf("an error occured while stopping the relayer: %s", err)
			}
		},
	)

	juno1Chain := juno1.(*cosmos.CosmosChain)
	juno2Chain := juno2.(*cosmos.CosmosChain)

	// Store ibc_reflect_send.wasm contract
	ibcReflectSendCodeId, err := juno1Chain.StoreContract(
		ctx, juno1User.KeyName(), "sample_contracts/ibc_reflect_send.wasm")
	require.NoError(t, err)

	// Instantiate ibc_reflect_send.wasm contract
	ibcReflectSendContractAddr, err := juno1Chain.InstantiateContract(
		ctx, juno1User.KeyName(), ibcReflectSendCodeId, "{}", true)
	require.NoError(t, err)

	// Store reflect.wasm contract
	reflectCodeId, err := juno2Chain.StoreContract(
		ctx, juno2User.KeyName(), "sample_contracts/reflect.wasm")
	require.NoError(t, err)

	// Instantiate reflect.wasm contract
	_, err = juno2Chain.InstantiateContract(
		ctx, juno2User.KeyName(), reflectCodeId, "{}", true)
	require.NoError(t, err)

	// Store ibc_reflect.wasm contract
	ibcReflectCodeId, err := juno2Chain.StoreContract(
		ctx, juno2User.KeyName(), "sample_contracts/ibc_reflect.wasm")
	require.NoError(t, err)

	// Instantiate ibc_reflect_send.wasm contract
	initMsg := "{\"reflect_code_id\":" + reflectCodeId + "}"
	ibcReflectContractAddr, err := juno2Chain.InstantiateContract(
		ctx, juno2User.KeyName(), ibcReflectCodeId, initMsg, true)
	require.NoError(t, err)

	err = testutil.WaitForBlocks(ctx, 2, juno1, juno2)
	require.NoError(t, err)

	// Set up channel
	ibcReflectSendPortId := "wasm." + ibcReflectSendContractAddr
	ibcReflectPortId := "wasm." + ibcReflectContractAddr
	err = r.CreateChannel(ctx, eRep, ibcPath, ibc.CreateChannelOptions{
		SourcePortName: ibcReflectSendPortId,
		DestPortName:   ibcReflectPortId,
		Order:          ibc.Ordered,
		Version:        "ibc-reflect-v1",
	})
	require.NoError(t, err)

	// Wait for the channel to get set up and whoami message to exchange
	err = testutil.WaitForBlocks(ctx, 10, juno1, juno2)
	require.NoError(t, err)

	// Get contract channel
	juno1ChannelInfo, err := r.GetChannels(ctx, eRep, juno1.Config().ChainID)
	require.NoError(t, err)
	juno1ChannelID := juno1ChannelInfo[len(juno1ChannelInfo)-1].ChannelID

	// Query ibc_reflect_send contract on Juno1 for remote address (populated via ibc)
	queryMsg := ReflectSendQueryMsg{Account: &AccountQuery{ChannelID: juno1ChannelID}}
	var ibcReflectSendResponse IbcReflectSendResponseData
	err = juno1Chain.QueryContract(ctx, ibcReflectSendContractAddr, queryMsg, &ibcReflectSendResponse)
	require.NoError(t, err)
	require.NotEmpty(t, ibcReflectSendResponse.Data.RemoteAddr)

	// Query ibc_reflect contract on Juno2 for local account address
	var ibcReflectResponse IbcReflectResponseData
	err = juno2Chain.QueryContract(ctx, ibcReflectContractAddr, queryMsg, &ibcReflectResponse)
	require.NoError(t, err)
	require.NotEmpty(t, ibcReflectResponse.Data.Account)

	// Verify that these addresses match, a match is a successful test run
	//    - ibc_reflect_send contract (Juno1) remote address (retrieved via ibc)
	//    - ibc_reflect contract (Juno2) account address populated locally
	require.Equal(t, ibcReflectSendResponse.Data.RemoteAddr, ibcReflectResponse.Data.Account)
}

type ReflectSendQueryMsg struct {
	Admin        *struct{}     `json:"admin,omitempty"`
	ListAccounts *struct{}     `json:"list_accounts,omitempty"`
	Account      *AccountQuery `json:"account,omitempty"`
}

type AccountQuery struct {
	ChannelID string `json:"channel_id"`
}

type Coin struct {
	Denom  string `json:"denom"`  // type, eg. "ATOM"
	Amount string `json:"amount"` // string encoing of decimal value, eg. "12.3456"
}

type Coins []Coin

type IbcReflectSendAccountResponse struct {
	LastUpdateTime uint64 `json:"last_update_time,string"`
	RemoteAddr     string `json:"remote_addr"`
	RemoteBalance  Coins  `json:"remote_balance"`
}

// ibc_reflect_send response data
type IbcReflectSendResponseData struct {
	Data IbcReflectSendAccountResponse `json:"data"`
}

type IbcReflectAccountResponse struct {
	Account string `json:"account"`
}

// ibc_reflect response data
type IbcReflectResponseData struct {
	Data IbcReflectAccountResponse `json:"data"`
}
