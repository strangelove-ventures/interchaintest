package relayer

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	cosmosRelayer "github.com/cosmos/relayer/relayer"
)

type CosmosRelayer struct {
	path    *cosmosRelayer.Path
	src     *cosmosRelayer.Chain
	dst     *cosmosRelayer.Chain
	rlyDone func()
}

func NewCosmosRelayerFromChains(src, dst *cosmosRelayer.Chain) *CosmosRelayer {
	path := cosmosRelayer.GenPath(src.ChainID, dst.ChainID, "transfer", "transfer", "UNORDERED", "ics20-1")
	src.PathEnd = path.Src
	dst.PathEnd = path.Dst
	return &CosmosRelayer{
		path: path,
		src:  src,
		dst:  dst,
	}
}

func NewCosmosRelayerFromPath(path *cosmosRelayer.Path) *CosmosRelayer {
	return &CosmosRelayer{
		path: path,
		src:  cosmosRelayer.UnmarshalChain(*path.Src),
		dst:  cosmosRelayer.UnmarshalChain(*path.Dst),
	}
}

// Implements Relayer interface
func (relayer *CosmosRelayer) StartRelayer() error {
	_, err := relayer.src.CreateClients(relayer.dst, true, true, false)
	if err != nil {
		return err
	}
	timeout := relayer.src.GetTimeout()

	_, err = relayer.src.CreateOpenConnections(relayer.dst, 3, timeout)
	if err != nil {
		return err
	}

	_, err = relayer.src.CreateOpenChannels(relayer.dst, 3, timeout)
	if err != nil {
		return err
	}

	relayer.rlyDone, err = cosmosRelayer.RunStrategy(relayer.src, relayer.dst, relayer.path.MustGetStrategy())
	if err != nil {
		return err
	}

	return nil
}

// Implements Relayer interface
func (relayer *CosmosRelayer) RelayPacketFromSource(amount sdk.Coin, dstAddr string) error {
	return relayer.src.SendTransferMsg(relayer.dst, amount, dstAddr, 0, 0)
}

// Implements Relayer interface
func (relayer *CosmosRelayer) RelayPacketFromDestination(amount sdk.Coin, dstAddr string) error {
	return relayer.dst.SendTransferMsg(relayer.src, amount, dstAddr, 0, 0)
}

// Implements Relayer interface
func (relayer *CosmosRelayer) StopRelayer() error {
	relayer.rlyDone()
	return nil
}
