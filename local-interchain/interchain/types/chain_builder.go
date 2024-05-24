package types

import (
	"fmt"

	"github.com/strangelove-ventures/interchaintest/v8/chain/cosmos"
)

// NewChainBuilder creates a new Chain.
func NewChainBuilder(name, chainID, binary, denom string) *Chain {
	return &Chain{
		Name:    name,
		ChainID: chainID,
		Denom:   denom,
		Binary:  binary,

		TrustingPeriod: "336h",
		ChainType:      "cosmos",
		CoinType:       118,
		GasAdjustment:  2.0,
		NumberVals:     1,
		NumberNode:     0,
		GasPrices:      fmt.Sprintf("0.025%s", denom),
		Debugging:      false,
		Genesis: Genesis{
			Accounts:        GenerateRandomAccounts("bech32", 10),
			Modify:          []cosmos.GenesisKV{},
			StartupCommands: []string{},
		},
	}
}

func (c *Chain) WithDenom(denom string) *Chain {
	c.Denom = denom
	return c
}

func (c *Chain) WithDockerImage(dockerImage DockerImage) *Chain {
	c.DockerImage = dockerImage
	return c
}

func (c *Chain) WithGasPrices(gasPrices string) *Chain {
	c.GasPrices = gasPrices
	return c
}

func (c *Chain) WithGasAdjustment(gasAdjustment float64) *Chain {
	c.GasAdjustment = gasAdjustment
	return c
}

func (c *Chain) WithValidators(numberVals int) *Chain {
	c.NumberVals = numberVals
	return c
}

func (c *Chain) WithNodes(numberNode int) *Chain {
	c.NumberNode = numberNode
	return c
}

func (c *Chain) WithChainType(chainType string) *Chain {
	c.ChainType = chainType
	return c
}

func (c *Chain) WithBech32Prefix(bech32Prefix string) *Chain {
	c.Bech32Prefix = bech32Prefix
	return c
}

func (c *Chain) WithDebugging(debugging bool) *Chain {
	c.Debugging = debugging
	return c
}

func (c *Chain) WithBlockTime(blockTime string) *Chain {
	c.BlockTime = blockTime
	return c
}

// with TrustingPeriod
func (c *Chain) WithTrustingPeriod(trustingPeriod string) *Chain {
	c.TrustingPeriod = trustingPeriod
	return c
}

func (c *Chain) WithHostPortOverride(hostPortOverride map[string]string) *Chain {
	c.HostPortOverride = hostPortOverride
	return c
}

func (c *Chain) WithICSConsumerLink(icsConsumerLink string) *Chain {
	c.ICSConsumerLink = icsConsumerLink
	return c
}

func (c *Chain) WithIBCPaths(ibcPaths []string) *Chain {
	c.IBCPaths = ibcPaths
	return c
}

func (c *Chain) WithGenesis(genesis Genesis) *Chain {
	c.Genesis = genesis
	return c
}

func (c *Chain) WithConfigFileOverrides(configFileOverrides []ConfigFileOverrides) *Chain {
	c.ConfigFileOverrides = configFileOverrides
	return c
}

func (c *Chain) WithEVMLoadStatePath(evmLoadStatePath string) *Chain {
	// TODO: check only if ethereum as chain type is used? else panic
	c.EVMLoadStatePath = evmLoadStatePath
	return c
}

// Build returns the final Chain.
func (c *Chain) Build() *Chain {
	return c
}
