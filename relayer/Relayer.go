package relayer

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type Relayer interface {
	StartRelayer() error

	RelayPacketFromSource(amount sdk.Coin, dstAddr string) error

	RelayPacketFromDestination(amount sdk.Coin, dstAddr string) error

	StopRelayer() error
}
