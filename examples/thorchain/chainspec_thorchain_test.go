package thorchain_test

import (
	"fmt"
	"strings"

	"cosmossdk.io/math"
	"github.com/strangelove-ventures/interchaintest/v9"
	"github.com/strangelove-ventures/interchaintest/v9/chain/thorchain"
	"github.com/strangelove-ventures/interchaintest/v9/chain/thorchain/common"
	"github.com/strangelove-ventures/interchaintest/v9/ibc"
)

var (
	Denom               = "rune"
	Binary              = "thornode"
	Bech32              = "tthor"
	CoinScale           = math.NewInt(100_000_000)
	StaticGas           = math.NewInt(2_000_000)
	InitialFaucetAmount = math.NewInt(100_000_000).Mul(CoinScale)
)

type ChainContract struct {
	Chain  string `json:"chain"`
	Router string `json:"router"`
}

func ThorchainDefaultChainSpec(testName string, numVals int, numFn int, ethRouter string, bscRouter string, thornodeEnvOverrides, bifrostEnvOverrides map[string]string) *interchaintest.ChainSpec {
	chainID := "thorchain"
	name := common.THORChain.String() // Must use this name for test
	chainImage := ibc.NewDockerImage("thorchain", "local", "1025:1025")
	genesisKVMods := []thorchain.GenesisKV{
		thorchain.NewGenesisKV("app_state.bank.params.default_send_enabled", true), // disable bank module transfers
		thorchain.NewGenesisKV("app_state.thorchain.reserve", "22000000000000000"), // mint to reserve for mocknet (220M)
		thorchain.NewGenesisKV("app_state.thorchain.chain_contracts", []ChainContract{
			{
				Chain:  "ETH",
				Router: ethRouter,
			},
			{
				Chain:  "BSC",
				Router: bscRouter,
			},
		}),
	}

	thornodeEnv := thornodeDefaults
	for k, v := range thornodeEnvOverrides {
		found := false
		modifiedEnv := fmt.Sprintf("%s=%s", k, v)
		for i, env := range thornodeEnv {
			if strings.Contains(env, fmt.Sprintf("%s=", k)) {
				found = true
				thornodeEnv[i] = modifiedEnv
			}
		}
		if !found {
			thornodeEnv = append(thornodeEnv, modifiedEnv)
		}
	}

	bifrostEnv := bifrostDefaults
	for k, v := range bifrostEnvOverrides {
		found := false
		modifiedEnv := fmt.Sprintf("%s=%s", k, v)
		for i, env := range bifrostEnv {
			if strings.Contains(env, fmt.Sprintf("%s=", k)) {
				found = true
				bifrostEnv[i] = modifiedEnv
			}
		}
		if !found {
			bifrostEnv = append(bifrostEnv, modifiedEnv)
		}
	}

	defaultChainConfig := ibc.ChainConfig{
		Images: []ibc.DockerImage{
			chainImage,
		},
		GasAdjustment:  1.5,
		Type:           "thorchain",
		Name:           name,
		ChainID:        chainID,
		Bin:            Binary,
		Bech32Prefix:   Bech32,
		Denom:          Denom,
		CoinType:       "931",
		GasPrices:      "0" + Denom,
		TrustingPeriod: "336h",
		Env:            thornodeEnv,
		SidecarConfigs: []ibc.SidecarConfig{
			{
				ProcessName: "bifrost",
				Image:       chainImage,
				HomeDir:     "/var/data/bifrost",
				Ports:       []string{"5040", "6040", "9000"},
				// StartCmd: []string{"bifrost", "-p"},
				StartCmd:         []string{"bifrost", "-p", "-l", "debug"},
				Env:              bifrostEnv,
				PreStart:         false,
				ValidatorProcess: true,
			},
		},
		ModifyGenesis:    thorchain.ModifyGenesis(genesisKVMods),
		HostPortOverride: map[int]int{1317: 1317},
	}

	return &interchaintest.ChainSpec{
		Name:          name,
		ChainName:     name,
		Version:       chainImage.Version,
		ChainConfig:   defaultChainConfig,
		NumValidators: &numVals,
		NumFullNodes:  &numFn,
	}
}

var (
	allNodeDefaults = []string{
		"NET=mocknet",
		"CHAIN_ID=thorchain",
		"SIGNER_NAME=thorchain",  // Must be thorchain, hardcoded in thorchain module
		"SIGNER_PASSWD=password", // Must use this password, used to generate ed25519
	}

	thornodeDefaults = append(allNodeDefaults, []string{
		"THOR_BLOCK_TIME=2s", // link to config override
		"THOR_API_LIMIT_COUNT=500",
		"THOR_API_LIMIT_DURATION=1s",
		"HARDFORK_BLOCK_HEIGHT=",
		"NEW_GENESIS_TIME=",
		"CHURN_MIGRATION_ROUNDS=2",
		"FUND_MIGRATION_INTERVAL=10",
	}...)

	bifrostDefaults = append(thornodeDefaults, []string{
		"BIFROST_CHAINS_AVAX_DISABLED=true",
		"BIFROST_CHAINS_BCH_DISABLED=true",
		"BIFROST_CHAINS_BNB_DISABLED=true",
		"BIFROST_CHAINS_BSC_DISABLED=true",
		"BIFROST_CHAINS_BTC_DISABLED=true",
		"BIFROST_CHAINS_DOGE_DISABLED=true",
		"BIFROST_CHAINS_ETH_DISABLED=true",
		"BIFROST_CHAINS_GAIA_DISABLED=true",
		"BIFROST_CHAINS_LTC_DISABLED=true",

		"BLOCK_SCANNER_BACKOFF=5s",
		"BIFROST_METRICS_PPROF_ENABLED=false",   // todo change to true
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
		"BIFROST_CHAINS_GAIA_BLOCK_SCANNER_OBSERVATION_FLEXIBILITY_BLOCKS=25",
		"BIFROST_CHAINS_LTC_BLOCK_SCANNER_OBSERVATION_FLEXIBILITY_BLOCKS=5",

		// maintain historical gas behavior for hard-coded smoke test values
		"BIFROST_CHAINS_ETH_BLOCK_SCANNER_MAX_GAS_LIMIT=80000",
		"BIFROST_CHAINS_AVAX_BLOCK_SCANNER_MAX_GAS_LIMIT=80000",
		"BIFROST_CHAINS_BSC_BLOCK_SCANNER_MAX_GAS_LIMIT=80000",

		// set fixed gas rate for evm chains
		"BIFROST_CHAINS_ETH_BLOCK_SCANNER_FIXED_GAS_RATE=30000000000",      // 30 gwei
		"BIFROST_CHAINS_AVAX_BLOCK_SCANNER_FIXED_GAS_RATE=100_000_000_000", // 100 navax
		"BIFROST_CHAINS_BSC_BLOCK_SCANNER_FIXED_GAS_RATE=50_000_000_000",   // 50 gwei

		// override bifrost whitelist tokens
		"BIFROST_CHAINS_AVAX_BLOCK_SCANNER_WHITELIST_TOKENS=0x52C84043CD9c865236f11d9Fc9F56aa003c1f922,0xB97EF9Ef8734C71904D8002F8b6Bc66Dd9c48a6E,0x17aB05351fC94a1a67Bf3f56DdbB941aE6c63E25",
		"BIFROST_CHAINS_BSC_BLOCK_SCANNER_WHITELIST_TOKENS=0x52C84043CD9c865236f11d9Fc9F56aa003c1f922,0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d",
	}...)
)
