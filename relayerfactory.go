package ibctest

import (
	"fmt"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/strangelove-ventures/ibctest/label"
	"github.com/strangelove-ventures/ibctest/relayer"
	"github.com/strangelove-ventures/ibctest/relayer/rly"
	"go.uber.org/zap"
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

	// Name returns a descriptive name of the factory,
	// indicating details of the Relayer that will be built.
	Name() string

	// Labels are reported to allow simple filtering of tests depending on this Relayer.
	// While the Name should be fully descriptive,
	// the Labels are intended to be short and fixed.
	//
	// Most relayers will probably only have one label indicative of its name,
	// but we allow multiple labels for future compatibility.
	Labels() []label.Relayer

	// Capabilities is an indication of the features this relayer supports.
	// Tests for any unsupported features will be skipped rather than failed.
	Capabilities() map[relayer.Capability]bool
}

// builtinRelayerFactory is the built-in relayer factory that understands
// how to start the cosmos relayer in a docker container.
type builtinRelayerFactory struct {
	impl    ibc.RelayerImplementation
	log     *zap.Logger
	options relayer.RelayerOptions
}

func NewBuiltinRelayerFactory(impl ibc.RelayerImplementation, logger *zap.Logger, options ...relayer.RelayerOption) RelayerFactory {
	return builtinRelayerFactory{impl: impl, log: logger, options: options}
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
		return rly.NewCosmosRelayer(
			f.log,
			t.Name(),
			home,
			pool,
			networkID,
			f.options...,
		)
	default:
		panic(fmt.Errorf("RelayerImplementation %v unknown", f.impl))
	}
}

func (f builtinRelayerFactory) Name() string {
	switch f.impl {
	case ibc.CosmosRly:
		// This is using the string "rly" instead of rly.ContainerImage
		// so that the slashes in the image repository don't add ambiguity
		// to subtest paths, when the factory name is used in calls to t.Run.
		for _, opt := range f.options {
			switch typedOpt := opt.(type) {
			case relayer.RelayerOptionDockerImage:
				return "rly@" + typedOpt.DockerImage.Version
			}
		}
		return "rly@" + rly.DefaultContainerVersion
	default:
		panic(fmt.Errorf("RelayerImplementation %v unknown", f.impl))
	}
}

func (f builtinRelayerFactory) Labels() []label.Relayer {
	switch f.impl {
	case ibc.CosmosRly:
		return []label.Relayer{label.Rly}
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
