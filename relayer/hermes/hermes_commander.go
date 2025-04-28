package hermes

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	ibcexported "github.com/cosmos/ibc-go/v8/modules/core/03-connection/types"
	"github.com/cosmos/ibc-go/v8/modules/core/23-commitment/types"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
)

var _ relayer.RelayerCommander = &commander{}

type commander struct {
	log             *zap.Logger
	extraStartFlags []string
}

func NewHermesCommander() relayer.RelayerCommander {
	return &commander{}
}

func (c commander) Name() string {
	return hermes
}

func (c commander) DefaultContainerImage() string {
	return defaultContainerImage
}

func (c commander) DefaultContainerVersion() string {
	return DefaultContainerVersion
}

func (c commander) DockerUser() string {
	return hermesDefaultUIDGID
}

func (c commander) ParseGetChannelsOutput(stdout, stderr string) ([]ibc.ChannelOutput, error) {
	jsonBz := extractJSONResult([]byte(stdout))
	var result ChannelOutputResult
	if err := json.Unmarshal(jsonBz, &result); err != nil {
		return nil, err
	}

	var ibcChannelOutput []ibc.ChannelOutput
	for _, r := range result.Result {
		ibcChannelOutput = append(ibcChannelOutput, ibc.ChannelOutput{
			State:    r.ChannelEnd.State,
			Ordering: r.ChannelEnd.Ordering,
			Counterparty: ibc.ChannelCounterparty{
				PortID:    r.ChannelEnd.Remote.PortID,
				ChannelID: r.ChannelEnd.Remote.ChannelID,
			},
			ConnectionHops: r.ChannelEnd.ConnectionHops,
			Version:        r.ChannelEnd.Version,
			PortID:         r.CounterPartyChannelEnd.Remote.PortID,
			ChannelID:      r.CounterPartyChannelEnd.Remote.ChannelID,
		})
	}

	return ibcChannelOutput, nil
}

func (c commander) ParseGetConnectionsOutput(stdout, stderr string) (ibc.ConnectionOutputs, error) {
	jsonBz := extractJSONResult([]byte(stdout))
	var queryResult ConnectionQueryResult
	if err := json.Unmarshal(jsonBz, &queryResult); err != nil {
		return ibc.ConnectionOutputs{}, err
	}

	var outputs ibc.ConnectionOutputs
	for _, r := range queryResult.Result {
		var versions []*ibcexported.Version
		for _, v := range r.ConnectionEnd.Versions {
			versions = append(versions, &ibcexported.Version{
				Identifier: v.Identifier,
				Features:   v.Features,
			})
		}

		outputs = append(outputs, &ibc.ConnectionOutput{
			ID:       r.ConnectionID,
			ClientID: r.ConnectionEnd.ClientID,
			Versions: versions,
			State:    r.ConnectionEnd.State,
			Counterparty: &ibcexported.Counterparty{
				ClientId:     r.ConnectionEnd.Counterparty.ClientID,
				ConnectionId: r.ConnectionEnd.Counterparty.ConnectionID,
				Prefix: types.MerklePrefix{
					KeyPrefix: []byte(r.ConnectionEnd.Counterparty.Prefix),
				},
			},
		})
	}
	return outputs, nil
}

func (c commander) ParseGetClientsOutput(stdout, stderr string) (ibc.ClientOutputs, error) {
	jsonBz := extractJSONResult([]byte(stdout))
	var queryResult ClientQueryResult
	if err := json.Unmarshal(jsonBz, &queryResult); err != nil {
		return ibc.ClientOutputs{}, err
	}

	var clientOutputs []*ibc.ClientOutput
	for _, r := range queryResult.ClientResult {
		clientOutputs = append(clientOutputs, &ibc.ClientOutput{
			ClientID: r.ClientID,
			ClientState: ibc.ClientState{
				ChainID: r.ChainID,
			},
		})
	}

	return clientOutputs, nil
}

func (c commander) Init(homeDir string) []string {
	return nil
}

func (c commander) GetChannels(chainID, homeDir string) []string {
	// the --verbose and --show-counterparty options are required to get enough information to correctly populate
	// the path.
	return []string{hermes, "--json", "query", "channels", "--chain", chainID, "--show-counterparty", "--verbose"}
}

func (c commander) GetConnections(chainID, homeDir string) []string {
	return []string{hermes, "--config", fmt.Sprintf("%s/%s", homeDir, hermesConfigPath), "--json", "query", "connections", "--chain", chainID, "--verbose"}
}

func (c commander) GetClients(chainID, homeDir string) []string {
	return []string{hermes, "--config", fmt.Sprintf("%s/%s", homeDir, hermesConfigPath), "--json", "query", "clients", "--host-chain", chainID}
}

func (c commander) StartRelayer(homeDir string, pathNames ...string) []string {
	cmd := []string{hermes, "--config", fmt.Sprintf("%s/%s", homeDir, hermesConfigPath), "start"}
	cmd = append(cmd, c.extraStartFlags...)
	return cmd
}

func (c commander) CreateWallet(keyName, address, mnemonic string) ibc.Wallet {
	return NewWallet(keyName, address, mnemonic)
}

func (c commander) UpdatePath(pathName, homeDir string, opts ibc.PathUpdateOptions) []string {
	panic("update path implemented in hermes relayer not the commander")
}

// the following methods do not have a single command that cleanly maps to a single hermes command without
// additional logic wrapping them. They have been implemented one layer up in the hermes relayer.

func (c commander) UpdateClients(pathName, homeDir string) []string {
	panic("update clients implemented in hermes relayer not the commander")
}

func (c commander) GeneratePath(srcChainID, dstChainID, pathName, homeDir string) []string {
	panic("generate path implemented in hermes relayer not the commander")
}

func (c commander) LinkPath(pathName, homeDir string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) []string {
	panic("link path implemented in hermes relayer not the commander")
}

func (c commander) RestoreKey(chainID, keyName, coinType, signingAlgorithm, mnemonic, homeDir string) []string {
	panic("restore key implemented in hermes relayer not the commander")
}

func (c commander) AddChainConfiguration(containerFilePath, homeDir string) []string {
	panic("add chain configuration implemented in hermes relayer not the commander")
}

func (c commander) AddKey(chainID, keyName, coinType, signingAlgorithm, homeDir string) []string {
	panic("add key implemented in hermes relayer not the commander")
}

func (c commander) CreateChannel(pathName string, opts ibc.CreateChannelOptions, homeDir string) []string {
	panic("create channel implemented in hermes relayer not the commander")
}

func (c commander) CreateClients(pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	panic("create clients implemented in hermes relayer not the commander")
}

func (c commander) CreateClient(srcChainID, dstChainID, pathName string, opts ibc.CreateClientOptions, homeDir string) []string {
	panic("create client implemented in hermes relayer not the commander")
}

func (c commander) CreateConnections(pathName string, homeDir string) []string {
	panic("create connections implemented in hermes relayer not the commander")
}

func (c commander) Flush(pathName, channelID, homeDir string) []string {
	panic("flush implemented in hermes relayer not the commander")
}

func (c commander) ConfigContent(ctx context.Context, cfg ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) ([]byte, error) {
	panic("config content implemented in hermes relayer not the commander")
}

func (c commander) ParseAddKeyOutput(stdout, stderr string) (ibc.Wallet, error) {
	panic("add key implemented in Hermes Relayer")
}

// ParseRestoreKeyOutput extracts the address from the hermes output.
func (c commander) ParseRestoreKeyOutput(stdout, stderr string) string {
	panic("implemented in Hermes Relayer")
}
