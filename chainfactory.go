package ibctest

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/strangelove-ventures/ibctest/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/chain/penumbra"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/label"
	"go.uber.org/zap"
)

// ChainFactory describes how to get chains for tests.
// This type currently supports a Pair method,
// but it may be expanded to a Triplet method in the future.
type ChainFactory interface {
	// Count reports how many chains this factory will produce from its Chains method.
	Count() int

	// Chains returns a set of chains.
	Chains(testName string) ([]ibc.Chain, error)

	// Name returns a descriptive name of the factory,
	// indicating all of its chains.
	// Depending on how the factory was configured,
	// this may report more than two chains.
	Name() string

	// Labels are reported to allow simple filtering of tests depending on these Chains.
	// While the Name should be fully descriptive,
	// the Labels are intended to be short and fixed.
	Labels() []label.Chain
}

// BuiltinChainFactory implements ChainFactory to return a fixed set of chains.
// Use NewBuiltinChainFactory to create an instance.
type BuiltinChainFactory struct {
	entries []BuiltinChainFactoryEntry
	log     *zap.Logger
}

// BuiltinChainFactoryEntry describes a chain to be returned from an instance of BuiltinChainFactory.
type BuiltinChainFactoryEntry struct {
	Name          string
	Version       string
	ChainID       string
	NumValidators int
	NumFullNodes  int
}

func (e BuiltinChainFactoryEntry) GetChain(log *zap.Logger, testName string) (ibc.Chain, error) {
	chainConfig, exists := builtinChainConfigs[e.Name]
	if !exists {
		availableChains := make([]string, 0, len(builtinChainConfigs))
		for k := range builtinChainConfigs {
			availableChains = append(availableChains, k)
		}
		sort.Strings(availableChains)

		return nil, fmt.Errorf("no chain configuration for %s (available chains are: %s)", e.Name, strings.Join(availableChains, ", "))
	}

	chainConfig.ChainID = e.ChainID

	switch chainConfig.Type {
	case "cosmos":
		chainConfig.Images[0].Version = e.Version
		return cosmos.NewCosmosChain(testName, chainConfig, e.NumValidators, e.NumFullNodes, log), nil
	case "penumbra":
		versionSplit := strings.Split(e.Version, ",")
		if len(versionSplit) != 2 {
			return nil, errors.New("penumbra version should be comma separated penumbra_version,tendermint_version")
		}
		chainConfig.Images[0].Version = versionSplit[1]
		chainConfig.Images[1].Version = versionSplit[0]
		return penumbra.NewPenumbraChain(testName, chainConfig, e.NumValidators, e.NumFullNodes), nil
	default:
		return nil, fmt.Errorf("unexpected error, unknown chain type: %s for chain: %s", chainConfig.Type, e.Name)
	}
}

// builtinChainConfigs is a mapping of valid builtin chain names
// to their predefined ibc.ChainConfig.
var builtinChainConfigs = map[string]ibc.ChainConfig{
	"gaia":     cosmos.NewCosmosHeighlinerChainConfig("gaia", "gaiad", "cosmos", "uatom", "0.01uatom", 1.3, "504h", false),
	"osmosis":  cosmos.NewCosmosHeighlinerChainConfig("osmosis", "osmosisd", "osmo", "uosmo", "0.0uosmo", 1.3, "336h", false),
	"juno":     cosmos.NewCosmosHeighlinerChainConfig("juno", "junod", "juno", "ujuno", "0.0025ujuno", 1.3, "672h", false),
	"agoric":   cosmos.NewCosmosHeighlinerChainConfig("agoric", "agd", "agoric", "urun", "0.01urun", 1.3, "672h", true),
	"icad":     cosmos.NewCosmosHeighlinerChainConfig("icad", "icad", "cosmos", "photon", "0.00photon", 1.2, "504h", false),
	"penumbra": penumbra.NewPenumbraChainConfig(),
}

// NewBuiltinChainFactory returns a BuiltinChainFactory that returns chains defined by entries.
func NewBuiltinChainFactory(entries []BuiltinChainFactoryEntry, logger *zap.Logger) *BuiltinChainFactory {
	return &BuiltinChainFactory{entries: entries, log: logger}
}

func (f *BuiltinChainFactory) Count() int {
	return len(f.entries)
}

func (f *BuiltinChainFactory) Chains(testName string) ([]ibc.Chain, error) {
	chains := make([]ibc.Chain, len(f.entries))
	for i, e := range f.entries {
		chain, err := e.GetChain(f.log, testName)
		if err != nil {
			return nil, err
		}
		chains[i] = chain
	}
	return chains, nil
}

func (f *BuiltinChainFactory) Name() string {
	parts := make([]string, len(f.entries))
	for i, e := range f.entries {
		parts[i] = e.Name + "@" + e.Version
	}
	return strings.Join(parts, "+")
}

func (f *BuiltinChainFactory) Labels() []label.Chain {
	labels := make([]label.Chain, len(f.entries))
	for i, e := range f.entries {
		label := label.Chain(e.Name)
		if !label.IsKnown() {
			// The label must be known (i.e. registered),
			// otherwise filtering from the command line will be broken.
			panic(fmt.Errorf("chain name %s is not a known label", e.Name))
		}
		labels[i] = label
	}
	return labels
}

// CustomChainFactory is a ChainFactory that supports returning chains that are defined by ChainConfig values.
type CustomChainFactory struct {
	entries []CustomChainFactoryEntry
	log     *zap.Logger
}

// CustomChainFactoryEntry describes a chain to be returned by a CustomChainFactory.
type CustomChainFactoryEntry struct {
	Config        ibc.ChainConfig
	NumValidators int
	NumFullNodes  int
}

// NewCustomChainFactory returns a CustomChainFactory that returns chains defined by entries.
func NewCustomChainFactory(entries []CustomChainFactoryEntry, logger *zap.Logger) *CustomChainFactory {
	return &CustomChainFactory{entries: entries, log: logger}
}

func (e CustomChainFactoryEntry) GetChain(testName string, log *zap.Logger) (ibc.Chain, error) {
	switch e.Config.Type {
	case "cosmos":
		return cosmos.NewCosmosChain(testName, e.Config, e.NumValidators, e.NumFullNodes, log), nil
	case "penumbra":
		return penumbra.NewPenumbraChain(testName, e.Config, e.NumValidators, e.NumFullNodes), nil
	default:
		return nil, fmt.Errorf("only (cosmos, penumbra) type chains are currently supported (got %q)", e.Config.Type)
	}
}

func (f *CustomChainFactory) Count() int {
	return len(f.entries)
}

func (f *CustomChainFactory) Chains(testName string) ([]ibc.Chain, error) {
	chains := make([]ibc.Chain, len(f.entries))
	for i, e := range f.entries {
		chain, err := e.GetChain(testName, f.log)
		if err != nil {
			return nil, err
		}
		chains[i] = chain
	}
	return chains, nil
}

func (f *CustomChainFactory) Name() string {
	parts := make([]string, len(f.entries))
	for i, e := range f.entries {
		parts[i] = e.Config.Name + "@" + e.Config.Images[0].Version
	}
	return strings.Join(parts, "+")
}

func (f *CustomChainFactory) Labels() []label.Chain {
	labels := make([]label.Chain, len(f.entries))
	for i, e := range f.entries {
		// Although the builtin chains panic if a label is unknown,
		// we don't apply that check on custom chain factories.
		labels[i] = label.Chain(e.Config.Name)
	}
	return labels
}
