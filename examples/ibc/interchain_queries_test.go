package ibc_test

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	keyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/icza/dyno"
	"github.com/strangelove-ventures/ibctest/v6"
	cosmosChain "github.com/strangelove-ventures/ibctest/v6/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"github.com/strangelove-ventures/ibctest/v6/internal/dockerutil"
	"github.com/strangelove-ventures/ibctest/v6/relayer"
	"github.com/strangelove-ventures/ibctest/v6/test"
	"github.com/strangelove-ventures/ibctest/v6/testreporter"
	"github.com/stretchr/testify/require"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestInterchainQueries is a test case that performs basic simulations and assertions around the packet implementation
// of interchain queries. See: https://github.com/quasar-finance/interchain-query-demo
func TestInterchainQueriesK(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	ibctest.KeepTempDirOnFailure(true)
	ibctest.KeepDockerVolumesOnFailure(true)
	tmpDir := ibctest.TempDir(t)
	logger := zaptest.NewLogger(t)
	logger.Info("tmpdir", zap.String("dir", tmpDir))

	client, network := ibctest.DockerSetup(t)

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	dockerImage := ibc.DockerImage{
		Repository: "ghcr.io/strangelove-ventures/heighliner/icqd",
		Version:    "latest",
		UidGid:     dockerutil.GetHeighlinerUserString(),
	}

	// Get both chains
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			ChainName: "sender",
			ChainConfig: ibc.ChainConfig{
				Type:           "cosmos",
				Name:           "sender",
				ChainID:        "sender",
				Images:         []ibc.DockerImage{dockerImage},
				Bin:            "icq",
				Bech32Prefix:   "cosmos",
				Denom:          "atom",
				GasPrices:      "0.00atom",
				TrustingPeriod: "300h",
				GasAdjustment:  1.1,
			}},
		{
			ChainName: "receiver",
			ChainConfig: ibc.ChainConfig{
				Type:           "cosmos",
				Name:           "receiver",
				ChainID:        "receiver",
				Images:         []ibc.DockerImage{dockerImage},
				Bin:            "icq",
				Bech32Prefix:   "cosmos",
				Denom:          "atom",
				GasPrices:      "0.00atom",
				TrustingPeriod: "300h",
				GasAdjustment:  1.1,
				ModifyGenesis:  modifyGenesisAllowICQQueries([]string{"/cosmos.bank.v1beta1.Query/AllBalances"}), // Add the whitelisted queries to the host chain
			}},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain1, chain2 := chains[0], chains[1]

	// Get a relayer instance
	r := ibctest.NewBuiltinRelayerFactory(
		ibc.CosmosRly,
		zaptest.NewLogger(t),
		relayer.StartupFlags("-b", "100"),
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
			CreateChannelOpts: ibc.CreateChannelOptions{
				SourcePortName: "interquery",
				DestPortName:   "icqhost",
				Order:          ibc.Unordered,
				Version:        "icq-1",
			},
		})

	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  false,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	// Fund user accounts, so we can query balances and make assertions.
	const userFunds = int64(10_000_000_000)
	users := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain1, chain2)
	chain1User := users[0]
	chain2User := users[1]

	// Wait a few blocks for user accounts to be created on chain.
	err = test.WaitForBlocks(ctx, 5, chain1, chain2)
	require.NoError(t, err)

	// Query for the recently created channel-id.
	channels, err := r.GetChannels(ctx, eRep, chain1.Config().ChainID)
	require.NoError(t, err)

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

	// Query for the balances of an account on the counterparty chain using interchain queries.
	chanID := channels[0].Counterparty.ChannelID
	require.NotEmpty(t, chanID)

	chain1Addr := chain1User.Bech32Address(chain1.Config().Bech32Prefix)
	require.NotEmpty(t, chain1Addr)

	chain2Addr := chain2User.Bech32Address(chain2.Config().Bech32Prefix)
	require.NotEmpty(t, chain2Addr)

	cmd := []string{"icq", "tx", "interquery", "send-query-all-balances", chanID, chain2Addr,
		"--node", chain1.GetRPCAddress(),
		"--home", chain1.HomeDir(),
		"--chain-id", chain1.Config().ChainID,
		"--from", chain1Addr,
		"--keyring-dir", chain1.HomeDir(),
		"--keyring-backend", keyring.BackendTest,
		"-y",
	}
	_, _, err = chain1.Exec(ctx, cmd, nil)
	require.NoError(t, err)

	// Wait a few blocks for query to be sent to counterparty.
	err = test.WaitForBlocks(ctx, 10, chain1)
	require.NoError(t, err)

	// Check the results from the interchain query above.
	cmd = []string{"icq", "query", "interquery", "query-state", strconv.Itoa(1),
		"--node", chain1.GetRPCAddress(),
		"--home", chain1.HomeDir(),
		"--chain-id", chain1.Config().ChainID,
		"--output", "json",
	}
	stdout, _, err := chain1.Exec(ctx, cmd, nil)
	logger.Info("Query result: ......")
	logger.Info(string(stdout))
	logger.Info("End Query result: ......")
	require.NoError(t, err)

	results := &icqResults{}
	err = json.Unmarshal(stdout, results)
	require.NoError(t, err)
	require.NotEmpty(t, results.Request)
	require.NotEmpty(t, results.Response)
}

// CW-20 packet transfer
// TestInterchainQueriesWASM is a test case that performs a round trip query from an ICQ wasm contract <> ICQ module.
func TestInterchainQueriesWASM(t *testing.T) {
	ibctest.KeepTempDirOnFailure(true)
	ibctest.KeepDockerVolumesOnFailure(true)
	tmpDir := ibctest.TempDir(t)
	logger := zaptest.NewLogger(t)
	logger.Info("tmpdir", zap.String("dir", tmpDir))

	if testing.Short() {
		t.Skip()
	}

	//t.Parallel()

	client, network := ibctest.DockerSetup(t)
	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)
	ctx := context.Background()
	contractFilePath := "../contracts/icq.wasm" //Contract that will be initialized on Juno chain

	// polymerDaoDockerImage := ibc.DockerImage{
	// 	Repository: "ghcr.io/strangelove-ventures/heighliner/icqd",
	// 	Version:    "latest",
	// 	UidGid:     dockerutil.GetHeighlinerUserString(),
	// }

	os.Setenv("EXPORT_GENESIS_FILE_PATH", "/home/kyle/projects/Strangelove/ibctest/juno_genesis_icq.json")
	os.Setenv("EXPORT_GENESIS_CHAIN", "sender")

	//Forces relayer to use the specific versions of the chains configured in the file.
	//These match our DockerImage versions below (from this test case)
	os.Setenv("IBCTEST_CONFIGURED_CHAINS", "./icq_wasm_configured_chains.yaml")

	junoDockerImage := ibc.DockerImage{
		Repository: "ghcr.io/strangelove-ventures/heighliner/juno",
		Version:    "v10.1.0",
		UidGid:     dockerutil.GetHeighlinerUserString(),
	}

	gaiaDockerImage := ibc.DockerImage{
		Repository: "ghcr.io/strangelove-ventures/heighliner/gaia",
		Version:    "v7.1.0",
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
	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{
			ChainName:     "sender",
			NumValidators: &minVal,
			ChainConfig: ibc.ChainConfig{
				Type:           "cosmos",
				Name:           "sender",
				ChainID:        "sender",
				Images:         []ibc.DockerImage{junoDockerImage},
				Bin:            "junod",
				Bech32Prefix:   "juno",
				Denom:          "ujuno",
				GasPrices:      "0.00ujuno",
				TrustingPeriod: "300h",
				GasAdjustment:  1.1,
				ModifyGenesis:  modifyGenesisAtPath(genesisAllowICQ, "app_state"),
			}},
		{
			ChainName:     "receiver",
			NumValidators: &minVal,
			ChainConfig: ibc.ChainConfig{
				Type:           "cosmos",
				Name:           "receiver",
				ChainID:        "receiver",
				Images:         []ibc.DockerImage{gaiaDockerImage},
				Bin:            "gaiad",
				Bech32Prefix:   "cosmos",
				Denom:          "uatom",
				GasPrices:      "0.00uatom",
				TrustingPeriod: "300h",
				GasAdjustment:  1.1,
				ModifyGenesis:  modifyGenesisAtPath(genesisAllowICQ, "app_state"),
			}},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	junoChain, gaiaChain := chains[0], chains[1]

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
		AddChain(junoChain).
		AddChain(gaiaChain).
		AddRelayer(r, relayerName).
		AddLink(ibctest.InterchainLink{
			Chain1:  junoChain,
			Chain2:  gaiaChain,
			Relayer: r,
			Path:    pathName,
		})

	logger.Info("ic.Build()")
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,
		//I believe the type registry doesn't have the wasm type needed b/c of the cosmos sdk version...
		// Failed to decode tx     {"chain_id": "sender", "test": "TestInterchainQueriesWASM", "height": 45, "error": "unable to resolve type URL /cosmwasm.wasm.v1.MsgInstantiateContract: tx parse error"
		BlockDatabaseFile: ibctest.DefaultBlockDatabaseFilepath(), //filepath.Join(tmpDir, ".ibctest", "databases", "block.db"), //
		SkipPathCreation:  false,
	}))

	// Wait a few blocks for user accounts to be created on chain
	logger.Info("wait for user accounts")
	err = test.WaitForBlocks(ctx, 5, junoChain, gaiaChain)
	require.NoError(t, err)

	// Fund user accounts so we can query balances
	junoFundAmt := int64(10_000_000_000)
	gaiaFundAmt := int64(99_999_999_999)
	junoUser := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), junoFundAmt, junoChain)[0]
	gaiaUser := ibctest.GetAndFundTestUsers(t, ctx, t.Name(), gaiaFundAmt, gaiaChain)[0]

	err = test.WaitForBlocks(ctx, 5, junoChain, gaiaChain)
	require.NoError(t, err)

	junoUserAddress := junoUser.Bech32Address(junoChain.Config().Bech32Prefix)
	require.NotEmpty(t, junoUserAddress)

	gaiaUserAddr := gaiaUser.Bech32Address(gaiaChain.Config().Bech32Prefix)
	require.NotEmpty(t, gaiaUserAddr)

	gaiaUserBalInitial, err := gaiaChain.GetBalance(ctx, gaiaUserAddr, gaiaChain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, gaiaFundAmt, gaiaUserBalInitial)

	junoUserBalInitial, err := junoChain.GetBalance(ctx, junoUserAddress, junoChain.Config().Denom)
	require.NoError(t, err)
	require.Equal(t, junoFundAmt, junoUserBalInitial)

	logger.Info("instantiating contract")
	initMessage := "{\"default_timeout\": 60}"

	//Instantiate the smart contract on the Juno test chain, facilitating testing of ICQ WASM functionality
	junoCChain := junoChain.(*cosmosChain.CosmosChain)
	contractResp, err := junoCChain.InstantiateContract(ctx, junoUserAddress, ibc.WalletAmount{Amount: 5000000}, contractFilePath, initMessage, true)
	require.NoError(t, err)
	logger.Info("icq contract deployed", zap.String("contractAddr", contractResp.Address))

	err = test.WaitForBlocks(ctx, 5, junoChain, gaiaChain)
	require.NoError(t, err)

	// Create channel between icq wasm contract <> icq module.
	err = r.CreateChannel(ctx, eRep, pathName, ibc.CreateChannelOptions{
		SourcePortName: contractResp.ContractInfo.IBCPortID,
		DestPortName:   "icqhost",
		Order:          ibc.Unordered,
		Version:        "icq-1",
	})
	require.NoError(t, err)

	// Query for the recently created channel-id.
	channels, err := r.GetChannels(ctx, eRep, gaiaChain.Config().ChainID)
	require.NoError(t, err)

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
	err = test.WaitForBlocks(ctx, 5, junoChain, gaiaChain)
	require.NoError(t, err)

	// Query for the balances of an account on the counterparty chain using interchain queries.
	chanID := channels[0].Counterparty.ChannelID
	require.NotEmpty(t, chanID)

	//Get the base64 encoded Gaia user address in the format required by the AllBalances query
	gaiaAddressQuery := fmt.Sprintf(`{"address":"%s"}`, gaiaUserAddr)
	gaiaAddressQueryB64 := base64.StdEncoding.EncodeToString([]byte(gaiaAddressQuery))

	query := executeQuery{
		Query: msgQuery{
			Channel: channels[0].ChannelID,
			Requests: []abcitypes.RequestQuery{
				{
					Path: "/cosmos.bank.v1beta1.Query/AllBalances",
					Data: []byte(gaiaAddressQueryB64), //was originally: eyJhZGRyZXNzIjoiYWRkcmVzcyJ9, //gaiaAddressQueryB64
				},
			},
		},
	}

	query2 := executeQuery{
		Query: msgQuery{
			Channel: channels[0].ChannelID,
			Requests: []abcitypes.RequestQuery{
				{
					Path: "/cosmos.bank.v1beta1.Query/AllBalances",
					Data: []byte("eyJhZGRyZXNzIjoiYWRkcmVzcyJ9"), //was originally: eyJhZGRyZXNzIjoiYWRkcmVzcyJ9, //gaiaAddressQueryB64
				},
			},
		},
	}

	b, err := json.Marshal(query)
	require.NoError(t, err)
	msg := string(b)
	logger.Info("Executing msg ->", zap.String("msg", msg))

	//Query the Juno contract, requesting the balance of the Gaia user's wallet via IBC.
	err = junoCChain.ExecuteContract(ctx, junoUserAddress, contractResp.Address, msg)
	require.NoError(t, err)

	b2, err := json.Marshal(query2)
	require.NoError(t, err)
	msg2 := string(b2)
	//Query the Juno contract, requesting the balance of the Gaia user's wallet via IBC.
	err = junoCChain.ExecuteContract(ctx, junoUserAddress, contractResp.Address, msg2)
	require.NoError(t, err)

	// Wait a few blocks for query to be sent to counterparty.
	err = test.WaitForBlocks(ctx, 10, junoChain)
	require.NoError(t, err)

	// Check the results from the interchain query above.
	cmd := []string{junoChain.Config().Bin, "query", "wasm", "contract-state", "all", contractResp.Address,
		"--node", junoChain.GetRPCAddress(),
		"--home", junoChain.HomeDir(),
		"--chain-id", junoChain.Config().ChainID,
		"--output", "json",
	}
	stdout, _, err := junoChain.Exec(ctx, cmd, nil)
	require.NoError(t, err)

	logger.Info("WASM Query result: ......")
	logger.Info(string(stdout))
	logger.Info("End Query result: ......")

	results := &contractStateResp{}
	err = json.Unmarshal(stdout, results)
	require.NoError(t, err)
	for _, kv := range results.Models {
		keyBytes, _ := hex.DecodeString(kv.Key)
		logger.Info("contract state key ->", zap.Any("name", string(keyBytes)))
		if string(keyBytes) == "query_result_counter" {
			valueBytes, err := base64.StdEncoding.DecodeString(kv.Value)
			require.NoError(t, err)
			logger.Info("contract state value ->", zap.Any("data", string(valueBytes)))
		}
	}
	logger.Info("contract state ->", zap.Any("results", results))
	require.NotEmpty(t, results.Models)
}

type msgQuery struct {
	Channel  string                   `json:"channel"`
	Requests []abcitypes.RequestQuery `json:"requests"`
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

type icqResults struct {
	Request struct {
		Type       string `json:"@type"`
		Address    string `json:"address"`
		Pagination struct {
			Key        interface{} `json:"key"`
			Offset     string      `json:"offset"`
			Limit      string      `json:"limit"`
			CountTotal bool        `json:"count_total"`
			Reverse    bool        `json:"reverse"`
		} `json:"pagination"`
	} `json:"request"`
	Response struct {
		Type     string `json:"@type"`
		Balances []struct {
			Amount string `json:"amount"`
			Denom  string `json:"denom"`
		} `json:"balances"`
		Pagination struct {
			NextKey interface{} `json:"next_key"`
			Total   string      `json:"total"`
		} `json:"pagination"`
	} `json:"response"`
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

func modifyGenesisAllowICQQueries(allowQueries []string) func(ibc.ChainConfig, []byte) ([]byte, error) {
	return func(chainConfig ibc.ChainConfig, genbz []byte) ([]byte, error) {
		g := make(map[string]interface{})
		if err := json.Unmarshal(genbz, &g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal genesis file: %w", err)
		}
		if err := dyno.Set(g, allowQueries, "app_state", "interchainquery", "params", "allow_queries"); err != nil {
			return nil, fmt.Errorf("failed to set allowed interchain queries in genesis json: %w", err)
		}
		out, err := json.Marshal(g)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
		}
		return out, nil
	}
}
