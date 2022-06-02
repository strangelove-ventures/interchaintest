package relayer

import (
	"github.com/strangelove-ventures/ibctest/ibc"
)

// RelayerOption is used to customize the relayer configuration, whether constructed with the
// RelayerFactory or with the more specific NewDockerRelayer or NewCosmosRelayer methods.
type RelayerOption interface {
	// relayerOption is a no-op to be more restrictive on what types can be used as RelayerOptions
	relayerOption()
}
type RelayerOptions []RelayerOption

type RelayerOptionDockerImage struct {
	DockerImage ibc.DockerImage
}

func CustomDockerImage(repository string, version string) RelayerOption {
	return RelayerOptionDockerImage{
		DockerImage: ibc.DockerImage{
			Repository: repository,
			Version:    version,
		},
	}
}

func (opt RelayerOptionDockerImage) relayerOption() {}

type RelayerOptionImagePull struct {
	Pull bool
}

func ImagePull(pull bool) RelayerOption {
	return RelayerOptionImagePull{
		Pull: pull,
	}
}

func (opt RelayerOptionImagePull) relayerOption() {}
