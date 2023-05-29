package interchaintest

import (
	"fmt"
	"testing"

	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/relayer"
	"github.com/strangelove-ventures/interchaintest/v7/relayer/hermes"
	"github.com/strangelove-ventures/interchaintest/v7/relayer/hyperspace"
	"github.com/strangelove-ventures/interchaintest/v7/relayer/rly"
	"go.uber.org/zap"
)

// RelayerFactory describes how to start a Relayer.
type RelayerFactory interface {
	// Build returns a Relayer associated with the given arguments.
	Build(
		t *testing.T,
		cli *client.Client,
		networkID string,
	) ibc.Relayer

	// Name returns a descriptive name of the factory,
	// indicating details of the Relayer that will be built.
	Name() string

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
	cli *client.Client,
	networkID string,
) ibc.Relayer {
	switch f.impl {
	case ibc.CosmosRly:
		return rly.NewCosmosRelayer(
			f.log,
			t.Name(),
			cli,
			networkID,
			f.options...,
		)
	case ibc.Hyperspace:
		return hyperspace.NewHyperspaceRelayer(
			f.log,
			t.Name(),
			cli,
			networkID,
			f.options...,
		)
	case ibc.Hermes:
		return hermes.NewHermesRelayer(f.log, t.Name(), cli, networkID, f.options...)
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
			switch o := opt.(type) {
			case relayer.RelayerOptionDockerImage:
				return "rly@" + o.DockerImage.Version
			}
		}
		return "rly@" + rly.DefaultContainerVersion
	case ibc.Hermes:
		for _, opt := range f.options {
			switch o := opt.(type) {
			case relayer.RelayerOptionDockerImage:
				return "hermes@" + o.DockerImage.Version
			}
		}
		return "hermes@" + hermes.DefaultContainerVersion
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
	case ibc.Hermes:
		// TODO: specify capability for hermes.
		return rly.Capabilities()
	default:
		panic(fmt.Errorf("RelayerImplementation %v unknown", f.impl))
	}
}
