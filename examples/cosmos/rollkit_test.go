package cosmos_test

import (
	"context"
	"fmt"
	"path"
	"testing"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// https://rollkit.dev/tutorials/gm-world
var (
	// TODO: get https://github.com/rollkit/local-celestia-devnet/blob/main/entrypoint.sh into interchaintest
	// 26650:26650 -p 26657:26657 -p 26658:26658 -p 26659:26659 -p 9090:9090 // 26650 runs th celestia-da server
	// DockerImage   = "ghcr.io/strangelove-ventures/heighliner/celestia"
	// DockerVersion = "v1.6.0"
	// DADockerImage   = "ghcr.io/rollkit/celestia-da"
	// DADockerVersion = "v0.12.6"

	// holds both the da and app binaries from heighliner so we can ignore docker mounting magic
	DockerImage   = "reece/full-celestia"
	DockerVersion = "v0.0.1"

	// App & node has a celestia user with home dir /home/celestia
	hardcodedPath = "/var/cosmos-chain/celestia"
	APP_PATH      = ".celestia-app"
	NODE_PATH     = path.Join(hardcodedPath, "bridge")

	// TODO: Set namespace and CELESTIA_CUSTOM env variable on bridge start
	// echo 0000$(openssl rand -hex 8)
	CELESTIA_NAMESPACE = "00007e5327f23637ed07"

	startCmd = []string{
		"celestia-da", "bridge", "init", "--node.store", NODE_PATH,
	}

	otherCmd = []string{
		"celestia-da", "bridge", "start",
		"--node.store", NODE_PATH,
		"--gateway",
		"--core.ip", "test-val-0-TestRollkitCelestia", // n.HostName()
		"--keyring.accname", "validator",
		"--gateway.addr", "0.0.0.0",
		"--rpc.addr", "0.0.0.0",
		"--da.grpc.namespace", CELESTIA_NAMESPACE,
		"--da.grpc.listen", "0.0.0.0:26650",
	}
)

func TestRollkitCelestia(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode.")
	}

	t.Parallel()

	numVals := 1
	numFullNodes := 0
	coinDecimals := int64(6)

	cfg := ibc.ChainConfig{
		Name:                "celestia", // Type: Rollkit / Rollup / Celestia?
		Denom:               "utia",
		Type:                "cosmos",
		GasPrices:           "0utia",
		TrustingPeriod:      "500h",
		HostPortOverride:    map[int]int{26650: 26650, 1317: 1317, 26656: 26656, 26657: 26657, 9090: 9090},
		EncodingConfig:      nil,
		SkipGenTx:           false,
		CoinDecimals:        &coinDecimals,
		AdditionalStartArgs: []string{"--grpc.enable"},
		ChainID:             "test",
		Bin:                 "celestia-appd",
		Images: []ibc.DockerImage{
			{
				Repository: DockerImage,
				Version:    DockerVersion,
				UidGid:     "1025:1025",
			},
		},
		Bech32Prefix: "celestia",
		CoinType:     "118",
		// ModifyGenesis: cosmos.ModifyGenesis([]cosmos.GenesisKV{}),
		GasAdjustment: 1.5,
		ConfigFileOverrides: testutil.Toml{"config/config.toml": testutil.Toml{
			"index_all_keys": true,
			"mode":           "validator", // be sure to only use 1 validator here
			"tx_index": testutil.Toml{
				"indexer": "kv", // TODO: since we execute queries against the validator (unsure if this works on a celestia validator node)
			},
		}},
		SidecarConfigs: []ibc.SidecarConfig{
			{
				ValidatorProcess: true,
				ProcessName:      "celestia-da",
				Image: ibc.DockerImage{
					Repository: DockerImage,
					Version:    DockerVersion,
					UidGid:     "1025:1025",
				},
				HomeDir:  hardcodedPath,
				Ports:    []string{"26650"},
				StartCmd: startCmd,
				Env:      []string{},
				PreStart: false,
			},
		},
	}

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			Name:          cfg.Name,
			ChainName:     cfg.Name,
			Version:       DockerVersion,
			ChainConfig:   cfg,
			NumValidators: &numVals,
			NumFullNodes:  &numFullNodes,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain := chains[0].(*cosmos.CosmosChain)

	ic := interchaintest.NewInterchain().
		AddChain(chain)

	ctx := context.Background()
	client, network := interchaintest.DockerSetup(t)

	require.NoError(t, ic.Build(ctx, nil, interchaintest.InterchainBuildOptions{
		TestName:         t.Name(),
		Client:           client,
		NetworkID:        network,
		SkipPathCreation: true,
	}))
	t.Cleanup(func() {
		_ = ic.Close()
	})

	var userFunds = math.NewInt(10_000_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain)
	chainUser := users[0]
	fmt.Println("chainUser", chainUser)

	fmt.Println("nodePaths", NODE_PATH)

	// TODO: From the guide, after 1 block (20 seconds?)
	n := chain.GetNode()

	// valCommonAddr, err := chain.GetAddress(ctx, "validator")
	// require.NoError(t, err)

	// // print valCommonAddr
	// fmt.Println("valCommonAddr", string(valCommonAddr))

	// // celestiavaloper (could also just query from staking?)
	// valAddr, err := sdk.GetFromBech32(string(valCommonAddr), cfg.Bech32Prefix+"valoper")
	// require.NoError(t, err)

	// fmt.Println("valAddr", string(valAddr))

	fmt.Println("n.ValidatorMnemonic", n.ValidatorMnemonic)

	// qgb register validator
	vals, err := chain.StakingQueryValidators(ctx, stakingtypes.BondStatusBonded)
	require.NoError(t, err)

	val := vals[0].OperatorAddress

	fmt.Println("n.Sidecars", n.Sidecars[0])

	// recover the validator key (from mnemonic) to the da layer home ( https://github.com/rollkit/local-celestia-devnet/blob/main/entrypoint.sh#L57 )
	if err := RecoverKey(ctx, n, "validator", n.ValidatorMnemonic, path.Join(NODE_PATH, "keys")); err != nil {
		t.Fatal(err)
	}

	sc := n.Sidecars[0]
	if err := sc.CreateContainer(ctx); err != nil {
		t.Fatal(err)
	}
	if err := sc.StartContainer(ctx); err != nil {
		t.Fatal(err)
	}

	// waits for 20 seconds (2s blocks * 10)
	testutil.WaitForBlocks(ctx, 10, chain)

	stdout, stderr, err := sc.Exec(ctx, otherCmd, nil)
	fmt.Println("stdout", stdout)
	fmt.Println("stderr", stderr)
	fmt.Println("err", err)

	testutil.WaitForBlocks(ctx, 10, chain)

	// register teh EVM address
	//  # private key: da6ed55cb2894ac2c9c10209c09de8e8b9d109b910338d5bf3d747a7e1fc9eb9
	txHash, err := n.ExecTx(ctx, "validator", "qgb", "register", val, "0x966e6f22781EF6a6A82BBB4DB3df8E225DfD9488", "--fees", "30000utia", "-b", "block", "-y")
	require.NoError(t, err)

	// convert txHash to response (this does not print, but works upon manually querying against the local node)
	res, err := n.TxHashToResponse(ctx, txHash)
	require.NoError(t, err)
	fmt.Printf("res: %+v\n", res)

	// wait for 100 blocks
	testutil.WaitForBlocks(ctx, 100, chain)

}

func RecoverKey(ctx context.Context, tn *cosmos.ChainNode, keyName, mnemonic, homeDir string) error {
	command := []string{
		"sh",
		"-c",
		fmt.Sprintf(`echo %q | %s keys add %s --recover --keyring-backend %s --coin-type %s --home %s --output json`, mnemonic, tn.Chain.Config().Bin, keyName, keyring.BackendTest, tn.Chain.Config().CoinType, homeDir),
	}

	_, _, err := tn.Exec(ctx, command, nil)
	return err
}
