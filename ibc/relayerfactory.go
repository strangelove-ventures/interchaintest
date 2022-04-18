package ibc

import (
	"fmt"

	"github.com/ory/dockertest"
)

// RelayerFactory describes how to start a Relayer.
type RelayerFactory interface {
	// Build returns a Relayer associated with the given arguments.
	Build(
		testName string,
		pool *dockertest.Pool,
		networkID string,
		home string,
		srcChain, dstChain Chain,
	) Relayer

	// UseDockerNetwork reports whether the relayer is run in the same docker network as the other chains.
	//
	// If false, the relayer will connect to the localhost-exposed ports instead of the docker hosts.
	UseDockerNetwork() bool
}

// builtinRelayerFactory is the built-in relayer factory that understands
// how to start the cosmos relayer in a docker container.
type builtinRelayerFactory struct {
	impl RelayerImplementation
}

// Build returns a relayer chosen depending on f.impl.
func (f builtinRelayerFactory) Build(
	testName string,
	pool *dockertest.Pool,
	networkID string,
	home string,
	srcChain, dstChain Chain,
) Relayer {
	switch f.impl {
	case CosmosRly:
		return NewCosmosRelayerFromChains(
			testName,
			srcChain,
			dstChain,
			pool,
			networkID,
			home,
		)
	default:
		panic(fmt.Errorf("RelayerImplementation %v unknown", f.impl))
	}
}

// UseDockerNetwork reports true.
func (f builtinRelayerFactory) UseDockerNetwork() bool {
	return true
}
