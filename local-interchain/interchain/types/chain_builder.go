package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"gopkg.in/yaml.v3"
)

// NewChainBuilder creates a new Chain.
func NewChainBuilder(name, chainID, binary, denom, bech32 string) *Chain {
	coinType := 118

	return &Chain{
		Name:    name,
		ChainID: chainID,
		Binary:  binary,
		Denom:   denom,

		TrustingPeriod: "336h",
		Bech32Prefix:   bech32,
		ChainType:      "cosmos",
		CoinType:       coinType,
		GasAdjustment:  2.0,
		NumberVals:     1,
		NumberNode:     0,
		GasPrices:      fmt.Sprintf("0.0%s", denom),
		Debugging:      false,
		Genesis: Genesis{
			Accounts:        []GenesisAccount{},
			Modify:          []cosmos.GenesisKV{},
			StartupCommands: []string{},
		},
	}
}

func (c *Chain) SetDenom(denom string) *Chain {
	c.Denom = denom
	return c
}

func (c *Chain) SetDockerImage(dockerImage ibc.DockerImage) *Chain {
	c.DockerImage = dockerImage
	return c
}

func (c *Chain) SetHostPortOverride(hostPortOverride map[string]string) *Chain {
	c.HostPortOverride = hostPortOverride
	return c
}

func (c *Chain) SetGasPrices(gasPrices string) *Chain {
	c.GasPrices = gasPrices
	return c
}

func (c *Chain) SetGasAdjustment(gasAdjustment float64) *Chain {
	c.GasAdjustment = gasAdjustment
	return c
}

func (c *Chain) SetValidators(numberVals int) *Chain {
	c.NumberVals = numberVals
	return c
}

func (c *Chain) SetNodes(numberNode int) *Chain {
	c.NumberNode = numberNode
	return c
}

func (c *Chain) SetChainType(chainType string) *Chain {
	c.ChainType = chainType
	return c
}

func (c *Chain) SetDebugging(debugging bool) *Chain {
	c.Debugging = debugging
	return c
}

func (c *Chain) SetBlockTime(blockTime string) *Chain {
	c.BlockTime = blockTime
	return c
}

func (c *Chain) SetTrustingPeriod(trustingPeriod string) *Chain {
	c.TrustingPeriod = trustingPeriod
	return c
}

func (c *Chain) SetICSConsumerLink(icsConsumerLink string) *Chain {
	c.ICSConsumerLink = icsConsumerLink
	return c
}

// SetIBCPaths hardcodes the set IBC paths array for the chain.
func (c *Chain) SetIBCPaths(ibcPaths []string) *Chain {
	c.IBCPaths = ibcPaths
	return c
}

// SetChainsIBCLink appends the new IBC path to both chains
func (c *Chain) SetAppendedIBCPathLink(counterParty *Chain) *Chain {
	ibcPath := fmt.Sprintf("%s_%s", c.ChainID, counterParty.ChainID)
	c.IBCPaths = append(c.IBCPaths, ibcPath)
	counterParty.IBCPaths = append(counterParty.IBCPaths, ibcPath)
	return c
}

func (c *Chain) SetGenesis(genesis Genesis) *Chain {
	c.Genesis = genesis
	return c
}

func (c *Chain) SetConfigFileOverrides(configFileOverrides []ConfigFileOverrides) *Chain {
	c.ConfigFileOverrides = configFileOverrides
	return c
}

func (c *Chain) SetBech32Prefix(bech32Prefix string) *Chain {
	c.Bech32Prefix = bech32Prefix
	c.SetRandomAccounts(5)
	return c
}

func (c *Chain) SetCoinType(num int) *Chain {
	c.CoinType = num
	c.SetRandomAccounts(5)
	return c
}

func (c *Chain) SetRandomAccounts(num int) *Chain {
	c.Genesis.Accounts = GenerateRandomAccounts(num, c.Bech32Prefix, c.CoinType)
	return c
}

func (c *Chain) SaveJSON(filePath string) error {
	config := new(ChainsConfig)
	config.Chains = append(config.Chains, *c)

	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}

	bz, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, bz, 0777)
}

func (c *Chain) SaveYAML(filePath string) error {
	config := new(ChainsConfig)
	config.Chains = append(config.Chains, *c)

	bz, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, bz, 0777)
}

func BaseHostPortOverride() map[string]string {
	return map[string]string{
		"26656": "26656",
		"26657": "26657",
		"1317":  "1317",
		"9090":  "9090",
	}
}
