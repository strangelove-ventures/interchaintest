package ibctest

import (
	"fmt"

	"github.com/strangelove-ventures/ibc-test-framework/chain/cosmos"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
)

// ChainFactory describes how to get chains for tests.
// This type currently supports a Pair method,
// but it may be expanded to a Triplet method in the future.
type ChainFactory interface {
	// Pair returns two chains for IBC.
	Pair(testName string) (ibc.Chain, ibc.Chain, error)
}

// BuiltinChainFactory implements ChainFactory to return a fixed set of chains.
// Use NewBuiltinChainFactory to create an instance.
type BuiltinChainFactory struct {
	entries []BuiltinChainFactoryEntry
}

// BuiltinChainFactoryEntry describes a chain to be returned from an instance of BuiltinChainFactory.
type BuiltinChainFactoryEntry struct {
	Name          string
	Version       string
	ChainID       string
	NumValidators int
	NumFullNodes  int
}

// NewBuiltinChainFactory returns a BuiltinChainFactory that returns chains defined by entries.
//
// Currently, NewBuiltinChainFactory will panic if entries is not of length 2.
// In the future, this method may allow or require entries to have length 3.
func NewBuiltinChainFactory(entries []BuiltinChainFactoryEntry) *BuiltinChainFactory {
	if len(entries) != 2 {
		panic(fmt.Errorf("NewBuiltinChainFactory: received %d entries but required 2", len(entries)))
	}

	return &BuiltinChainFactory{entries: entries}
}

// Pair returns two chains to be used in tests that expect exactly two chains.
func (f *BuiltinChainFactory) Pair(testName string) (ibc.Chain, ibc.Chain, error) {
	e := f.entries[0]
	src, err := GetChain(testName, e.Name, e.Version, e.ChainID, e.NumValidators, e.NumFullNodes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get chain with name %q: %w", e.Name, err)
	}

	e = f.entries[1]
	dst, err := GetChain(testName, e.Name, e.Version, e.ChainID, e.NumValidators, e.NumFullNodes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get chain with name %q: %w", e.Name, err)
	}

	return src, dst, nil
}

// CustomChainFactory is a ChainFactory that supports returning chains that are defined by ChainConfig values.
type CustomChainFactory struct {
	entries []CustomChainFactoryEntry
}

// CustomChainFactoryEntry describes a chain to be returned by a CustomChainFactory.
type CustomChainFactoryEntry struct {
	Type          string
	Config        ibc.ChainConfig
	NumValidators int
	NumFullNodes  int
}

// NewCustomChainFactory returns a CustomChainFactory that returns chains defined by entries.
//
// Currently, NewCustomChainFactory will panic if entries is not of length 2.
// In the future, this method may allow or require entries to have length 3.
func NewCustomChainFactory(entries []CustomChainFactoryEntry) *CustomChainFactory {
	if len(entries) != 2 {
		panic(fmt.Errorf("NewCustomChainFactory: received %d entries but required 2", len(entries)))
	}

	return &CustomChainFactory{entries: entries}
}

// Pair returns two chains to be used in tests that expect exactly two chains.
func (f *CustomChainFactory) Pair(testName string) (ibc.Chain, ibc.Chain, error) {
	e := f.entries[0]
	if e.Type != "cosmos" {
		return nil, nil, fmt.Errorf("only cosmos type chains are currently supported (got %q)", e.Type)
	}
	src := cosmos.NewCosmosChain(testName, e.Config, e.NumValidators, e.NumFullNodes)

	e = f.entries[1]
	if e.Type != "cosmos" {
		return nil, nil, fmt.Errorf("only cosmos type chains are currently supported (got %q)", e.Type)
	}
	dst := cosmos.NewCosmosChain(testName, e.Config, e.NumValidators, e.NumFullNodes)

	return src, dst, nil
}
