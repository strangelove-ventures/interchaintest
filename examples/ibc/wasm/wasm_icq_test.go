package wasm

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	//"github.com/icza/dyno"
	"os"
	"testing"
	"time"

	"github.com/strangelove-ventures/ibctest/v3"
	cosmosChain "github.com/strangelove-ventures/ibctest/v3/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v3/chain/cosmos/wasm"
	"github.com/strangelove-ventures/ibctest/v3/ibc"
	"github.com/strangelove-ventures/ibctest/v3/internal/dockerutil"
	"github.com/strangelove-ventures/ibctest/v3/relayer"
	"github.com/strangelove-ventures/ibctest/v3/test"
	"github.com/strangelove-ventures/ibctest/v3/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// CW-20 packet transfer
// TestInterchainQueriesWASM is a test case that performs a round trip query from an ICQ wasm contract <> ICQ module.
func TestInterchainQueriesWASM(t *testing.T) {
	logger := zaptest.NewLogger(t)

	if testing.Short() {
		t.Skip()
	}

	client, network := ibctest.DockerSetup(t)
	f, err := ibctest.CreateLogFile(fmt.Sprintf("wasm_ibc_test_%d.json", time.Now().Unix()))
	require.NoError(t, err)
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)
	ctx := context.Background()
	contractFilePath := "sample_contracts/icq.wasm" //Contract that will be initialized on chain

	//Supports receiving interchain queries. This means the ICQ Module is present.
	// interchainQueriesImage := ibc.DockerImage{
	// 	Repository: "ghcr.io/strangelove-ventures/heighliner/icqd",
	// 	Version:    "latest",
	// 	UidGid:     dockerutil.GetHeighlinerUserString(),
	// }

	wasmImage := ibc.DockerImage{
		Repository: "kyle/wasmd", //"ghcr.io/polymerdao/wasmd",
		Version:    "v1.0",       //"latest",
		UidGid:     dockerutil.GetHeighlinerUserString(),
	}

	os.Setenv("EXPORT_GENESIS_FILE_PATH", "/home/kyle/projects/Strangelove/ibctest/juno_genesis_icq.json")
	os.Setenv("EXPORT_GENESIS_CHAIN", "juno1")

	//Forces relayer to use the specific versions of the chains configured in the file.
	//These match our DockerImage versions below (from this test case)
	//os.Setenv("IBCTEST_CONFIGURED_CHAINS", "./icq_wasm_configured_chains.yaml")

	//sender
	// junoDockerImage := ibc.DockerImage{
	// 	Repository: "ghcr.io/strangelove-ventures/heighliner/juno",
	// 	Version:    "v10.1.0",
	// 	UidGid:     dockerutil.GetHeighlinerUserString(),
	// }

	// gaiaDockerImage := ibc.DockerImage{
	// 	Repository: "ghcr.io/strangelove-ventures/heighliner/gaia",
	// 	Version:    "v7.1.0",
	// 	UidGid:     dockerutil.GetHeighlinerUserString(),
	// }

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
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
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
				ModifyGenesis:  modifyGenesisAtPath(genesisAllowICQ, "app_state"), //modifyGenesisAllowICQQueries([]string{"/cosmos.bank.v1beta1.Query/AllBalances"}), //modifyGenesisAtPath(genesisAllowICQ, "app_state"),
			}},
	})

	// Chain Factory
	// cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
	// 	{Name: "juno", ChainName: "juno1", Version: "latest", ChainConfig: ibc.ChainConfig{
	// 		GasPrices:      "0.00ujuno",
	// 		ModifyGenesis:  modifyGenesisAtPath(genesisAllowICQ, "app_state"),
	// 		EncodingConfig: wasm.WasmEncoding(),
	// 	}},
	// 	{Name: "juno", ChainName: "juno2", Version: "latest", ChainConfig: ibc.ChainConfig{
	// 		GasPrices:      "0.00ujuno",
	// 		ModifyGenesis:  modifyGenesisAtPath(genesisAllowICQ, "app_state"),
	// 		EncodingConfig: wasm.WasmEncoding(),
	// 	}},
	// })

	// minVal := 1
	// cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
	// 	{
	// 		ChainName:     "sender",
	// 		NumValidators: &minVal,
	// 		ChainConfig: ibc.ChainConfig{
	// 			Type:           "cosmos",
	// 			Name:           "sender",
	// 			ChainID:        "sender",
	// 			Images:         []ibc.DockerImage{junoDockerImage},
	// 			Bin:            "junod",
	// 			Bech32Prefix:   "juno",
	// 			Denom:          "ujuno",
	// 			GasPrices:      "0.00ujuno",
	// 			TrustingPeriod: "300h",
	// 			GasAdjustment:  1.1,
	// 			ModifyGenesis:  modifyGenesisAtPath(genesisAllowICQ, "app_state"),
	// 		}},
	// 	{
	// 		ChainName:     "receiver",
	// 		NumValidators: &minVal,
	// 		ChainConfig: ibc.ChainConfig{
	// 			Type:           "cosmos",
	// 			Name:           "receiver",
	// 			ChainID:        "receiver",
	// 			Images:         []ibc.DockerImage{gaiaDockerImage},
	// 			Bin:            "gaiad",
	// 			Bech32Prefix:   "cosmos",
	// 			Denom:          "uatom",
	// 			GasPrices:      "0.00uatom",
	// 			TrustingPeriod: "300h",
	// 			GasAdjustment:  1.1,
	// 			ModifyGenesis:  modifyGenesisAtPath(genesisAllowICQ, "app_state"),
	// 		}},
	// })

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	chain1, chain2 := chains[0], chains[1]

	// Get a relayer instance

	r := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		logger,
		relayer.RelayerOptionExtraStartFlags{Flags: []string{"-p", "events", "-b", "100"}},
	).Build(t, client, network)

	// Build the network; spin up the chains and configure the relayer
	const pathName = "test1-test2"
	const relayerName = "relayer"

	ic := ibctest.NewInterchain().
		AddChain(chain1).
		AddChain(chain2).
		AddRelayer(r, relayerName).
		AddLink(ibctest.InterchainLink{
			Chain1:  chain1,
			Chain2:  chain2,
			Relayer: r,
			Path:    pathName,
		})

	logger.Info("ic.Build()")
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(), //filepath.Join(tmpDir, ".ibctest", "databases", "block.db"), //
		SkipPathCreation:  false,
	}))

	// Wait a few blocks for user accounts to be created on chain
	logger.Info("wait for user accounts")
	err = test.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	// Fund user accounts so we can query balances
	chain1UserAmt := int64(10_000_000_000)
	chain2UserAmt := int64(99_999_999_999)
	chain1User := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), chain1UserAmt, chain1)[0]
	chain2User := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), chain2UserAmt, chain2)[0]

	err = test.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	chain1UserAddress := chain1User.Bech32Address(chain1.Config().Bech32Prefix)
	require.NotEmpty(t, chain1UserAddress)

	chain2UserAddress := chain2User.Bech32Address(chain2.Config().Bech32Prefix)
	logger.Info("Address", zap.String("chain 2 user", chain2UserAddress))
	require.NotEmpty(t, chain2UserAddress)

	chain1UserBalInitial, err := chain1.GetBalance(ctx, chain1UserAddress, chain1.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, chain1UserAmt, chain1UserBalInitial)

	chain2UserBalInitial, err := chain2.GetBalance(ctx, chain2UserAddress, chain2.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, chain2UserAmt, chain2UserBalInitial)

	logger.Info("instantiating contract")
	initMessage := "{\"default_timeout\": 1000}"
	chain1CChain := chain1.(*cosmosChain.CosmosChain)

	wasmIcqCodeId, err := chain1CChain.StoreContract(ctx, chain1User.KeyName, contractFilePath)
	require.NoError(t, err)
	err = test.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	//Instantiate the smart contract on the Juno test chain, facilitating testing of ICQ WASM functionality
	//ibc.WalletAmount{Amount: 5000000}, contractFilePath,
	contractAddr, err := chain1CChain.InstantiateContract(ctx, chain1User.KeyName, wasmIcqCodeId, initMessage, true)
	require.NoError(t, err)
	logger.Info("icq contract deployed", zap.String("contractAddr", contractAddr))

	err = test.WaitForBlocks(ctx, 5, chain1, chain2)
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
	err = test.WaitForBlocks(ctx, 5, chain1, chain2)
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
	err = test.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)
	logger.Info("channel", zap.String("info", fmt.Sprintf("Channel Port: %s, Channel ID: %s, Counterparty Channel ID: %s", channel.PortID, channel.ChannelID, channel.Counterparty.ChannelID)))

	// Query for the balances of an account on the counterparty chain using interchain queries.
	//Get the base64 encoded Gaia user address in the format required by the AllBalances query
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

	// heightC1, err := strconv.ParseInt(blockHeightC1.SyncInfo.Height, 10, 64)
	// require.NoError(t, err)
	// heightC2, err := strconv.ParseInt(blockHeightC2.SyncInfo.Height, 10, 64)
	// require.NoError(t, err)

	query := executeQuery{
		Query: msgQuery{
			Timeout: 10000,
			Channel: channel.ChannelID,
			Requests: []RequestQuery{ //can't use abci.RequestQuery since Height/Prove JSON fields are omitted which causes contract errors
				{
					Height: 0,
					Prove:  false,
					Path:   "/cosmos.bank.v1beta1.Query/AllBalances",
					Data:   []byte(chain2UserAddrQueryB64), //was originally: eyJhZGRyZXNzIjoiYWRkcmVzcyJ9, //chain2UserAddrQueryB64
				},
			},
		},
	}

	// query2 := executeQuery{
	// 	Query: msgQuery{
	// 		Channel: channels[0].ChannelID,
	// 		Requests: []abcitypes.RequestQuery{
	// 			{
	// 				Path: "/cosmos.bank.v1beta1.Query/AllBalances",
	// 				Data: []byte("eyJhZGRyZXNzIjoiYWRkcmVzcyJ9"), //was originally: eyJhZGRyZXNzIjoiYWRkcmVzcyJ9, //gaiaAddressQueryB64
	// 			},
	// 		},
	// 	},
	// }

	b, err := json.Marshal(query)
	require.NoError(t, err)
	msg := string(b)
	logger.Info("Executing msg ->", zap.String("msg", msg))

	//Query the Juno contract, requesting the balance of the Gaia user's wallet via IBC.
	hash, err := chain1CChain.ExecuteContractWithResult(ctx, chain1User.KeyName, contractAddr, msg)

	require.NoError(t, err)

	// Check the results from the interchain query above.
	cmd = []string{chain1.Config().Bin, "query", "tx", hash,
		"--node", chain1.GetRPCAddress(),
		"--home", chain1.HomeDir(),
		"--chain-id", chain1.Config().ChainID,
		"--output", "json",
	}
	stdout, _, err = chain1.Exec(ctx, cmd, nil)
	require.NoError(t, err)

	logger.Info("TX Query result: ......")
	logger.Info(string(stdout))
	logger.Info("End TX query result: ......")

	//b2, err := json.Marshal(query2)
	//msg2 := string(b2)
	//Query the Juno contract, requesting the balance of the Gaia user's wallet via IBC.
	// err = chain1CChain.ExecuteContract(ctx, chain1User.KeyName, contractAddr, msg2)
	// require.NoError(t, err)

	// Wait a few blocks for query to be sent to counterparty.
	err = test.WaitForBlocks(ctx, 10, chain1, chain2)
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

	logger.Info("WASM Query result: ......")
	logger.Info(string(stdout))
	logger.Info("End Query result: ......")

	results := &contractStateResp{}
	err = json.Unmarshal(stdout, results)
	require.NoError(t, err)
	for _, kv := range results.Models {
		keyBytes, _ := hex.DecodeString(kv.Key)
		valueBytes, err := base64.StdEncoding.DecodeString(kv.Value)
		require.NoError(t, err)
		logger.Info("Query result", zap.String("key", string(keyBytes)), zap.String("value", string(valueBytes)))
	}
	// logger.Info("contract state ->", zap.Any("results", results))
	require.NotEmpty(t, results.Models)
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

// type icqResults struct {
// 	Request struct {
// 		Type       string `json:"@type"`
// 		Address    string `json:"address"`
// 		Pagination struct {
// 			Key        interface{} `json:"key"`
// 			Offset     string      `json:"offset"`
// 			Limit      string      `json:"limit"`
// 			CountTotal bool        `json:"count_total"`
// 			Reverse    bool        `json:"reverse"`
// 		} `json:"pagination"`
// 	} `json:"request"`
// 	Response struct {
// 		Type     string `json:"@type"`
// 		Balances []struct {
// 			Amount string `json:"amount"`
// 			Denom  string `json:"denom"`
// 		} `json:"balances"`
// 		Pagination struct {
// 			NextKey interface{} `json:"next_key"`
// 			Total   string      `json:"total"`
// 		} `json:"pagination"`
// 	} `json:"response"`
// }

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

// func modifyGenesisAllowICQQueries(allowQueries []string) func(ibc.ChainConfig, []byte) ([]byte, error) {
// 	return func(chainConfig ibc.ChainConfig, genbz []byte) ([]byte, error) {
// 		g := make(map[string]interface{})
// 		if err := json.Unmarshal(genbz, &g); err != nil {
// 			return nil, fmt.Errorf("failed to unmarshal genesis file: %w", err)
// 		}
// 		if err := dyno.Set(g, allowQueries, "app_state", "interchainquery", "params", "allow_queries"); err != nil {
// 			return nil, fmt.Errorf("failed to set allowed interchain queries in genesis json: %w", err)
// 		}
// 		out, err := json.Marshal(g)
// 		if err != nil {
// 			return nil, fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
// 		}
// 		return out, nil
// 	}
// }
