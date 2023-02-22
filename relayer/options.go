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

type OptionDockerImage struct {
	DockerImage ibc.DockerImage
}

// CustomDockerImage overrides the default relayer docker image.
// uidGid is the uid:gid format owner that should be used within the container.
// If uidGid is empty, root user will be assumed.
func CustomDockerImage(repository string, version string, uidGid string) Option {
	return OptionDockerImage{
		DockerImage: ibc.DockerImage{
			Repository: repository,
			Version:    version,
			UIDGid:     uidGid,
		},
	}
}

func (opt OptionDockerImage) relayerOption() {}

type OptionImagePull struct {
	Pull bool
}

func ImagePull(pull bool) Option {
	return OptionImagePull{
		Pull: pull,
	}
}

func (opt OptionImagePull) relayerOption() {}

type OptionExtraStartFlags struct {
	Flags []string
}

// StartupFlags appends additional flags when starting the relayer.
func StartupFlags(flags ...string) Option {
	return OptionExtraStartFlags{
		Flags: flags,
	}
}

func (opt OptionExtraStartFlags) relayerOption() {}
