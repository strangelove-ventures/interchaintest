package types

import (
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

type Config struct {
	Chains  []Chain    `json:"chains"`
	Relayer Relayer    `json:"relayer"`
	Server  RestServer `json:"server"`
}

type AppStartConfig struct {
	Address string
	Port    uint16

	Cfg *Config

	Relayer Relayer
	AuthKey string // optional password for API interaction
}

type RestServer struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

type DockerImage struct {
	Repository string `json:"repository"`
	Version    string `json:"version"`
	UidGid     string `json:"uid_gid"`
}

type Relayer struct {
	DockerImage  DockerImage `json:"docker_image"`
	StartupFlags []string    `json:"startup_flags"`
}

type IBCChannel struct {
	ChainID string             `json:"chain_id"`
	Channel *ibc.ChannelOutput `json:"channel"`
}
