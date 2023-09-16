package wasm

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v7"
	cosmosChain "github.com/strangelove-ventures/interchaintest/v7/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v7/chain/cosmos/wasm"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/internal/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v7/relayer"
	"github.com/strangelove-ventures/interchaintest/v7/testreporter"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestInterchainQueriesWASM is a test case that performs a round trip query from an ICQ wasm contract <> ICQ module.
// On the sender chain, CosmWasm capability is required to instantiate/execute the smart contract. On the receiver chain,
// the ICQ module is required to be present in order to receive interchain queries.
func TestInterchainQueriesWASM(t *testing.T) {
	//TODO (1): force relayer to use specific versions of the chains configured in the file.
	//os.Setenv("IBCTEST_CONFIGURED_CHAINS", "./icq_wasm_configured_chains.yaml")

	//TODO (2): use Juno as sender "ghcr.io/strangelove-ventures/heighliner/juno:v10.1.0"
	//and Strangelove's icqd (or another chain with ICQ module present) as receiver.

	logger := zaptest.NewLogger(t)

	if testing.Short() {
		t.Skip()
	}

	client, network := interchaintest.DockerSetup(t)
	f, err := interchaintest.CreateLogFile(fmt.Sprintf("wasm_ibc_test_%d.json", time.Now().Unix()))
	require.NoError(t, err)
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)
	ctx := context.Background()
	contractFilePath := "sample_contracts/icq.wasm" //Contract that will be initialized on chain

	wasmImage := ibc.DockerImage{
		Repository: "ghcr.io/strangelove-ventures/heighliner/wasmd",
		Version:    "v0.0.1",
		UidGid:     dockerutil.GetHeighlinerUserString(),
	}

	genesisAllowICQ := map[string]interface{}{
		"interchainquery": map[string]interface{}{
			"host_port": "icqhost",
			"params": map[string]interface{}{
				"host_enabled":  true,
				"allow_queries": []interface{}{"/cosmos.bank.v1beta1.Query/AllBalances"},
			},
		},
	}

	minVal := 1
	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			ChainName:     "sender",
			NumValidators: &minVal,
			ChainConfig: ibc.ChainConfig{
				Type:           "cosmos",
				Name:           "sender",
				ChainID:        "sender",
				Images:         []ibc.DockerImage{wasmImage},
				Bin:            "wasmd",
				Bech32Prefix:   "wasm",
				Denom:          "uatom",
				GasPrices:      "0.00uatom",
				TrustingPeriod: "300h",
				GasAdjustment:  1.1,
				EncodingConfig: wasm.WasmEncoding(),
				ModifyGenesis:  modifyGenesisAtPath(genesisAllowICQ, "app_state"),
			}},
		{
			ChainName:     "receiver",
			NumValidators: &minVal,
			ChainConfig: ibc.ChainConfig{
				Type:           "cosmos",
				Name:           "receiver",
				ChainID:        "receiver",
				Images:         []ibc.DockerImage{wasmImage},
				Bin:            "wasmd",
				Bech32Prefix:   "wasm",
				Denom:          "uatom",
				GasPrices:      "0.00uatom",
				TrustingPeriod: "300h",
				GasAdjustment:  1.1,
				EncodingConfig: wasm.WasmEncoding(),
				ModifyGenesis:  modifyGenesisAtPath(genesisAllowICQ, "app_state"),
			}},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chain1, chain2 := chains[0], chains[1]

	// Get a relayer instance
	r := interchaintest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		logger,
		relayer.StartupFlags("-p", "events", "-b", "100"),
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
		})

	logger.Info("ic.Build()")
	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation: false,
	}))

	// Wait a few blocks for user accounts to be created on chain
	logger.Info("wait for user accounts")
	err = testutil.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	// Fund user accounts so we can query balances
	chain1UserAmt := math.NewInt(10_000_000_000)
	chain2UserAmt := math.NewInt(99_999_999_999)
	chain1User := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), chain1UserAmt.Int64(), chain1)[0]
	chain2User := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), chain2UserAmt.Int64(), chain2)[0]

	err = testutil.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	chain1UserAddress := chain1User.FormattedAddress()
	require.NotEmpty(t, chain1UserAddress)

	chain2UserAddress := chain2User.FormattedAddress()
	logger.Info("Address", zap.String("chain 2 user", chain2UserAddress))
	require.NotEmpty(t, chain2UserAddress)

	chain1UserBalInitial, err := chain1.GetBalance(ctx, chain1UserAddress, chain1.Config().Denom)
	require.NoError(t, err)
	require.True(t, chain1UserBalInitial.Equal(chain1UserAmt))

	chain2UserBalInitial, err := chain2.GetBalance(ctx, chain2UserAddress, chain2.Config().Denom)
	require.NoError(t, err)
	require.True(t, chain2UserBalInitial.Equal(chain2UserAmt))

	logger.Info("instantiating contract")
	initMessage := "{\"default_timeout\": 1000}"
	chain1CChain := chain1.(*cosmosChain.CosmosChain)

	wasmIcqCodeId, err := chain1CChain.StoreContract(ctx, chain1User.KeyName(), contractFilePath)
	require.NoError(t, err)
	err = testutil.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	//Instantiate the smart contract on the test chain, facilitating testing of ICQ WASM functionality
	contractAddr, err := chain1CChain.InstantiateContract(ctx, chain1User.KeyName(), wasmIcqCodeId, initMessage, true)
	require.NoError(t, err)
	logger.Info("icq contract deployed", zap.String("contractAddr", contractAddr))

	err = testutil.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	icqWasmPortId := "wasm." + contractAddr
	destPort := "icqhost"
	// Create channel between icq wasm contract <> icq module.
	err = r.CreateChannel(ctx, eRep, pathName, ibc.CreateChannelOptions{
		SourcePortName: icqWasmPortId,
		DestPortName:   destPort,
		Order:          ibc.Unordered,
		Version:        "icq-1",
	})
	require.NoError(t, err)
	err = testutil.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	// Query for the recently created channel-id.
	chain2Channels, err := r.GetChannels(ctx, eRep, chain2.Config().ChainID)
	require.NoError(t, err)

	for _, c1 := range chain2Channels {
		logger.Info("Channel", zap.String("Info", fmt.Sprintf("Channel ID: %s, Port ID: %s, Version: %s, Counterparty Channel ID: %s, Counterparty Port ID: %s", c1.ChannelID, c1.PortID, c1.Version, c1.Counterparty.ChannelID, c1.Counterparty.PortID)))
	}

	require.NoError(t, err)
	channel := FirstWithPort(chain2Channels, destPort)
	require.NotNil(t, channel)
	require.NotEmpty(t, channel.Counterparty.ChannelID)

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
	logger.Info("channel", zap.String("info", fmt.Sprintf("Channel Port: %s, Channel ID: %s, Counterparty Channel ID: %s", channel.PortID, channel.ChannelID, channel.Counterparty.ChannelID)))

	//Query for the balances of an account on the counterparty chain using interchain queries.
	//Get the base64 encoded chain2 user address in the format required by the AllBalances query
	chain2UserAddrQuery := fmt.Sprintf(`{"address":"%s"}`, chain2UserAddress)
	chain2UserAddrQueryB64 := base64.StdEncoding.EncodeToString([]byte(chain2UserAddrQuery))

	// Get current block height for chain 2
	cmd := []string{chain2.Config().Bin, "status",
		"--node", chain2.GetRPCAddress(),
		"--home", chain2.HomeDir(),
	}
	stdout, _, err := chain2.Exec(ctx, cmd, nil)
	require.NoError(t, err)
	blockHeightC2 := &statusResults{}
	err = json.Unmarshal(stdout, blockHeightC2)
	require.NoError(t, err)

	//and chain 1
	// Get current block height
	cmd = []string{chain1.Config().Bin, "status",
		"--node", chain1.GetRPCAddress(),
		"--home", chain1.HomeDir(),
	}
	stdout, _, err = chain1.Exec(ctx, cmd, nil)
	require.NoError(t, err)
	blockHeightC1 := &statusResults{}
	err = json.Unmarshal(stdout, blockHeightC1)
	require.NoError(t, err)

	logger.Info("Chain height", zap.String("Chain 1", blockHeightC1.SyncInfo.Height), zap.String("Chain 2", blockHeightC2.SyncInfo.Height))

	query := executeQuery{
		Query: msgQuery{
			Timeout: 1000,
			Channel: channel.ChannelID,
			Requests: []RequestQuery{ //can't use abci.RequestQuery since Height/Prove JSON fields are omitted which causes contract errors
				{
					Height: 0,
					Prove:  false,
					Path:   "/cosmos.bank.v1beta1.Query/AllBalances",
					Data:   []byte(chain2UserAddrQueryB64),
				},
			},
		},
	}

	b, err := json.Marshal(query)
	require.NoError(t, err)
	msg := string(b)
	logger.Info("Executing msg ->", zap.String("msg", msg))

	//Query the contract on chain 1. The contract makes an interchain query to chain 2 to get the chain 2 user's balance.
	hash, err := chain1CChain.ExecuteContract(ctx, chain1User.KeyName(), contractAddr, msg)

	require.NoError(t, err)

	// Check the results from the interchain query above.
	cmd = []string{chain1.Config().Bin, "query", "tx", hash,
		"--node", chain1.GetRPCAddress(),
		"--home", chain1.HomeDir(),
		"--chain-id", chain1.Config().ChainID,
		"--output", "json",
	}
	_, _, err = chain1.Exec(ctx, cmd, nil)
	require.NoError(t, err)

	// Wait a few blocks for query to be sent to counterparty.
	err = testutil.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	// Check the results from the interchain query above.
	cmd = []string{chain1.Config().Bin, "query", "wasm", "contract-state", "all", contractAddr,
		"--node", chain1.GetRPCAddress(),
		"--home", chain1.HomeDir(),
		"--chain-id", chain1.Config().ChainID,
		"--output", "json",
	}

	stdout, _, err = chain1.Exec(ctx, cmd, nil)
	require.NoError(t, err)
	results := &contractStateResp{}
	err = json.Unmarshal(stdout, results)
	require.NoError(t, err)
	hasIcqQuery := false

	for _, kv := range results.Models {
		keyBytes, _ := hex.DecodeString(kv.Key)
		valueBytes, err := base64.StdEncoding.DecodeString(kv.Value)
		require.NoError(t, err)
		if string(keyBytes) == "query_result_counter" {
			res, err := strconv.Atoi(string(valueBytes))
			require.NoError(t, err)
			if res > 0 {
				hasIcqQuery = true
				logger.Info("ICQ query result counter", zap.Int("counter", res))
			}
		}
	}
	require.Equal(t, hasIcqQuery, true)
}

func FirstWithPort(channels []ibc.ChannelOutput, port string) *ibc.ChannelOutput {
	for _, channel := range channels {
		if channel.PortID == port {
			return &channel
		}
	}
	return nil
}

type RequestQuery struct {
	Data   []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
	Path   string `protobuf:"bytes,2,opt,name=path,proto3" json:"path,omitempty"`
	Height int64  `protobuf:"varint,3,opt,name=height,proto3" json:"height"` //do NOT 'omitempty' for JSON field or contract queries will error
	Prove  bool   `protobuf:"varint,4,opt,name=prove,proto3" json:"prove"`   //do NOT 'omitempty' for JSON field or contract queries will error
}

type msgQuery struct {
	Channel  string         `json:"channel"`
	Requests []RequestQuery `json:"requests"`
	Timeout  uint64         `json:"timeout"`
}

type executeQuery struct {
	Query msgQuery `json:"query"`
}

type kvPair struct {
	Key   string // hex encoded string
	Value string // b64 encoded json
}

type contractStateResp struct {
	Models []kvPair
}

type statusResults struct {
	SyncInfo struct {
		Height string `json:"latest_block_height"`
	} `json:"SyncInfo"`
}

func modifyGenesisAtPath(insertedBlock map[string]interface{}, key string) func(ibc.ChainConfig, []byte) ([]byte, error) {
	return func(chainConfig ibc.ChainConfig, genbz []byte) ([]byte, error) {
		g := make(map[string]interface{})
		if err := json.Unmarshal(genbz, &g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis file: %w", err)
		}

		//Get the section of the genesis file under the given key (e.g. "app_state")
		genesisBlockI, ok := g[key]
		if !ok {
			return nil, fmt.Errorf("genesis json does not have top level key: %s", key)
		}

		blockBytes, mErr := json.Marshal(genesisBlockI)
		if mErr != nil {
			return nil, fmt.Errorf("genesis json marshal error for block with key: %s", key)
		}

		genesisBlock := make(map[string]interface{})
		mErr = json.Unmarshal(blockBytes, &genesisBlock)
		if mErr != nil {
			return nil, fmt.Errorf("genesis json unmarshal error for block with key: %s", key)
		}

		for k, v := range insertedBlock {
			genesisBlock[k] = v
		}

		g[key] = genesisBlock
		out, err := json.Marshal(g)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
		}
		return out, nil
	}
}
