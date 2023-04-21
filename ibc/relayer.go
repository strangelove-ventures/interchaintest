package ibc

import (
	"context"
	"fmt"
	"time"

	chantypes "github.com/cosmos/ibc-go/v7/modules/core/04-channel/types"
	ptypes "github.com/cosmos/ibc-go/v7/modules/core/05-port/types"
	host "github.com/cosmos/ibc-go/v7/modules/core/24-host"
)

// Relayer represents an instance of a relayer that can be support IBC.
// Built-in implementations are run through Docker,
// but they could exec out to external processes
// or even be implemented in-process in Go.
//
// All of the methods on Relayer accept a RelayerExecReporter.
// It is intended that Relayer implementations will call the reporters' TrackRelayerExec method
// so that details of the relayer execution are included in the test report.
//
// If a relayer does not properly call into the reporter,
// the tests will still execute properly,
// but the report will be missing details.
type Relayer interface {
	// restore a mnemonic to be used as a relayer wallet for a chain
	RestoreKey(ctx context.Context, rep RelayerExecReporter, cfg ChainConfig, keyName, mnemonic string) error

	// generate a new key
	AddKey(ctx context.Context, rep RelayerExecReporter, chainID, keyName, coinType string) (Wallet, error)

	// GetWallet returns a Wallet for that relayer on the given chain and a boolean indicating if it was found.
	GetWallet(chainID string) (Wallet, bool)

	// add relayer configuration for a chain
	AddChainConfiguration(ctx context.Context, rep RelayerExecReporter, chainConfig ChainConfig, keyName, rpcAddr, grpcAddr string) error

	// generate new path between two chains
	GeneratePath(ctx context.Context, rep RelayerExecReporter, srcChainID, dstChainID, pathName string) error

	// setup channels, connections, and clients
	LinkPath(ctx context.Context, rep RelayerExecReporter, pathName string, channelOpts CreateChannelOptions, clientOptions CreateClientOptions) error

	// update path channel filter
	UpdatePath(ctx context.Context, rep RelayerExecReporter, pathName string, filter ChannelFilter) error

	// update clients, such as after new genesis
	UpdateClients(ctx context.Context, rep RelayerExecReporter, pathName string) error

	// get channel IDs for chain
	GetChannels(ctx context.Context, rep RelayerExecReporter, chainID string) ([]ChannelOutput, error)

	// GetConnections returns a slice of IBC connection details composed of the details for each connection on a specified chain.
	GetConnections(ctx context.Context, rep RelayerExecReporter, chainID string) (ConnectionOutputs, error)

	// GetClients returns a slice of IBC client details composed of the details for each client on a specified chain.
	GetClients(ctx context.Context, rep RelayerExecReporter, chainID string) (ClientOutputs, error)

	// After configuration is initialized, begin relaying.
	// This method is intended to create a background worker that runs the relayer.
	// You must call StopRelayer to cleanly stop the relaying.
	StartRelayer(ctx context.Context, rep RelayerExecReporter, pathNames ...string) error

	// StopRelayer stops a relayer that started work through StartRelayer.
	StopRelayer(ctx context.Context, rep RelayerExecReporter) error

	// Flush flushes any outstanding packets and then returns.
	Flush(ctx context.Context, rep RelayerExecReporter, pathName string, channelID string) error

	// CreateClients performs the client handshake steps necessary for creating a light client
	// on src that tracks the state of dst, and a light client on dst that tracks the state of src.
	CreateClients(ctx context.Context, rep RelayerExecReporter, pathName string, opts CreateClientOptions) error

	// CreateConnections performs the connection handshake steps necessary for creating a connection
	// between the src and dst chains.
	CreateConnections(ctx context.Context, rep RelayerExecReporter, pathName string) error

	// CreateChannel creates a channel on the given path with the provided options.
	CreateChannel(ctx context.Context, rep RelayerExecReporter, pathName string, opts CreateChannelOptions) error

	// UseDockerNetwork reports whether the relayer is run in the same docker network as the other chains.
	//
	// If false, the relayer will connect to the localhost-exposed ports instead of the docker hosts.
	//
	// Relayer implementations provided by the interchaintest module will report true,
	// but custom implementations may report false.
	UseDockerNetwork() bool

	// Exec runs an arbitrary relayer command.
	// If the Relayer implementation runs in Docker,
	// whether the invoked command is run in a one-off container or execing into an already running container
	// is an implementation detail.
	//
	// "env" are environment variables in the format "MY_ENV_VAR=value"
	Exec(ctx context.Context, rep RelayerExecReporter, cmd []string, env []string) RelayerExecResult

	// Set the wasm client contract hash in the chain's config if the counterparty chain in a path used 08-wasm
	// to instantiate the client.
	SetClientContractHash(ctx context.Context, rep RelayerExecReporter, cfg ChainConfig, hash string) error
}

// GetTransferChannel will return the transfer channel assuming only one client,
// one connection, and one channel with "transfer" port exists between two chains.
func GetTransferChannel(ctx context.Context, r Relayer, rep RelayerExecReporter, srcChainID, dstChainID string) (*ChannelOutput, error) {
	srcClients, err := r.GetClients(ctx, rep, srcChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get clients on source chain: %w", err)
	}

	if len(srcClients) == 0 {
		return nil, fmt.Errorf("no clients exist on source chain: %w", err)
	}

	var srcClientID string
	for _, client := range srcClients {
		// TODO continue for expired clients
		if client.ClientState.ChainID == dstChainID {
			if srcClientID != "" {
				return nil, fmt.Errorf("found multiple clients on %s tracking %s", srcChainID, dstChainID)
			}
			srcClientID = client.ClientID
		}
	}

	if srcClientID == "" {
		return nil, fmt.Errorf("unable to find client on %s tracking %s", srcChainID, dstChainID)
	}

	srcConnections, err := r.GetConnections(ctx, rep, srcChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get connections on source chain: %w", err)
	}

	if len(srcConnections) == 0 {
		return nil, fmt.Errorf("no connections exist on source chain: %w", err)
	}

	var srcConnectionID string
	for _, connection := range srcConnections {
		if connection.ClientID == srcClientID {
			if srcConnectionID != "" {
				return nil, fmt.Errorf("found multiple connections on %s for client %s", srcChainID, srcClientID)
			}
			srcConnectionID = connection.ID
		}
	}

	if srcConnectionID == "" {
		return nil, fmt.Errorf("unable to find connection on %s for client %s", srcChainID, srcClientID)
	}

	srcChannels, err := r.GetChannels(ctx, rep, srcChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channels on source chain: %w", err)
	}

	if len(srcChannels) == 0 {
		return nil, fmt.Errorf("no channels exist on source chain: %w", err)
	}

	var srcChan *ChannelOutput
	for _, channel := range srcChannels {
		if len(channel.ConnectionHops) == 1 && channel.ConnectionHops[0] == srcConnectionID && channel.PortID == "transfer" {
			if srcChan != nil {
				return nil, fmt.Errorf("found multiple transfer channels on %s for connection %s", srcChainID, srcConnectionID)
			}
			srcChan = &channel
		}
	}

	if srcChan == nil {
		return nil, fmt.Errorf("no transfer channel found between chains: %s - %s", srcChainID, dstChainID)
	}

	return srcChan, nil
}

// RelyaerExecResult holds the details of a call to Relayer.Exec.
type RelayerExecResult struct {
	// This type is a redeclaration of dockerutil.ContainerExecResult.
	// While most relayer implementations are in Docker,
	// the dockerutil package is and will continue to be internal,
	// so we need an externally importable type for third-party Relayer implementations.
	//
	// A type alias would be a potential fit here
	// (i.e. type RelayerExecResult = dockerutil.ContainerExecResult)
	// but that would be slightly misleading as not all implementations are in Docker;
	// and the type is small enough and has no methods associated,
	// so a redeclaration keeps things simple for external implementers.

	// Err is only set when there is a failure to execute.
	// A successful execution that exits non-zero will have a nil Err
	// and an appropriate ExitCode.
	Err error

	ExitCode       int
	Stdout, Stderr []byte
}

// CreateChannelOptions contains the configuration for creating a channel.
type CreateChannelOptions struct {
	SourcePortName string
	DestPortName   string

	Order Order

	Version string
}

// DefaultChannelOpts returns the default settings for creating an ics20 fungible token transfer channel.
func DefaultChannelOpts() CreateChannelOptions {
	return CreateChannelOptions{
		SourcePortName: "transfer",
		DestPortName:   "transfer",
		Order:          Unordered,
		Version:        "ics20-1",
	}
}

// Validate will check that the specified CreateChannelOptions are valid.
func (opts CreateChannelOptions) Validate() error {
	switch {
	case host.PortIdentifierValidator(opts.SourcePortName) != nil:
		return ptypes.ErrInvalidPort
	case host.PortIdentifierValidator(opts.DestPortName) != nil:
		return ptypes.ErrInvalidPort
	case opts.Version == "":
		return fmt.Errorf("invalid channel version")
	case opts.Order.Validate() != nil:
		return chantypes.ErrInvalidChannelOrdering
	}
	return nil
}

// Order represents an IBC channel's ordering.
type Order int

const (
	Invalid Order = iota
	Ordered
	Unordered
)

// String returns the lowercase string representation of the Order.
func (o Order) String() string {
	switch o {
	case Unordered:
		return "unordered"
	case Ordered:
		return "ordered"
	default:
		return "invalid"
	}
}

// Validate checks that the Order type is a valid value.
func (o Order) Validate() error {
	if o == Ordered || o == Unordered {
		return nil
	}
	return chantypes.ErrInvalidChannelOrdering
}

// CreateClientOptions contains the configuration for creating a client.
type CreateClientOptions struct {
	TrustingPeriod string
}

// DefaultClientOpts returns the default settings for creating clients.
// These default options are usually determined by the relayer
func DefaultClientOpts() CreateClientOptions {
	return CreateClientOptions{
		TrustingPeriod: "0",
	}
}

func (opts CreateClientOptions) Validate() error {
	_, err := time.ParseDuration(opts.TrustingPeriod)
	if err != nil {
		return err
	}
	return nil
}

// ExecReporter is the interface of a narrow type returned by testreporter.RelayerExecReporter.
// This avoids a direct dependency on the testreporter package,
// and it avoids the relayer needing to be aware of a *testing.T.
type RelayerExecReporter interface {
	TrackRelayerExec(
		// The name of the docker container in which this relayer command executed,
		// or empty if it did not run in docker.
		containerName string,

		// The command line passed to this invocation of the relayer.
		command []string,

		// The standard output and standard error that the relayer produced during this invocation.
		stdout, stderr string,

		// The exit code of executing the command.
		// This field may not be applicable for e.g. an in-process relayer implementation.
		exitCode int,

		// When the command started and finished.
		startedAt, finishedAt time.Time,

		// Any error that occurred during execution.
		// This indicates a failure to execute,
		// e.g. the relayer binary not being found, failure communicating with Docker, etc.
		// If the process completed with a non-zero exit code,
		// those details should be indicated between stdout, stderr, and exitCode.
		err error,
	)
}

// NopRelayerExecReporter is a no-op RelayerExecReporter.
type NopRelayerExecReporter struct{}

func (NopRelayerExecReporter) TrackRelayerExec(string, []string, string, string, int, time.Time, time.Time, error) {
}
