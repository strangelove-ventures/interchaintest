package relayer

import (
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
)

// RelayerOption is used to customize the relayer configuration, whether constructed with the
// RelayerFactory or with the more specific NewDockerRelayer or NewCosmosRelayer methods.
type Option interface {
	// relayerOption is a no-op to be more restrictive on what types can be used as RelayerOptions
	relayerOption()
}
type Options []Option

type RelayerOptionDockerImage struct {
	DockerImage ibc.DockerImage
}

// CustomDockerImage overrides the default relayer docker image.
// uidGid is the uid:gid format owner that should be used within the container.
// If uidGid is empty, root user will be assumed.
func CustomDockerImage(repository string, version string, uidGid string) Option {
	return RelayerOptionDockerImage{
		DockerImage: ibc.DockerImage{
			Repository: repository,
			Version:    version,
			UidGid:     uidGid,
		},
	}
}

func (opt RelayerOptionDockerImage) relayerOption() {}

type RelayerOptionImagePull struct {
	Pull bool
}

func ImagePull(pull bool) Option {
	return RelayerOptionImagePull{
		Pull: pull,
	}
}

func (opt RelayerOptionImagePull) relayerOption() {}

type RelayerOptionExtraStartFlags struct {
	Flags []string
}

// StartupFlags appends additional flags when starting the relayer.
func StartupFlags(flags ...string) Option {
	return RelayerOptionExtraStartFlags{
		Flags: flags,
	}
}

func (opt RelayerOptionExtraStartFlags) relayerOption() {}
