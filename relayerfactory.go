package interchaintest

import (
	"fmt"

	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
	"github.com/strangelove-ventures/interchaintest/v8/relayer/hermes"
	"github.com/strangelove-ventures/interchaintest/v8/relayer/rly"
	"go.uber.org/zap"
)

type TestName interface {
	Name() string
}

// RelayerFactory describes how to start a Relayer.
type RelayerFactory interface {
	// Build returns a Relayer associated with the given arguments.
	Build(
		t TestName,
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
	options []relayer.RelayerOpt
	version string
}

func NewBuiltinRelayerFactory(impl ibc.RelayerImplementation, logger *zap.Logger, options ...relayer.RelayerOpt) RelayerFactory {
	return &builtinRelayerFactory{impl: impl, log: logger, options: options}
}

// Build returns a relayer chosen depending on f.impl.
func (f *builtinRelayerFactory) Build(
	t TestName,
	cli *client.Client,
	networkID string,
) ibc.Relayer {
	switch f.impl {
	case ibc.CosmosRly:
		r := rly.NewCosmosRelayer(
			f.log,
			t.Name(),
			cli,
			networkID,
			f.options...,
		)
		f.setRelayerVersion(r.ContainerImage())
		return r
	case ibc.Hermes:
		r := hermes.NewHermesRelayer(f.log, t.Name(), cli, networkID, f.options...)
		f.setRelayerVersion(r.ContainerImage())
		return r
	default:
		panic(fmt.Errorf("RelayerImplementation %v unknown", f.impl))
	}
}

func (f *builtinRelayerFactory) setRelayerVersion(di ibc.DockerImage) {
	f.version = di.Version
}

func (f *builtinRelayerFactory) Name() string {
	switch f.impl {
	case ibc.CosmosRly:
		if f.version == "" {
			return "rly@" + f.version
		}
		return "rly@" + rly.DefaultContainerVersion
	case ibc.Hermes:
		if f.version == "" {
			return "hermes@" + f.version
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
