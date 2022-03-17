package relayer

type WalletAmount struct {
	Mnemonic string
	Address  string
	Denom    string
	Amount   int64
}

type Relayer interface {
	StartRelayer() error

	InitializeSourceWallet() (WalletAmount, error)

	InitializeDestinationWallet() (WalletAmount, error)

	SetSourceRPC(rpcAddress string) error

	SetDestinationRPC(rpcAddress string) error

	GetSourceBalance(denom string) (WalletAmount, error)

	GetDestinationBalance(denom string) (WalletAmount, error)

	RelayPacketFromSource(amount WalletAmount) error

	RelayPacketFromDestination(amount WalletAmount) error

	StopRelayer() error
}
