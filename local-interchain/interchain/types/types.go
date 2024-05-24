package types

import (
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

type Config struct {
	Chains  []Chain    `json:"chains" yaml:"chains"`
	Relayer Relayer    `json:"relayer" yaml:"relayer"`
	Server  RestServer `json:"server" yaml:"server"`
}

type AppStartConfig struct {
	Address string
	Port    uint16

	Cfg *Config

	Relayer Relayer
	AuthKey string // optional password for API interaction
}

type RestServer struct {
	Host string `json:"host" yaml:"host"`
	Port string `json:"port" yaml:"port"`
}

type DockerImage struct {
	Repository string `json:"repository" yaml:"repository"`
	Version    string `json:"version" yaml:"version"`
	UidGid     string `json:"uid_gid" yaml:"uid_gid"`
}

type Relayer struct {
	DockerImage  DockerImage `json:"docker_image" yaml:"docker_image"`
	StartupFlags []string    `json:"startup_flags" yaml:"startup_flags"`
}

type IBCChannel struct {
	ChainID string             `json:"chain_id" yaml:"chain_id"`
	Channel *ibc.ChannelOutput `json:"channel" yaml:"channel"`
}

// ConfigFileOverrides overrides app toml configuration files.
type ConfigFileOverrides struct {
	File  string        `json:"file" yaml:"file"`
	Paths testutil.Toml `json:"paths" yaml:"paths"`
}

// ChainsConfig is the chain configuration for the file.
type ChainsConfig struct {
	Chains []Chain `json:"chains" yaml:"chains"`
}
