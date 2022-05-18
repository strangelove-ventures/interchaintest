package label

import (
	"fmt"
)

// Test is a label associated with an individual test.
// All test labels are known at compile time.
type Test string

const (
	Timeout          Test = "timeout"
	HeightTimeout    Test = "height_timeout"
	TimestampTimeout Test = "timestamp_timeout"
)

var knownTestLabels = map[Test]struct{}{
	Timeout:          {},
	HeightTimeout:    {},
	TimestampTimeout: {},
}

func (l Test) IsKnown() bool {
	_, exists := knownTestLabels[l]
	return exists
}

// Relayer is a label associated with a relayer during tests.
// Relayer values must be registered through RegisterRelayerLabel, typically inside init functions.
type Relayer string

const (
	Rly    Relayer = "rly"
	Hermes Relayer = "hermes"
)

var knownRelayerLabels = map[Relayer]struct{}{
	Rly:    {},
	Hermes: {},
}

func (l Relayer) IsKnown() bool {
	_, exists := knownRelayerLabels[l]
	return exists
}

// RegisterRelayerLabel is available for external packages that may import ibctest,
// to register any external relayer implementations they may provide.
func RegisterRelayerLabel(l Relayer) {
	if _, exists := knownRelayerLabels[l]; exists {
		panic(fmt.Errorf("relayer label %q already exists and must not be double registered", l))
	}

	knownRelayerLabels[l] = struct{}{}
}

// Chain is a label associated with a chain during tests.
// Chain values must be registered through RegisterChainLabel, typically inside init functions.
type Chain string

func (l Chain) IsKnown() bool {
	_, exists := knownChainLabels[l]
	return exists
}

const (
	// Cosmos-based chains should include this label.
	Cosmos Chain = "cosmos"

	// Specific chains follow.

	Gaia    Chain = "gaia"
	Osmosis Chain = "osmosis"
	Juno    Chain = "juno"
	Agoric  Chain = "agoric"

	Penumbra Chain = "penumbra"
)

var knownChainLabels = map[Chain]struct{}{
	Gaia:     {},
	Osmosis:  {},
	Juno:     {},
	Agoric:   {},
	Penumbra: {},
}

// RegisterChainLabel is available for external packages that may import ibctest,
// to register any external chain implementations they may provide.
func RegisterChainLabel(l Chain) {
	if _, exists := knownChainLabels[l]; exists {
		panic(fmt.Errorf("chain label %q already exists and must not be double registered", l))
	}

	knownChainLabels[l] = struct{}{}
}
