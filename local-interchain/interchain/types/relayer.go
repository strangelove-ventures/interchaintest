package types

type Relayer struct {
	DockerImage  DockerImage `json:"docker_image"`
	StartupFlags []string    `json:"startup_flags"`
}

func (r Relayer) SetRelayerDefaults() Relayer {
	if r.DockerImage.Repository == "" {
		r.DockerImage.Repository = "ghcr.io/cosmos/relayer"
	}

	if r.DockerImage.Version == "" {
		r.DockerImage.Version = "latest"
	}

	if r.DockerImage.UidGid == "" {
		r.DockerImage.UidGid = "100:1000"
	}

	return r
}
