package ibc

import (
	"context"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/ory/dockertest/v3"
)

type Chain interface {
	// fetch chain configuration
	Config() ChainConfig

	// initializes node structs so that things like initializing keys can be done before starting the chain
	Initialize(testName string, homeDirectory string, dockerPool *dockertest.Pool, networkID string) error

	// sets up everything needed (validators, gentx, fullnodes, peering, additional accounts) for chain to start from genesis
	Start(testName string, ctx context.Context, additionalGenesisWallets ...WalletAmount) error

	// start a chain with a provided genesis file. Will override validators for first 2/3 of voting power
	StartWithGenesisFile(testName string, ctx context.Context, home string, pool *dockertest.Pool, networkID string, genesisFilePath string) error

	// export state at specific height
	ExportState(ctx context.Context, height int64) (string, error)

	// retrieves rpc address that can be reached by other containers in the docker network
	GetRPCAddress() string

	// retrieves grpc address that can be reached by other containers in the docker network
	GetGRPCAddress() string

	// GetHostRPCAddress returns the rpc address that can be reached by processes on the host machine.
	// Note that this will not return a valid value until after Start returns.
	GetHostRPCAddress() string

	// GetHostGRPCAddress returns the grpc address that can be reached by processes on the host machine.
	// Note that this will not return a valid value until after Start returns.
	GetHostGRPCAddress() string

	// get current height
	Height() (int64, error)

	// creates a test key in the "user" node, (either the first fullnode or the first validator if no fullnodes)
	CreateKey(ctx context.Context, keyName string) error

	// fetches the bech32 address for a test key on the "user" node (either the first fullnode or the first validator if no fullnodes)
	GetAddress(ctx context.Context, keyName string) ([]byte, error)

	// send funds to wallet from user account
	SendFunds(ctx context.Context, keyName string, amount WalletAmount) error

	// sends an IBC transfer from a test key on the "user" node (either the first fullnode or the first validator if no fullnodes)
	// returns tx hash
	SendIBCTransfer(ctx context.Context, channelID, keyName string, amount WalletAmount, timeout *IBCTimeout) (string, error)

	// takes file path to smart contract and initialization message. returns contract address
	InstantiateContract(ctx context.Context, keyName string, amount WalletAmount, fileName, initMessage string, needsNoAdminFlag bool) (string, error)

	// executes a contract transaction with a message using it's address
	ExecuteContract(ctx context.Context, keyName string, contractAddress string, message string) error

	// dump state of contract at block height
	DumpContractState(ctx context.Context, contractAddress string, height int64) (*DumpContractStateResponse, error)

	// create balancer pool
	CreatePool(ctx context.Context, keyName string, contractAddress string, swapFee float64, exitFee float64, assets []WalletAmount) error

	// waits for # of blocks to be produced. Returns latest height
	WaitForBlocks(number int64) (int64, error)

	// fetch balance for a specific account address and denom
	GetBalance(ctx context.Context, address string, denom string) (int64, error)

	// get the fees in native denom for an amount of spent gas
	GetGasFeesInNativeDenom(gasPaid int64) int64

	// fetch transaction
	GetTransaction(ctx context.Context, txHash string) (*types.TxResponse, error)

	// GetPacketAcknowledgment fetches ibc packet ack or an error if not found
	GetPacketAcknowledgment(ctx context.Context, portID, channelID string, seq uint64) (PacketAcknowledgment, error)

	// GetPacketSequence returns the packet sequence given the transaction's hash
	GetPacketSequence(ctx context.Context, txHash string) (uint64, error)

	// cleanup any resources that won't be cleaned up by container and test file teardown
	// for example if containers use a different user, and need the files to be deleted inside the container
	Cleanup(ctx context.Context) error
}
