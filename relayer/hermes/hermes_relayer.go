package hermes

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/pelletier/go-toml"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/relayer"
	"go.uber.org/zap"
)

const (
	hermes                  = "hermes"
	defaultContainerImage   = "ghcr.io/informalsystems/hermes"
	DefaultContainerVersion = "1.8.2"

	hermesDefaultUidGid = "1000:1000"
	hermesHome          = "/home/hermes"
	hermesConfigPath    = ".hermes/config.toml"
)

var (
	_ ibc.Relayer = &Relayer{}
	// parseRestoreKeyOutputPattern extracts the address from the hermes output.
	// SUCCESS Restored key 'g2-2' (cosmos1czklnpzwaq3hfxtv6ne4vas2p9m5q3p3fgkz8e) on chain g2-2
	parseRestoreKeyOutputPattern = regexp.MustCompile(`\((.*)\)`)
)

// Relayer is the ibc.Relayer implementation for hermes.
type Relayer struct {
	*relayer.DockerRelayer

	// lock protects the relayer's state
	lock         sync.RWMutex
	paths        map[string]*pathConfiguration
	chainConfigs []ChainConfig
	chainLocks   map[string]*sync.Mutex
}

// ChainConfig holds all values required to write an entry in the "chains" section in the hermes config file.
type ChainConfig struct {
	cfg                        ibc.ChainConfig
	keyName, rpcAddr, grpcAddr string
}

// pathConfiguration represents the concept of a "path" which is implemented at the interchain test level rather
// than the hermes level.
type pathConfiguration struct {
	chainA, chainB pathChainConfig
}

// pathChainConfig holds all values that will be required when interacting with a path.
type pathChainConfig struct {
	chainID      string
	clientID     string
	connectionID string
	portID       string
}

// NewHermesRelayer returns a new hermes relayer.
func NewHermesRelayer(log *zap.Logger, testName string, cli *client.Client, networkID string, options ...relayer.RelayerOpt) *Relayer {
	c := commander{log: log}

	options = append(options, relayer.HomeDir(hermesHome))
	dr, err := relayer.NewDockerRelayer(context.TODO(), log, testName, cli, networkID, c, options...)
	if err != nil {
		panic(err)
	}
	c.extraStartFlags = dr.GetExtraStartupFlags()

	return &Relayer{
		DockerRelayer: dr,
		chainLocks:    map[string]*sync.Mutex{},
	}
}

// AddChainConfiguration is called once per chain configuration, which means that in the case of hermes, the single
// config file is overwritten with a new entry each time this function is called.
func (r *Relayer) AddChainConfiguration(ctx context.Context, rep ibc.RelayerExecReporter, chainConfig ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) error {
	configContent, err := r.configContent(chainConfig, keyName, rpcAddr, grpcAddr)
	if err != nil {
		return fmt.Errorf("failed to generate config content: %w", err)
	}

	if err := r.WriteFileToHomeDir(ctx, hermesConfigPath, configContent); err != nil {
		return fmt.Errorf("failed to write hermes config: %w", err)
	}

	if err := r.validateConfig(ctx, rep); err != nil {
		return err
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	r.chainLocks[chainConfig.ChainID] = &sync.Mutex{}
	return nil
}

// LinkPath performs the operations that happen when a path is linked. This includes creating clients, creating connections
// and establishing a channel. This happens across multiple operations rather than a single link path cli command.
func (r *Relayer) LinkPath(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, channelOpts ibc.CreateChannelOptions, clientOpts ibc.CreateClientOptions) error {
	r.lock.RLock()
	_, ok := r.paths[pathName]
	r.lock.RUnlock()
	if !ok {
		return fmt.Errorf("path %s not found", pathName)
	}

	if err := r.CreateClients(ctx, rep, pathName, clientOpts); err != nil {
		return err
	}

	if err := r.CreateConnections(ctx, rep, pathName); err != nil {
		return err
	}

	if err := r.CreateChannel(ctx, rep, pathName, channelOpts); err != nil {
		return err
	}

	return nil
}

func (r *Relayer) CreateChannel(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, opts ibc.CreateChannelOptions) error {
	pathConfig, unlock, err := r.getAndLockPath(pathName)
	if err != nil {
		return err
	}
	defer unlock()

	cmd := []string{hermes, "--json", "create", "channel", "--order", opts.Order.String(), "--a-chain", pathConfig.chainA.chainID, "--a-port", opts.SourcePortName, "--b-port", opts.DestPortName, "--a-connection", pathConfig.chainA.connectionID}
	if opts.Version != "" {
		cmd = append(cmd, "--channel-version", opts.Version)
	}
	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return res.Err
	}
	pathConfig.chainA.portID = opts.SourcePortName
	pathConfig.chainB.portID = opts.DestPortName
	return nil
}

func (r *Relayer) CreateConnections(ctx context.Context, rep ibc.RelayerExecReporter, pathName string) error {
	pathConfig, unlock, err := r.getAndLockPath(pathName)
	if err != nil {
		return err
	}
	defer unlock()

	cmd := []string{hermes, "--json", "create", "connection", "--a-chain", pathConfig.chainA.chainID, "--a-client", pathConfig.chainA.clientID, "--b-client", pathConfig.chainB.clientID}

	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return res.Err
	}

	chainAConnectionID, chainBConnectionID, err := GetConnectionIDsFromStdout(res.Stdout)
	if err != nil {
		return err
	}
	pathConfig.chainA.connectionID = chainAConnectionID
	pathConfig.chainB.connectionID = chainBConnectionID
	return res.Err
}

func (r *Relayer) UpdateClients(ctx context.Context, rep ibc.RelayerExecReporter, pathName string) error {
	pathConfig, unlock, err := r.getAndLockPath(pathName)
	if err != nil {
		return err
	}
	defer unlock()

	updateChainACmd := []string{hermes, "--json", "update", "client", "--host-chain", pathConfig.chainA.chainID, "--client", pathConfig.chainA.clientID}
	res := r.Exec(ctx, rep, updateChainACmd, nil)
	if res.Err != nil {
		return res.Err
	}
	updateChainBCmd := []string{hermes, "--json", "update", "client", "--host-chain", pathConfig.chainB.chainID, "--client", pathConfig.chainB.clientID}
	return r.Exec(ctx, rep, updateChainBCmd, nil).Err
}

// CreateClients creates clients on both chains.
// Note: in the go relayer this can be done with a single command using the path reference,
// however in Hermes this needs to be done as two separate commands.
func (r *Relayer) CreateClients(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, opts ibc.CreateClientOptions) error {
	pathConfig, unlock, err := r.getAndLockPath(pathName)
	if err != nil {
		return err
	}
	defer unlock()

	chainACreateClientCmd := []string{hermes, "--json", "create", "client", "--host-chain", pathConfig.chainA.chainID, "--reference-chain", pathConfig.chainB.chainID}
	if opts.TrustingPeriod != "" {
		chainACreateClientCmd = append(chainACreateClientCmd, "--trusting-period", opts.TrustingPeriod)
	}
	if opts.MaxClockDrift != "" {
		chainACreateClientCmd = append(chainACreateClientCmd, "--clock-drift", opts.MaxClockDrift)
	}
	res := r.Exec(ctx, rep, chainACreateClientCmd, nil)
	if res.Err != nil {
		return res.Err
	}

	chainAClientId, err := GetClientIdFromStdout(res.Stdout)
	if err != nil {
		return err
	}
	pathConfig.chainA.clientID = chainAClientId

	chainBCreateClientCmd := []string{hermes, "--json", "create", "client", "--host-chain", pathConfig.chainB.chainID, "--reference-chain", pathConfig.chainA.chainID}
	if opts.TrustingPeriod != "" {
		chainBCreateClientCmd = append(chainBCreateClientCmd, "--trusting-period", opts.TrustingPeriod)
	}
	if opts.MaxClockDrift != "" {
		chainBCreateClientCmd = append(chainBCreateClientCmd, "--clock-drift", opts.MaxClockDrift)
	}
	res = r.Exec(ctx, rep, chainBCreateClientCmd, nil)
	if res.Err != nil {
		return res.Err
	}

	chainBClientId, err := GetClientIdFromStdout(res.Stdout)
	if err != nil {
		return err
	}
	pathConfig.chainB.clientID = chainBClientId

	return res.Err
}

func (r *Relayer) CreateClient(ctx context.Context, rep ibc.RelayerExecReporter, srcChainID, dstChainID, pathName string, opts ibc.CreateClientOptions) error {
	pathConfig, unlock, err := r.getAndLockPath(pathName)
	if err != nil {
		return err
	}
	defer unlock()

	createClientCmd := []string{hermes, "--json", "create", "client", "--host-chain", srcChainID, "--reference-chain", dstChainID}
	if opts.TrustingPeriod != "" {
		createClientCmd = append(createClientCmd, "--trusting-period", opts.TrustingPeriod)
	}
	if opts.MaxClockDrift != "" {
		createClientCmd = append(createClientCmd, "--clock-drift", opts.MaxClockDrift)
	}
	res := r.Exec(ctx, rep, createClientCmd, nil)
	if res.Err != nil {
		return res.Err
	}

	clientId, err := GetClientIdFromStdout(res.Stdout)
	if err != nil {
		return err
	}

	if pathConfig.chainA.chainID == srcChainID {
		pathConfig.chainA.chainID = clientId
	} else if pathConfig.chainB.chainID == srcChainID {
		pathConfig.chainB.chainID = clientId
	} else {
		return fmt.Errorf("%s not found in path config", srcChainID)
	}

	return res.Err
}

// RestoreKey restores a key from a mnemonic. In hermes, you must provide a file containing the mnemonic. We need
// to copy the contents of the mnemonic into a file on disk and then reference the newly created file.
func (r *Relayer) RestoreKey(ctx context.Context, rep ibc.RelayerExecReporter, cfg ibc.ChainConfig, keyName, mnemonic string) error {
	chainID := cfg.ChainID
	relativeMnemonicFilePath := fmt.Sprintf("%s/mnemonic.txt", chainID)
	if err := r.WriteFileToHomeDir(ctx, relativeMnemonicFilePath, []byte(mnemonic)); err != nil {
		return fmt.Errorf("failed to write mnemonic file: %w", err)
	}

	cmd := []string{hermes, "keys", "add", "--chain", chainID, "--mnemonic-file", fmt.Sprintf("%s/%s", r.HomeDir(), relativeMnemonicFilePath), "--key-name", keyName}

	// Restoring a key should be near-instantaneous, so add a 1-minute timeout
	// to detect if Docker has hung.
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return res.Err
	}

	addrBytes := parseRestoreKeyOutput(string(res.Stdout))
	r.AddWallet(chainID, NewWallet(chainID, addrBytes, mnemonic))
	return nil
}

func (r *Relayer) UpdatePath(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, opts ibc.PathUpdateOptions) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	// the concept of paths doesn't exist in hermes, but update our in-memory paths so we can use them elsewhere
	path, ok := r.paths[pathName]
	if !ok {
		return fmt.Errorf("path %s not found", pathName)
	}
	if opts.SrcChainID != nil {
		path.chainA.chainID = *opts.SrcChainID
	}
	if opts.DstChainID != nil {
		path.chainB.chainID = *opts.DstChainID
	}
	if opts.SrcClientID != nil {
		path.chainA.clientID = *opts.SrcClientID
	}
	if opts.DstClientID != nil {
		path.chainB.clientID = *opts.DstClientID
	}
	if opts.SrcConnID != nil {
		path.chainA.connectionID = *opts.SrcConnID
	}
	if opts.DstConnID != nil {
		path.chainB.connectionID = *opts.DstConnID
	}
	return nil
}

func (r *Relayer) Flush(ctx context.Context, rep ibc.RelayerExecReporter, pathName string, channelID string) error {
	r.lock.RLock()
	path := r.paths[pathName]
	channels, err := r.GetChannels(ctx, rep, path.chainA.chainID)
	if err != nil {
		return err
	}
	var portID string
	for _, ch := range channels {
		if ch.ChannelID == channelID {
			portID = ch.PortID
			break
		}
	}
	if portID == "" {
		return fmt.Errorf("channel %s not found on chain %s", channelID, path.chainA.chainID)
	}
	r.lock.RUnlock()
	cmd := []string{hermes, "clear", "packets", "--chain", path.chainA.chainID, "--channel", channelID, "--port", portID}
	res := r.Exec(ctx, rep, cmd, nil)
	return res.Err
}

// GeneratePath establishes an in memory path representation. The concept does not exist in hermes, so it is handled
// at the interchain test level.
func (r *Relayer) GeneratePath(ctx context.Context, rep ibc.RelayerExecReporter, srcChainID, dstChainID, pathName string) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.paths == nil {
		r.paths = map[string]*pathConfiguration{}
	}
	r.paths[pathName] = &pathConfiguration{
		chainA: pathChainConfig{
			chainID: srcChainID,
		},
		chainB: pathChainConfig{
			chainID: dstChainID,
		},
	}
	return nil
}

// configContent returns the contents of the hermes config file as a byte array. Note: as hermes expects a single file
// rather than multiple config files, we need to maintain a list of chain configs each time they are added to write the
// full correct file update calling Relayer.AddChainConfiguration.
func (r *Relayer) configContent(cfg ibc.ChainConfig, keyName, rpcAddr, grpcAddr string) ([]byte, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.chainConfigs = append(r.chainConfigs, ChainConfig{
		cfg:      cfg,
		keyName:  keyName,
		rpcAddr:  rpcAddr,
		grpcAddr: grpcAddr,
	})
	hermesConfig := NewConfig(r.chainConfigs...)
	bz, err := toml.Marshal(hermesConfig)
	if err != nil {
		return nil, err
	}
	return bz, nil
}

// validateConfig validates the hermes config file. Any errors are propagated to the test.
func (r *Relayer) validateConfig(ctx context.Context, rep ibc.RelayerExecReporter) error {
	cmd := []string{hermes, "--config", fmt.Sprintf("%s/%s", r.HomeDir(), hermesConfigPath), "config", "validate"}
	res := r.Exec(ctx, rep, cmd, nil)
	if res.Err != nil {
		return res.Err
	}
	return nil
}

// extractJsonResult extracts the json result for the hermes query.
func extractJsonResult(stdout []byte) []byte {
	stdoutLines := strings.Split(string(stdout), "\n")
	var jsonOutput string
	for _, line := range stdoutLines {
		if strings.Contains(line, "result") {
			jsonOutput = line
			break
		}
	}
	return []byte(jsonOutput)
}

func (r *Relayer) getAndLockPath(pathName string) (*pathConfiguration, func(), error) {
	// we don't get an RLock here because we could deadlock while trying to get the chain locks
	r.lock.Lock()
	path, ok := r.paths[pathName]
	defer r.lock.Unlock()
	if !ok {
		return nil, nil, fmt.Errorf("path %s not found", pathName)
	}
	chainALock := r.chainLocks[path.chainA.chainID]
	chainBLock := r.chainLocks[path.chainB.chainID]
	chainALock.Lock()
	chainBLock.Lock()
	unlock := func() {
		chainALock.Unlock()
		chainBLock.Unlock()
	}
	return path, unlock, nil
}

// GetClientIdFromStdout extracts the client ID from stdout.
func GetClientIdFromStdout(stdout []byte) (string, error) {
	var clientCreationResult ClientCreationResponse
	if err := json.Unmarshal(extractJsonResult(stdout), &clientCreationResult); err != nil {
		return "", err
	}
	return clientCreationResult.Result.CreateClient.ClientID, nil
}

// GetConnectionIDsFromStdout extracts the connectionIDs on both ends from the stdout.
func GetConnectionIDsFromStdout(stdout []byte) (string, string, error) {
	var connectionResponse ConnectionResponse
	if err := json.Unmarshal(extractJsonResult(stdout), &connectionResponse); err != nil {
		return "", "", err
	}
	return connectionResponse.Result.ASide.ConnectionID, connectionResponse.Result.BSide.ConnectionID, nil
}

// GetChannelIDsFromStdout extracts the channelIDs on both ends from stdout.
func GetChannelIDsFromStdout(stdout []byte) (string, string, error) {
	var channelResponse ChannelCreationResponse
	if err := json.Unmarshal(extractJsonResult(stdout), &channelResponse); err != nil {
		return "", "", err
	}
	return channelResponse.Result.ASide.ChannelID, channelResponse.Result.BSide.ChannelID, nil
}

// parseRestoreKeyOutput extracts the address from the hermes output.
func parseRestoreKeyOutput(stdout string) string {
	fullMatchIdx, addressGroupIdx := 0, 1
	return parseRestoreKeyOutputPattern.FindAllStringSubmatch(stdout, -1)[fullMatchIdx][addressGroupIdx]
}
