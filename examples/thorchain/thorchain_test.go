package thorchain_test

import (
	"context"
	"testing"
	
	"cosmossdk.io/math"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	numValidators = 4
	numFullNodes  = 0

	Denom  = "rune"
	Binary = "thornode"
	Bech32 = "tthor"
)

func TestThorchain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	t.Parallel()

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), ThorchainSpec(t.Name(), numValidators, numFullNodes))

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	chain := chains[0].(*thorchain.Thorchain)

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

	// Temporary before bank transfers are disabled

	fundAmount := math.NewInt(100_000_000)
	users := interchaintest.GetAndFundTestUsers(t, ctx, "default", fundAmount, chain)
	thorchainUser := users[0]
	err = testutil.WaitForBlocks(ctx, 2, chain)
	require.NoError(t, err, "thorchain failed to make blocks")

	// Check balances are correct
	thorchainUserAmount, err := chain.GetBalance(ctx, thorchainUser.FormattedAddress(), chain.Config().Denom)
	require.NoError(t, err)
	require.True(t, thorchainUserAmount.Equal(fundAmount), "Initial thorchain user amount not expected")
}


func ThorchainSpec(testName string, numVals int, numFn int) []*interchaintest.ChainSpec {
	chainID := "thorchain"
	name := "Thorchain"
	chainImage := ibc.NewDockerImage("thorchain", "local", "1025:1025")

	defaultChainConfig := ibc.ChainConfig{
		Images: []ibc.DockerImage{
			chainImage,
		},
		GasAdjustment: 1.5,
		Type:           "thorchain",
		Name:           name,
		ChainID:        chainID,
		Bin:            Binary,
		Bech32Prefix:   Bech32,
		Denom:          Denom,
		CoinType:       "931",
		GasPrices:      "0" + Denom,
		TrustingPeriod: "336h",
		Env:            thornodeDefaults,
		SidecarConfigs: []ibc.SidecarConfig{
			{
				ProcessName: "bifrost",
				Image: chainImage,
				HomeDir: "/var/data/bifrost",
				Ports: []string{"5040", "6040", "9000"},
				StartCmd: []string{"bifrost", "-p"},
				Env: bifrostDefaults,
				PreStart: false,
				ValidatorProcess: true,
			},
		},
	}

	return []*interchaintest.ChainSpec{
		{
			Name:          name,
			ChainName:     name,
			Version:       chainImage.Version,
			ChainConfig:   defaultChainConfig,
			NumValidators: &numVals,
			NumFullNodes:  &numFn,
		},
	}
}

var (
	allNodeDefaults = []string{
		"NET=mocknet", 
		"CHAIN_ID=thorchain",
		"SIGNER_NAME=thorchain",
		"SIGNER_PASSWD=password", // Must use this password, used to generate ed25519
	}

	thornodeDefaults = append(allNodeDefaults, []string{
		"THOR_BLOCK_TIME=2s", // link to config override
		"THOR_API_LIMIT_COUNT=100",
		"THOR_API_LIMIT_DURATION=1s",
		"HARDFORK_BLOCK_HEIGHT=",
		"NEW_GENESIS_TIME=",
		"CHURN_MIGRATION_ROUNDS=2",
		"FUND_MIGRATION_INTERVAL=10",
	
		// set at runtime
		//NODES: 1
   	 	//SEED: thornode (don't need)
  		//SIGNER_SEED_PHRASE: "dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog fossil"
   		//AVAX_HOST: ${AVAX_HOST:-http://avalanche:9650/ext/bc/C/rpc} (is this needed for thornode?)
   		//ETH_HOST: ${ETH_HOST:-http://ethereum:8545 (is this needed for thornode?)}
   		//BSC_HOST: ${BSC_HOST:-http://binance-smart:8545 (is this needed for thornode?)}

		// 
	}...)

	bifrostDefaults = append(thornodeDefaults, []string{
		// set at runtime
		//SIGNER_SEED_PHRASE: "dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog fossil"
		//CHAIN_API: thornode:1317
		//CHAIN_RPC: thornode:26657
		//PEER: ${PEER:-}

		// set at runtime (when enabled)
		//BINANCE_HOST: ${BINANCE_HOST:-http://binance:26660}
		//BTC_HOST: ${BTC_HOST:-bitcoin:18443}
		//DOGE_HOST: ${DOGE_HOST:-dogecoin:18332}
		//BCH_HOST: ${BCH_HOST:-bitcoin-cash:28443}
		//LTC_HOST: ${LTC_HOST:-litecoin:38443}
		//ETH_HOST: ${ETH_HOST:-http://ethereum:8545}
		//AVAX_HOST: ${AVAX_HOST:-http://avalanche:9650/ext/bc/C/rpc}
		//GAIA_HOST: ${GAIA_HOST:-http://gaia:26657}
		//GAIA_GRPC_HOST: ${GAIA_GRPC_HOST:-gaia:9090}
		
		// disable chains until brought in
		"BIFROST_CHAINS_AVAX_DISABLED=true",
		"BIFROST_CHAINS_BCH_DISABLED=true",
		"BIFROST_CHAINS_BNB_DISABLED=true",
		"BIFROST_CHAINS_BSC_DISABLED=true",
		"BIFROST_CHAINS_BTC_DISABLED=true",
		"BIFROST_CHAINS_DOGE_DISABLED=true",
		"BIFROST_CHAINS_ETH_DISABLED=true",
		"BIFROST_CHAINS_GAIA_DISABLED=true",
		"BIFROST_CHAINS_LTC_DISABLED=true",
		
		// block above should take care of these
		//"GAIA_DISABLED=true",
		//"DOGE_DISABLED=true",
		//"LTC_DISABLED=true",
		//"AVAX_DISABLED=true",

		"BLOCK_SCANNER_BACKOFF=5s",
		"BIFROST_METRICS_PPROF_ENABLED=false", // todo change to true
		"BIFROST_SIGNER_BACKUP_KEYSHARES=false", // todo change to true
		"BIFROST_SIGNER_AUTO_OBSERVE=false",
		"BIFROST_SIGNER_KEYGEN_TIMEOUT=30s",
		"BIFROST_SIGNER_KEYSIGN_TIMEOUT=30s",
		"BIFROST_CHAINS_AVAX_BLOCK_SCANNER_OBSERVATION_FLEXIBILITY_BLOCKS=5",
		"BIFROST_CHAINS_BCH_BLOCK_SCANNER_OBSERVATION_FLEXIBILITY_BLOCKS=5",
		"BIFROST_CHAINS_BNB_BLOCK_SCANNER_OBSERVATION_FLEXIBILITY_BLOCKS=5",
		"BIFROST_CHAINS_BTC_BLOCK_SCANNER_OBSERVATION_FLEXIBILITY_BLOCKS=5",
		"BIFROST_CHAINS_DOGE_BLOCK_SCANNER_OBSERVATION_FLEXIBILITY_BLOCKS=5",
		"BIFROST_CHAINS_ETH_BLOCK_SCANNER_OBSERVATION_FLEXIBILITY_BLOCKS=5",
		"BIFROST_CHAINS_GAIA_BLOCK_SCANNER_OBSERVATION_FLEXIBILITY_BLOCKS=5",
		"BIFROST_CHAINS_LTC_BLOCK_SCANNER_OBSERVATION_FLEXIBILITY_BLOCKS=5",

		// maintain historical gas behavior for hard-coded smoke test values
		"BIFROST_CHAINS_ETH_BLOCK_SCANNER_MAX_GAS_LIMIT=80000",
		"BIFROST_CHAINS_AVAX_BLOCK_SCANNER_MAX_GAS_LIMIT=80000",
		"BIFROST_CHAINS_BSC_BLOCK_SCANNER_MAX_GAS_LIMIT=80000",

		// enable bsc
		//"BIFROST_CHAINS_BSC_DISABLED=false", // todo change to false once brought in
		//"BIFROST_CHAINS_BSC_RPC_HOST: ${BSC_HOST:-http://binance-smart:8545}
		//"BIFROST_CHAINS_BSC_BLOCK_SCANNER_RPC_HOST: ${BSC_HOST:-http://binance-smart:8545}
  
		// set fixed gas rate for evm chains
		"BIFROST_CHAINS_ETH_BLOCK_SCANNER_FIXED_GAS_RATE=20_000_000_000", // 20 gwei
		"BIFROST_CHAINS_AVAX_BLOCK_SCANNER_FIXED_GAS_RATE=100_000_000_000", // 100 navax
		"BIFROST_CHAINS_BSC_BLOCK_SCANNER_FIXED_GAS_RATE=50_000_000_000", // 50 gwei
  
		// override bifrost whitelist tokens
		"BIFROST_CHAINS_AVAX_BLOCK_SCANNER_WHITELIST_TOKENS=0x52C84043CD9c865236f11d9Fc9F56aa003c1f922,0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E,0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25",
		"BIFROST_CHAINS_BSC_BLOCK_SCANNER_WHITELIST_TOKENS=0x52C84043CD9c865236f11d9Fc9F56aa003c1f922,0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d",
  

	}...)
)