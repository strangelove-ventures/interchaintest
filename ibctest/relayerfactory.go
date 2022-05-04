package ibctest

import (
	"fmt"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"
	"github.com/strangelove-ventures/ibc-test-framework/log"
	"github.com/strangelove-ventures/ibc-test-framework/relayer"
	"github.com/strangelove-ventures/ibc-test-framework/relayer/rly"
)

// RelayerFactory describes how to start a Relayer.
type RelayerFactory interface {
	// Build returns a Relayer associated with the given arguments.
	Build(
		t *testing.T,
		pool *dockertest.Pool,
		networkID string,
		home string,
	) ibc.Relayer

	// Capabilities is an indication of the features this relayer supports.
	// Tests for any unsupported features will be skipped rather than failed.
	Capabilities() map[relayer.Capability]bool

	// UseDockerNetwork reports whether the relayer is run in the same docker network as the other chains.
	//
	// If false, the relayer will connect to the localhost-exposed ports instead of the docker hosts.
	UseDockerNetwork() bool
}

// builtinRelayerFactory is the built-in relayer factory that understands
// how to start the cosmos relayer in a docker container.
type builtinRelayerFactory struct {
	impl ibc.RelayerImplementation
	log  log.Logger
}

func NewBuiltinRelayerFactory(impl ibc.RelayerImplementation, logger log.Logger) RelayerFactory {
	return builtinRelayerFactory{impl: impl, log: logger}
}

// Build returns a relayer chosen depending on f.impl.
func (f builtinRelayerFactory) Build(
	t *testing.T,
	pool *dockertest.Pool,
	networkID string,
	home string,
) ibc.Relayer {
	switch f.impl {
	case ibc.CosmosRly:
		return rly.NewCosmosRelayerFromChains(
			t,
			pool,
			networkID,
			home,
			f.log,
		)
	default:
		panic(fmt.Errorf("RelayerImplementation %v unknown", f.impl))
	}
}

// Capabilities returns the set of capabilities for the
// relayer implementation backing this factory.
func (f builtinRelayerFactory) Capabilities() map[relayer.Capability]bool {
	switch f.impl {
	case ibc.CosmosRly:
		return rly.Capabilities()
	default:
		panic(fmt.Errorf("RelayerImplementation %v unknown", f.impl))
	}
}

// UseDockerNetwork reports true.
func (f builtinRelayerFactory) UseDockerNetwork() bool {
	return true
}
