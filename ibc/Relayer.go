package ibc

import (
	"context"
)

type WalletAmount struct {
	Address string
	Denom   string
	Amount  int64
}

type Relayer interface {
	// restore a mnemonic to be used as a relayer wallet for a chain
	RestoreKey(ctx context.Context, chainID, keyName, mnemonic string) error

	// add relayer configuration for a chain
	AddChainConfiguration(ctx context.Context, chainConfig ChainConfig, keyName, rpcAddr, grpcAddr string) error

	// generate new path between two chains
	GeneratePath(ctx context.Context, srcChainID, dstChainID, pathName string) error

	// after configuration is initialized, begin relaying
	StartRelayer(ctx context.Context, pathName string) error

	// wait until all existing packets are relayed
	ClearQueue(ctx context.Context) error

	// shutdown relayer
	StopRelayer(ctx context.Context) error
}
