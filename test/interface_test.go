package test

import (
	"testing"

	"github.com/cosmos/cosmos-sdk/simapp/params"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
)

// ChainType represents the type of chain to instantiate
type ChainType struct {
	Name         string
	Repository   string
	Version      string
	Bin          string
	Bech32Prefix string
	Ports        map[docker.Port]struct{}
}

// TestNode represents a node in the test network that is being created
type TestNode struct {
	Home         string
	Index        int
	ChainID      string
	Chain        *ChainType
	GenesisCoins string
	Validator    bool
	Pool         *dockertest.Pool
	Client       rpcclient.Client
	Container    *docker.Container
	t            *testing.T
	ec           params.EncodingConfig
}

type ContainerPort struct {
	Name      string
	Container *docker.Container
	Port      docker.Port
}

type Hosts []ContainerPort
