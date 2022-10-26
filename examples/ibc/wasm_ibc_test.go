package ibc_test

import (
	"context"
	"fmt"
	"testing"
	"time"
	"encoding/json"

	//transfertypes "github.com/cosmos/ibc-go/v6/modules/apps/transfer/types"
	"github.com/strangelove-ventures/ibctest/v6"
	"github.com/strangelove-ventures/ibctest/v6/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/strangelove-ventures/ibctest/v6/test"
	"github.com/strangelove-ventures/ibctest/v6/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// This test is meant to be used as a basic ibctest tutorial.
// Code snippets are broken down in ./docs/upAndRunning.md
func TestWasmIbc(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	ctx := context.Background()

	// Chain Factory
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "juno", ChainName: "juno1", Version: "latest", ChainConfig: ibc.ChainConfig{
			GasPrices:  "0.0025ujuno",
		}},
		{Name: "juno", ChainName: "juno2", Version: "latest", ChainConfig: ibc.ChainConfig{
			GasPrices:  "0.0025ujuno",
		}},

	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	juno1, juno2 := chains[0], chains[1]

	// Relayer Factory
	client, network := ibctest.DockerSetup(t)
	r := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
		t, client, network)

	// Prep Interchain
	const ibcPath = "wasmpath"
	ic := ibctest.NewInterchain().
		AddChain(juno1).
		AddChain(juno2).
		AddRelayer(r, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  juno1,
			Chain2:  juno2,
			Relayer: r,
			Path:    ibcPath,
			CreateChannelOpts: ibc.CreateChannelOptions{
				SourcePortName: "ibcA",
				DestPortName: "ibcB",
				Order: ibc.Ordered,
				Version: "ibc-reflect-v1",
			},
		})

	// Log location
	f, err := ibctest.CreateLogFile(fmt.Sprintf("wasm_ibc_test_%d.json", time.Now().Unix()))
	require.NoError(t, err)
	// Reporter/logs
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)

	// Build interchain
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(),

		SkipPathCreation: false},
	),
	)
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Create and Fund User Wallets
	fundAmount := int64(100_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, "default", int64(fundAmount), juno1, juno2)
	juno1User := users[0]
	juno2User := users[1]

	err = test.WaitForBlocks(ctx, 5, juno1, juno2)
	require.NoError(t, err)

	juno1UserBalInitial, err := juno1.GetBalance(ctx, juno1User.Bech32Address(juno1.Config().Bech32Prefix), juno1.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, fundAmount, juno1UserBalInitial)

	juno2UserBalInitial, err := juno2.GetBalance(ctx, juno2User.Bech32Address(juno2.Config().Bech32Prefix), juno2.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, fundAmount, juno2UserBalInitial)

	// Get Channel ID
	/*juno1ChannelInfo, err := r.GetChannels(ctx, eRep, juno1.Config().ChainID)
	require.NoError(t, err)
	juno1ChannelID := juno1ChannelInfo[0].ChannelID

	juno2ChannelInfo, err := r.GetChannels(ctx, eRep, juno2.Config().ChainID)
	require.NoError(t, err)
	juno2ChannelID := juno2ChannelInfo[0].ChannelID*/

	// Start the relayer on both paths
	err = r.StartRelayer(ctx, eRep, ibcPath)
	require.NoError(t, err)
	//r.

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

	juno1ContractAddr, _, err := juno1Chain.InstantiateContract(
		ctx, 
		juno1User.KeyName, 
		ibc.WalletAmount{
			Address: juno1User.Bech32Address(juno1.Config().Bech32Prefix),
			Denom: juno1.Config().Denom,
			Amount: int64(10_000_000),
		},
		"sample_contracts/ibc_reflect_send.wasm",
		"{}",
		true,
	)
	require.NoError(t, err)
	

	_, codeId, err := juno2Chain.InstantiateContract(
		ctx, 
		juno2User.KeyName, 
		ibc.WalletAmount{
			Address: juno2User.Bech32Address(juno2.Config().Bech32Prefix),
			Denom: juno2.Config().Denom,
			Amount: int64(1_000_000),
		},
		"sample_contracts/reflect.wasm",
		"{}",
		true,
	)
	require.NoError(t, err)

	initMsg := "{\"reflect_code_id\":" + codeId + "}"
	juno2ContractAddr, _, err := juno2Chain.InstantiateContract(
		ctx, 
		juno2User.KeyName, 
		ibc.WalletAmount{
			Address: juno2User.Bech32Address(juno2.Config().Bech32Prefix),
			Denom: juno2.Config().Denom,
			Amount: int64(1_000_000),
		},
		"sample_contracts/ibc_reflect.wasm",
		initMsg,
		true,
	)
	require.NoError(t, err)

	err = test.WaitForBlocks(ctx, 5, juno1, juno2)
	require.NoError(t, err)

	juno1Height, err := juno1.Height(ctx)
	require.NoError(t, err, "error fetching juno1 height")
	juno2Height, err := juno2.Height(ctx)
	require.NoError(t, err, "error fetching juno2 height")

	juno1ContractState, err := juno1Chain.DumpContractState(ctx, juno1ContractAddr, int64(juno1Height))
	t.Logf("Juno1ContractState (1) %s\n", *juno1ContractState)
	juno2ContractState, err := juno2Chain.DumpContractState(ctx, juno2ContractAddr, int64(juno2Height))
	t.Logf("Juno2ContractState (1) %s\n", *juno2ContractState)
	//err = juno1.(*cosmos.CosmosChain).ExecuteContract(ctx, juno1User.KeyName, juno1ContractAddr, "message")
	//require.NoError(t, err)

	//juno1ChannelInfo, err := r.GetChannels(ctx, eRep, juno1.Config().ChainID)
	//require.NoError(t, err)
	//juno1ChannelID := juno1ChannelInfo[0].ChannelID

	queryMsg := ReflectSendQueryMsg{Account: &AccountQuery{ChannelID: "ibcA"}}
	query, err := json.Marshal(queryMsg)
	require.NoError(t, err)


	//query := "{\"channel_id\"=\"channel-1\"}"
	//stdout, _, err := juno1Chain.Exec(ctx, "query", "wasm", "contract-state", "smart", juno1ContractAddr)

	stdout, _, err := juno1Chain.QueryContract(ctx, juno1ContractAddr, string(query))
	require.NoError(t, err)

	accountRes := AccountResponse{}
	err = json.Unmarshal([]byte(stdout), &accountRes)
	require.NoError(t, err)
	t.Logf("Juno1Query RemoteAddr %s", accountRes.RemoteAddr)

	err = test.WaitForBlocks(ctx, 5, juno1, juno2)
	require.NoError(t, err)

	juno1Height, err = juno1.Height(ctx)
	require.NoError(t, err, "error fetching juno1 (2) height")
	juno2Height, err = juno2.Height(ctx)
	require.NoError(t, err, "error fetching juno2 (2) height")

	juno1ContractState, err = juno1Chain.DumpContractState(ctx, juno1ContractAddr, int64(juno1Height))
	t.Logf("Juno1ContractState (2) %s\n", *juno1ContractState)
	juno2ContractState, err = juno2Chain.DumpContractState(ctx, juno2ContractAddr, int64(juno2Height))
	t.Logf("Juno2ContractState (2) %s\n", *juno2ContractState)
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

type AccountResponse struct {
	LastUpdateTime uint64            `json:"last_update_time,string"`
	RemoteAddr     string            `json:"remote_addr"`
	RemoteBalance  Coins 		 	 `json:"remote_balance"`
}