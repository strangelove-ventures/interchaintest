package types

import (
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
)

type Config struct {
	Chains  []Chain    `json:"chains"`
	Relayer Relayer    `json:"relayer"`
	Server  RestServer `json:"server"`
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

type IBCChannel struct {
	ChainID string             `json:"chain_id"`
	Channel *ibc.ChannelOutput `json:"channel"`
}
