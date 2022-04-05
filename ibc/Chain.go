package ibc

import (
	"context"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/ory/dockertest"
)

type ChainConfig struct {
	Type           string
	Name           string
	ChainID        string
	Repository     string
	Version        string
	Bin            string
	Bech32Prefix   string
	Denom          string
	GasPrices      string
	GasAdjustment  float64
	TrustingPeriod string
}

type WalletAmount struct {
	Address string
	Denom   string
	Amount  int64
}

type IBCTimeout struct {
	NanoSeconds uint64
	Height      uint64
}

type Chain interface {
	// fetch chain configuration
	Config() ChainConfig

	// initializes node structs so that things like initializing keys can be done before starting the chain
	Initialize(testName string, homeDirectory string, dockerPool *dockertest.Pool, networkID string) error

	// sets up everything needed (validators, gentx, fullnodes, peering, additional accounts) for chain to start from genesis
	Start(testName string, ctx context.Context, additionalGenesisWallets []WalletAmount) error

	// retrieves rpc address that can be reached by other containers in the docker network
	GetRPCAddress() string

	// retrieves grpc address that can be reached by other containers in the docker network
	GetGRPCAddress() string

	// creates a test key in the "user" node, (either the first fullnode or the first validator if no fullnodes)
	CreateKey(ctx context.Context, keyName string) error

	// fetches the bech32 address for a test key on the "user" node (either the first fullnode or the first validator if no fullnodes)
	GetAddress(keyName string) ([]byte, error)

	// send funds to wallet from user account
	SendFunds(ctx context.Context, keyName string, amount WalletAmount) error

	// sends an IBC transfer from a test key on the "user" node (either the first fullnode or the first validator if no fullnodes)
	// returns tx hash
	SendIBCTransfer(ctx context.Context, channelID, keyName string, amount WalletAmount, timeout *IBCTimeout) (string, error)

	// takes file path to smart contract and initialization message. returns contract address
	InstantiateContract(ctx context.Context, keyName string, amount WalletAmount, fileName, initMessage string, needsNoContactFlag bool) (string, error)

	// executes a contract transaction with a message using it's address
	ExecuteContract(ctx context.Context, keyName string, contractAddress string, message string) error

	// create balancer pool
	CreatePool(ctx context.Context, keyName string, contractAddress string, swapFee float64, exitFee float64, assets []WalletAmount) error

	// waits for # of blocks to be produced
	WaitForBlocks(number int64) error

	// fetch balance for a specific account address and denom
	GetBalance(ctx context.Context, address string, denom string) (int64, error)

	// fetch transaction
	GetTransaction(ctx context.Context, txHash string) (*types.TxResponse, error)
}
