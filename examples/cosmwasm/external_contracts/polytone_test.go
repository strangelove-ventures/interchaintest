package polytone_test

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos/wasm"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestPolytoneDeployment(t *testing.T) {
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
	f, err := interchaintest.CreateLogFile(fmt.Sprintf("polytone_deployment_%d.json", time.Now().Unix()))
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

	// Deploy polytone contracts
	polytoneContracts, err := juno1Chain.SetupPolytone(
		ctx,
		r,
		eRep,
		ibcPath,
		juno1User.KeyName(),
		juno2Chain,
		juno2User.KeyName(),
	)
	require.NoError(t, err)
	require.NotEmpty(t, polytoneContracts)

	// Wait for the channel to get set up
	err = testutil.WaitForBlocks(ctx, 2, juno1, juno2)
	require.NoError(t, err)

	// Upload and instantiate polytone_tester contract
	testerContract, err := juno1Chain.UploadAndInstantiateContract(ctx, juno1User.KeyName(), "../../../external_contracts/polytone/v1.0.0/polytone_tester.wasm", "{}", "polytone_tester", true)
	require.NoError(t, err)
	require.NotEmpty(t, testerContract)

	log.Println("Querying contract on Juno1 for remote address (populated via ibc) ", polytoneContracts.ChannelID)

	roundtripExec, err := juno1Chain.ExecuteContract(ctx,
		juno1User.KeyName(),
		polytoneContracts.Note.Address,
		fmt.Sprintf(`{"execute": {"msgs": [], "timeout_seconds": "100", "callback": {"receiver": "%s", "msg": "aGVsbG8K"}}}`,
			testerContract.Address))
	require.NoError(t, err)
	require.Equal(t, uint32(0), roundtripExec.Code)

	// Wait for the packet to be relayed
	err = testutil.WaitForBlocks(ctx, 2, juno1, juno2)
	require.NoError(t, err)

	var activeChannelResponse NoteRemoteAddressResponse
	err = juno1Chain.NullableQueryContract(ctx, polytoneContracts.Note.Address,
		fmt.Sprintf(`{"remote_address": {"local_address": "%v"}}`, juno1User.FormattedAddress()), activeChannelResponse)
	require.NoError(t, err)
	log.Printf("activeChannelResponse: %v", activeChannelResponse)
	// require.NotEmpty(t, activeChannelResponse.Data)

	// var pairResponse NotePairResponse
	// err = juno1Chain.NullableQueryContract(ctx, polytoneContracts.Note.Address, fmt.Sprintf(`{"pair":{}}`), &pairResponse)
	// require.NoError(t, err)
	// log.Printf("pairResponse: %v", pairResponse)
	// require.NotEmpty(t, pairResponse.Data)

	// // Query ibc_reflect_send contract on Juno1 for remote address (populated via ibc)
	// var ibcReflectSendResponse IbcReflectSendResponseData
	// err = juno1Chain.QueryContract(ctx, ibcReflectSendContractAddr, queryMsg, &ibcReflectSendResponse)
	// require.NoError(t, err)
	// require.NotEmpty(t, ibcReflectSendResponse.Data.RemoteAddr)

	// // Query ibc_reflect contract on Juno2 for local account address
	// var ibcReflectResponse IbcReflectResponseData
	// err = juno2Chain.QueryContract(ctx, ibcReflectContractAddr, queryMsg, &ibcReflectResponse)
	// require.NoError(t, err)
	// require.NotEmpty(t, ibcReflectResponse.Data.Account)

	// // Verify that these addresses match, a match is a successful test run
	// //    - ibc_reflect_send contract (Juno1) remote address (retrieved via ibc)
	// //    - ibc_reflect contract (Juno2) account address populated locally
	// require.Equal(t, ibcReflectSendResponse.Data.RemoteAddr, ibcReflectResponse.Data.Account)
}

type Coin struct {
	Denom  string `json:"denom"`  // type, eg. "ATOM"
	Amount string `json:"amount"` // string encoing of decimal value, eg. "12.3456"
}

type Coins []Coin

type NoteActiveChannelResponse struct {
	Data string `json:"data"`
}

type NotePairResponse struct {
	Data struct {
		Pair struct {
			ConnectionID string `json:"connection_id"`
			RemotePort   string `json:"remote_port"`
		} `json:"pair"`
	} `json:"data",omitempty`
}

type NoteRemoteAddressResponse struct {
	Data string `json:"data",omimtempty`
}
