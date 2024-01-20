package cosmos_test

import (
	"context"
	"fmt"
	"path"
	"testing"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// https://rollkit.dev/tutorials/gm-world
const (
	// TODO: get https://github.com/rollkit/local-celestia-devnet/blob/main/entrypoint.sh into interchaintest
	// 26650:26650 -p 26657:26657 -p 26658:26658 -p 26659:26659 -p 9090:9090 // 26650 runs th celestia-da server
	// DockerImage   = "ghcr.io/celestiaorg/celestia-app"
	DockerImage   = "ghcr.io/strangelove-ventures/heighliner/celestia"
	DockerVersion = "v1.6.0"

	DADockerImage   = "ghcr.io/rollkit/celestia-da"
	DADockerVersion = "v0.12.6"

	// App & node has a celestia user with home dir /home/celestia
	APP_PATH  = ".celestia-app"
	NODE_PATH = "bridge"
)

func TestRollkitCelestia(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	t.Parallel()

	numVals := 1
	numFullNodes := 0
	coinDecimals := int64(6)

	// https://github.com/rollkit/local-celestia-devnet/blob/main/entrypoint.sh

	// configFileOverrides := make(map[string]any)
	// configFileOverrides["config/config.toml"] = make(testutil.Toml)
	// configFileOverrides["config/client.toml"] = make(testutil.Toml)

	cfg := ibc.ChainConfig{
		Name:           "celestia", // Type: Rollkit / Rollup / Celestia?
		Denom:          "utia",
		Type:           "cosmos",
		GasPrices:      "0utia",
		TrustingPeriod: "500h",
		// HostPortOverride: ,
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
		}},
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

	// var userFunds = math.NewInt(10_000_000_000)
	// users := interchaintest.GetAndFundTestUsers(t, ctx, t.Name(), userFunds, chain)
	// chainUser := users[0]
	// fmt.Println("chainUser", chainUser)

	nodePath := path.Join(chain.GetNode().HomeDir(), NODE_PATH)
	fmt.Println("nodePath", nodePath)

	// n := chain.GetNode()
	// n.Exec()

	// TODO: get the genesis file from the node
	// Start the celestia da bridge
}
