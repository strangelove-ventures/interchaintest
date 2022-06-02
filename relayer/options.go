package relayer

import (
	"github.com/strangelove-ventures/ibctest/ibc"
)

type RelayerOption interface{}
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

type RelayerOptionImagePull struct {
	Pull bool
}

func ImagePull(pull bool) RelayerOption {
	return RelayerOptionImagePull{
		Pull: pull,
	}
}
