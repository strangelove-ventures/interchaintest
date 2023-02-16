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

// RegisterRelayerLabel is available for external packages that may import interchaintest,
// to register any external relayer implementations they may provide.
func RegisterRelayerLabel(l Relayer) {
	if _, exists := knownRelayerLabels[l]; exists {
		panic(fmt.Errorf("relayer label %q already exists and must not be double registered", l))
	}

	knownRelayerLabels[l] = struct{}{}
}
