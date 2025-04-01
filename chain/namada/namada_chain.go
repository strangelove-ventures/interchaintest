package namada

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	stdmath "math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	dockerimage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"cosmossdk.io/math"

	cometbft "github.com/cometbft/cometbft/abci/types"

	"github.com/strangelove-ventures/interchaintest/v8/chain/internal/tendermint"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

const (
	NamAddress    = "tnam1qxgfw7myv4dh0qna4hq0xdg6lx77fzl7dcem8h7e"
	NamTokenDenom = int64(6)
	MaspAddress   = "tnam1pcqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqzmefah"
	gasPayerAlias = "gas-payer"
)

type NamadaChain struct {
	log           *zap.Logger
	testName      string
	cfg           ibc.ChainConfig
	NumValidators int
	numFullNodes  int
	Validators    NamadaNodes
	FullNodes     NamadaNodes

	isRunning bool
}

// New instance of NamadaChain.
func NewNamadaChain(testName string, chainConfig ibc.ChainConfig, numValidators int, numFullNodes int, log *zap.Logger) *NamadaChain {
	return &NamadaChain{
		log:           log,
		testName:      testName,
		cfg:           chainConfig,
		NumValidators: numValidators,
		numFullNodes:  numFullNodes,
	}
}

// Chain config.
func (c *NamadaChain) Config() ibc.ChainConfig {
	return c.cfg
}

// Initialize the chain.
func (c *NamadaChain) Initialize(ctx context.Context, testName string, cli *client.Client, networkID string) error {
	chainCfg := c.Config()
	for _, image := range chainCfg.Images {
		rc, err := cli.ImagePull(
			ctx,
			image.Repository+":"+image.Version,
			dockerimage.PullOptions{
				Platform: "amd64",
			})
		if err != nil {
			c.log.Error("Failed to pull image",
				zap.Error(err),
				zap.String("repository", image.Repository),
				zap.String("tag", image.Version),
			)
		} else {
			_, _ = io.Copy(io.Discard, rc)
			_ = rc.Close()
		}
	}

	for i := 0; i < c.NumValidators; i++ {
		nn, err := NewNamadaNode(ctx, c.log, c, i, true, testName, cli, networkID, chainCfg.Images[0])
		if err != nil {
			return err
		}
		c.Validators = append(c.Validators, nn)
	}

	for i := 0; i < c.numFullNodes; i++ {
		nn, err := NewNamadaNode(ctx, c.log, c, i, false, testName, cli, networkID, chainCfg.Images[0])
		if err != nil {
			return err
		}
		c.FullNodes = append(c.FullNodes, nn)
	}

	tempBaseDir, err := os.MkdirTemp("", "namada")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempBaseDir)

	c.log.Debug("Temporary base directory",
		zap.String("path", tempBaseDir),
	)
	c.isRunning = false

	return nil
}

// Start to set up.
func (c *NamadaChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	err := c.downloadTemplates(ctx)
	if err != nil {
		return fmt.Errorf("downloading template files failed: %v", err)
	}
	err = c.downloadWasms(ctx)
	if err != nil {
		return fmt.Errorf("downloading wasm files failed: %v", err)
	}

	err = c.setValidators(ctx)
	if err != nil {
		return fmt.Errorf("setting validators failed: %v", err)
	}

	err = c.initAccounts(ctx, additionalGenesisWallets...)
	if err != nil {
		return fmt.Errorf("initializing accounts failed: %v", err)
	}

	err = c.updateParameters(ctx)
	if err != nil {
		return fmt.Errorf("updating parameters failed: %v", err)
	}

	err = c.initNetwork(ctx)
	if err != nil {
		return fmt.Errorf("init-network failed: %v", err)
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for _, n := range c.Validators {
		eg.Go(func() error {
			if err := c.copyGenesisFiles(egCtx, n); err != nil {
				return err
			}
			return n.CreateContainer(egCtx)
		})
	}

	for _, n := range c.FullNodes {
		eg.Go(func() error {
			if err := c.copyGenesisFiles(egCtx, n); err != nil {
				return err
			}
			return n.CreateContainer(egCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	eg, egCtx = errgroup.WithContext(ctx)
	for _, n := range c.Validators {
		eg.Go(func() error {
			return n.StartContainer(egCtx)
		})
	}

	for _, n := range c.FullNodes {
		eg.Go(func() error {
			return n.StartContainer(egCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	if err := testutil.WaitForBlocks(ctx, 2, c.getNode()); err != nil {
		return err
	}

	c.isRunning = true

	return nil
}

func (c *NamadaChain) getNode() *NamadaNode {
	return c.Validators[0]
}

// Execute a command.
func (c *NamadaChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	return c.getNode().Exec(ctx, cmd, env)
}

// Exports the chain state at the specific height.
func (c *NamadaChain) ExportState(ctx context.Context, height int64) (string, error) {
	panic("implement me")
}

// Get the RPC address.
func (c *NamadaChain) GetRPCAddress() string {
	return fmt.Sprintf("http://%s:26657", c.getNode().HostName())
}

// Get the gRPC address. This isn't used for Namada.
func (c *NamadaChain) GetGRPCAddress() string {
	// Returns a dummy address because Namada doesn't support gRPC
	return fmt.Sprintf("http://%s:9090", c.getNode().HostName())
}

// Get the host RPC address.
func (c *NamadaChain) GetHostRPCAddress() string {
	return "http://" + c.getNode().hostRPCPort
}

// Get the host peer address.
func (c *NamadaChain) GetHostPeerAddress() string {
	return c.getNode().hostP2PPort
}

// Get the host gRPC address.
func (c *NamadaChain) GetHostGRPCAddress() string {
	panic("No gRPC address for Namada")
}

// Get Namada home directory.
func (c *NamadaChain) HomeDir() string {
	return c.getNode().HomeDir()
}

// Create a test key.
func (c *NamadaChain) CreateKey(ctx context.Context, keyName string) error {
	var err error
	cmd := []string{
		c.cfg.Bin,
		"wallet",
		"--base-dir",
		c.HomeDir(),
		"gen",
		"--alias",
		keyName,
		"--unsafe-dont-encrypt",
	}
	if !c.isRunning {
		cmd = append(cmd, "--pre-genesis")
	}
	_, _, err = c.Exec(ctx, cmd, c.Config().Env)

	return err
}

// Recovery a test key.
func (c *NamadaChain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	cmd := []string{
		"echo",
		mnemonic,
		"|",
		c.cfg.Bin,
		"wallet",
		"--base-dir",
		c.HomeDir(),
		"derive",
		"--alias",
		keyName,
		"--unsafe-dont-encrypt",
	}
	_, _, err := c.Exec(ctx, cmd, c.Config().Env)

	return err
}

// Get the Namada address.
func (c *NamadaChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	cmd := []string{
		c.cfg.Bin,
		"wallet",
		"--base-dir",
		c.HomeDir(),
		"find",
		"--alias",
		keyName,
	}
	if !c.isRunning {
		cmd = append(cmd, "--pre-genesis")
	}
	output, _, err := c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return nil, fmt.Errorf("getting an address failed with name %q: %w", keyName, err)
	}
	outputStr := string(output)
	re := regexp.MustCompile(`(tnam|znam|zvknam)[0-9a-z]+`)
	address := re.FindString(outputStr)

	if address == "" {
		return nil, fmt.Errorf("no address with name %q: %w", keyName, err)
	}

	return []byte(address), nil
}

// Get the key alias.
func (c *NamadaChain) getAlias(ctx context.Context, address string) (string, error) {
	cmd := []string{
		c.cfg.Bin,
		"wallet",
		"--base-dir",
		c.HomeDir(),
		"find",
		"--address",
		address,
	}
	if !c.isRunning {
		cmd = append(cmd, "--pre-genesis")
	}
	output, _, err := c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return "", fmt.Errorf("getting the alias failed with address %s: %w", address, err)
	}
	outputStr := string(output)
	re := regexp.MustCompile(`Found alias (\S+)`)
	matches := re.FindStringSubmatch(outputStr)
	if len(matches) < 2 {
		return "", fmt.Errorf("no alias found: %s", outputStr)
	}
	alias := matches[1]

	return alias, nil
}

// Send funds to a wallet from a user account.
func (c *NamadaChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	var transferCmd string
	if strings.HasPrefix(amount.Address, "znam") {
		transferCmd = "shield"
	} else {
		transferCmd = "transparent-transfer"
	}
	cmd := []string{
		c.cfg.Bin,
		"client",
		"--base-dir",
		c.HomeDir(),
		transferCmd,
		"--source",
		keyName,
		"--target",
		amount.Address,
		"--token",
		amount.Denom,
		"--amount",
		amount.Amount.String(),
		"--node",
		c.GetRPCAddress(),
	}
	_, _, err := c.Exec(ctx, cmd, c.Config().Env)

	return err
}

// Send funds to a wallet from a user account with a memo.
func (c *NamadaChain) SendFundsWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	var transferCmd string
	if strings.HasPrefix(amount.Address, "znam") {
		transferCmd = "shield"
	} else {
		transferCmd = "transparent-transfer"
	}
	cmd := []string{
		c.cfg.Bin,
		"client",
		"--base-dir",
		c.HomeDir(),
		transferCmd,
		"--source",
		keyName,
		"--target",
		amount.Address,
		"--token",
		amount.Denom,
		"--amount",
		amount.Amount.String(),
		"--memo",
		note,
		"--node",
		c.GetRPCAddress(),
	}
	_, _, err := c.Exec(ctx, cmd, c.Config().Env)

	return note, err
}

// Send on IBC transfer.
func (c *NamadaChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	cmd := []string{
		c.cfg.Bin,
		"client",
		"--base-dir",
		c.HomeDir(),
		"ibc-transfer",
		"--source",
		keyName,
		"--receiver",
		amount.Address,
		"--token",
		amount.Denom,
		"--amount",
		amount.Amount.String(),
		"--channel-id",
		channelID,
		"--node",
		c.GetRPCAddress(),
	}

	if c.Config().Gas != "" {
		_, err := strconv.ParseInt(c.Config().Gas, 10, 64)
		if err != nil {
			return ibc.Tx{}, fmt.Errorf("invalid gas limit: %s", c.Config().Gas)
		}
		cmd = append(cmd, "--gas-limit", c.Config().Gas)
	}

	if options.Port != "" {
		cmd = append(cmd, "--port-id", options.Port)
	}

	if options.Memo != "" {
		cmd = append(cmd, "--ibc-memo", options.Memo)
	}

	if options.Timeout != nil {
		if options.Timeout.NanoSeconds > 0 {
			timestamp := time.Unix(0, int64(options.Timeout.NanoSeconds))
			currentTime := time.Now()
			if currentTime.After(timestamp) {
				return ibc.Tx{}, fmt.Errorf("invalid timeout timestamp: %d", options.Timeout.NanoSeconds)
			}
			offset := int64(timestamp.Sub(currentTime).Seconds())
			cmd = append(cmd, "--timeout-sec-offset", strconv.FormatInt(offset, 10))
		}

		if options.Timeout.Height > 0 {
			cmd = append(cmd, "--timeout-height", strconv.FormatInt(options.Timeout.Height, 10))
		}
	}

	if strings.HasPrefix(keyName, "shielded") {
		cmd = append(cmd, "--gas-payer", gasPayerAlias)
	}

	output, _, err := c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return ibc.Tx{}, fmt.Errorf("the transaction failed: %s, %v", output, err)
	}
	outputStr := string(output)
	c.log.Log(zap.InfoLevel, outputStr)

	re := regexp.MustCompile(`Transaction hash: ([0-9A-F]+)`)
	matches := re.FindStringSubmatch(outputStr)
	var txHash string
	if len(matches) > 1 {
		txHash = matches[1]
	} else {
		return ibc.Tx{}, fmt.Errorf("the transaction failed: %s", outputStr)
	}

	re = regexp.MustCompile(`Transaction batch ([0-9A-F]+) was applied at height (\d+), consuming (\d+) gas units`)
	matchesAll := re.FindAllStringSubmatch(outputStr, -1)
	if len(matches) == 0 {
		return ibc.Tx{}, fmt.Errorf("the transaction failed: %s", outputStr)
	}

	var height int64
	var gas int64
	for _, match := range matchesAll {
		if len(match) == 4 {
			// it is ok to overwrite them of the last transaction
			height, _ = strconv.ParseInt(match[2], 10, 64)
			gas, _ = strconv.ParseInt(match[3], 10, 64)
		}
	}

	tx := ibc.Tx{
		TxHash:   txHash,
		Height:   height,
		GasSpent: gas,
	}

	results, err := c.getNode().Client.BlockResults(ctx, &height)
	if err != nil {
		return ibc.Tx{}, fmt.Errorf("checking the events failed: %v", err)
	}
	const evType = "send_packet"
	tendermintEvents := results.EndBlockEvents
	var events []cometbft.Event
	for _, event := range tendermintEvents {
		if event.Type != evType {
			continue
		}
		jsonEvent, err := json.Marshal(event)
		if err != nil {
			return ibc.Tx{}, fmt.Errorf("converting an events failed: %v", err)
		}
		var event cometbft.Event
		err = json.Unmarshal(jsonEvent, &event)
		if err != nil {
			return ibc.Tx{}, fmt.Errorf("converting an event failed: %v", err)
		}
		events = append(events, event)
	}

	var (
		seq, _           = tendermint.AttributeValue(events, evType, "packet_sequence")
		srcPort, _       = tendermint.AttributeValue(events, evType, "packet_src_port")
		srcChan, _       = tendermint.AttributeValue(events, evType, "packet_src_channel")
		dstPort, _       = tendermint.AttributeValue(events, evType, "packet_dst_port")
		dstChan, _       = tendermint.AttributeValue(events, evType, "packet_dst_channel")
		timeoutHeight, _ = tendermint.AttributeValue(events, evType, "packet_timeout_height")
		timeoutTS, _     = tendermint.AttributeValue(events, evType, "packet_timeout_timestamp")
		dataHex, _       = tendermint.AttributeValue(events, evType, "packet_data_hex")
	)
	tx.Packet.SourcePort = srcPort
	tx.Packet.SourceChannel = srcChan
	tx.Packet.DestPort = dstPort
	tx.Packet.DestChannel = dstChan
	tx.Packet.TimeoutHeight = timeoutHeight

	data, err := hex.DecodeString(dataHex)
	if err != nil {
		return tx, fmt.Errorf("malformed data hex %s: %w", dataHex, err)
	}
	tx.Packet.Data = data

	seqNum, err := strconv.ParseUint(seq, 10, 64)
	if err != nil {
		return tx, fmt.Errorf("invalid packet sequence from events %s: %w", seq, err)
	}
	tx.Packet.Sequence = seqNum

	timeoutNano, err := strconv.ParseUint(timeoutTS, 10, 64)
	if err != nil {
		return tx, fmt.Errorf("invalid packet timestamp timeout %s: %w", timeoutTS, err)
	}
	tx.Packet.TimeoutTimestamp = ibc.Nanoseconds(timeoutNano)

	return tx, err
}

// Shielded transfer (shielded account to shielded account) on Namada.
func (c *NamadaChain) ShieldedTransfer(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	cmd := []string{
		c.cfg.Bin,
		"client",
		"--base-dir",
		c.HomeDir(),
		"transfer",
		"--source",
		keyName,
		"--target",
		amount.Address,
		"--token",
		amount.Denom,
		"--amount",
		amount.Amount.String(),
		"--gas-payer",
		gasPayerAlias,
		"--node",
		c.GetRPCAddress(),
	}
	_, _, err := c.Exec(ctx, cmd, c.Config().Env)

	return err
}

// Generate an IBC shielding transfer for the following shielding transfer via IBC.
func (c *NamadaChain) GenIbcShieldingTransfer(ctx context.Context, channelID string, amount ibc.WalletAmount, options ibc.TransferOptions) (string, error) {
	var portID string
	if options.Port == "" {
		portID = "transfer"
	} else {
		portID = options.Port
	}

	cmd := []string{
		c.cfg.Bin,
		"client",
		"--base-dir",
		c.HomeDir(),
		"ibc-gen-shielding",
		"--output-folder-path",
		c.HomeDir(),
		"--target",
		amount.Address,
		"--token",
		amount.Denom,
		"--amount",
		amount.Amount.String(),
		"--port-id",
		portID,
		"--channel-id",
		channelID,
		"--node",
		c.GetRPCAddress(),
	}
	output, _, err := c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return "", fmt.Errorf("failed to generate the IBC shielding transfer: %v", err)
	}
	outputStr := string(output)

	re := regexp.MustCompile(`Output IBC shielding transfer for ([^\s]+) to (.+)`)
	matches := re.FindStringSubmatch(outputStr)
	var path string
	if len(matches) > 2 {
		path = matches[2]
	} else {
		return "", fmt.Errorf("failed to get the file path of the IBC shielding transfer")
	}
	relPath, _ := filepath.Rel(c.HomeDir(), path)
	shieldingTransfer, err := c.getNode().ReadFile(ctx, relPath)
	if err != nil {
		return "", fmt.Errorf("failed to read the IBC shielding transfer file: %v", err)
	}

	return string(shieldingTransfer), nil
}

// Get the current block height.
func (c *NamadaChain) Height(ctx context.Context) (int64, error) {
	return c.getNode().Height(ctx)
}

// Get the balance with the key alias, not the address.
func (c *NamadaChain) GetBalance(ctx context.Context, keyName string, denom string) (math.Int, error) {
	if strings.HasPrefix(keyName, "shielded") {
		cmd := []string{
			c.cfg.Bin,
			"client",
			"--base-dir",
			c.HomeDir(),
			"shielded-sync",
			"--viewing-keys",
			keyName,
			"--node",
			c.GetRPCAddress(),
		}
		output, _, err := c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return math.NewInt(0), fmt.Errorf("shielded-sync failed: error %s, output %s", err, output)
		}
	}

	cmd := []string{
		c.cfg.Bin,
		"client",
		"--base-dir",
		c.HomeDir(),
		"balance",
		"--token",
		denom,
		"--owner",
		keyName,
		"--node",
		c.GetRPCAddress(),
	}
	output, _, err := c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return math.NewInt(0), fmt.Errorf("getting the balance failed: error %s, output %s", err, output)
	}
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	// Parse the balance from the output like `nam: 1000.000000`
	re := regexp.MustCompile(`:\s*(\d+(\.\d+)?)$`)

	ret := math.NewInt(0)
	for _, line := range lines {
		if strings.Contains(line, "Last committed masp epoch") {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			amount, err := strconv.ParseFloat(matches[1], 64)
			if err != nil {
				return math.NewInt(0), fmt.Errorf("parsing the amount failed: %s", outputStr)
			}
			var multiplier float64
			if denom == c.Config().Denom {
				multiplier = stdmath.Pow(10, float64(*c.Config().CoinDecimals))
			} else {
				// IBC token denom is always zero
				multiplier = 1.0
			}
			// the result should be an integer
			ret = math.NewInt(int64(amount * multiplier))
		}
	}

	return ret, err
}

// Get the gas fees.
func (c *NamadaChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	panic("implement me")
}

// All acks at the height.
func (c *NamadaChain) Acknowledgements(ctx context.Context, height int64) ([]ibc.PacketAcknowledgement, error) {
	panic("implement me")
}

// All timeouts at the height.
func (c *NamadaChain) Timeouts(ctx context.Context, height int64) ([]ibc.PacketTimeout, error) {
	panic("implement me")
}

// Build a Namada wallet. Generates a spending key when the keyName prefixed with "shielded".
func (c *NamadaChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		if err := c.RecoverKey(ctx, keyName, mnemonic); err != nil {
			return nil, fmt.Errorf("failed to recover key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}

		addrBytes, err := c.GetAddress(ctx, keyName)
		if err != nil {
			return nil, fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, c.cfg.Name, err)
		}

		return NewWallet(keyName, addrBytes, mnemonic, c.cfg), nil
	}

	if !c.isRunning {
		return c.createGenesisKey(ctx, keyName)
	} else {
		return c.createKeyAndMnemonic(ctx, keyName, strings.HasPrefix(keyName, "shielded"))
	}
}

// Build a Namada wallet for a relayer.
func (c *NamadaChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	return c.createKeyAndMnemonic(ctx, keyName, false)
}

// Create an established account key for genesis.
func (c *NamadaChain) createGenesisKey(ctx context.Context, keyName string) (ibc.Wallet, error) {
	alias := fmt.Sprintf("%s-key", keyName)
	_, err := c.createKeyAndMnemonic(ctx, alias, false)
	if err != nil {
		return &NamadaWallet{}, err
	}

	transactionPath := filepath.Join(c.HomeDir(), fmt.Sprintf("established-account-tx-%s.toml", keyName))
	address, err := c.initGenesisEstablishedAccount(ctx, alias, transactionPath)
	if err != nil {
		return &NamadaWallet{}, err
	}

	if err := c.addAddress(ctx, keyName, address); err != nil {
		return &NamadaWallet{}, err
	}

	return NewWallet(keyName, []byte(address), "", c.cfg), nil
}

func (c *NamadaChain) createKeyAndMnemonic(ctx context.Context, keyName string, isShielded bool) (ibc.Wallet, error) {
	cmd := []string{
		c.cfg.Bin,
		"wallet",
		"--base-dir",
		c.HomeDir(),
		"gen",
		"--alias",
		keyName,
		"--unsafe-dont-encrypt",
	}
	if isShielded && !c.isRunning {
		return nil, fmt.Errorf("generating a shielded account in pre-genesis is not allowed in this test")
	}
	if isShielded {
		cmd = append(cmd, "--shielded")
	}
	if !c.isRunning {
		cmd = append(cmd, "--pre-genesis")
	}
	output, _, err := c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return nil, fmt.Errorf("failed to generate an account for key %q on chain %s: %w", keyName, c.cfg.Name, err)
	}
	outputStr := string(output)
	re := regexp.MustCompile(`[a-z]+(?:\s+[a-z]+){23}`)
	mnemonic := re.FindString(outputStr)

	addrBytes, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, c.cfg.Name, err)
	}

	wallet := NewWallet(keyName, addrBytes, mnemonic, c.Config())

	// Generate a payment address
	if isShielded {
		cmd = []string{
			c.cfg.Bin,
			"wallet",
			"--base-dir",
			c.HomeDir(),
			"gen-payment-addr",
			"--alias",
			wallet.PaymentAddressKeyName(),
			"--key",
			keyName,
		}
		if !c.isRunning {
			cmd = append(cmd, "--pre-genesis")
		}
		_, _, err := c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return nil, fmt.Errorf("failed to generate a payment address for key %q on chain %s: %w", keyName, c.Config().Name, err)
		}

		addrBytes, err := c.GetAddress(ctx, wallet.PaymentAddressKeyName())
		if err != nil {
			return nil, fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
		// replace the address with the payment address
		wallet = NewWallet(keyName, addrBytes, mnemonic, c.Config())
	}

	return wallet, nil
}

func (c *NamadaChain) addAddress(ctx context.Context, keyName, address string) error {
	cmd := []string{
		c.cfg.Bin,
		"wallet",
		"--base-dir",
		c.HomeDir(),
		"add",
		"--alias",
		keyName,
		"--value",
		address,
	}
	if !c.isRunning {
		cmd = append(cmd, "--pre-genesis")
	}
	_, _, err := c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return fmt.Errorf("address couldn't be added: %v", err)
	}

	return nil
}

func (c *NamadaChain) downloadTemplates(ctx context.Context) error {
	baseURL := fmt.Sprintf("https://raw.githubusercontent.com/anoma/namada/%s/genesis/localnet", c.Config().Images[0].Version)
	files := []string{
		"parameters.toml",
		"tokens.toml",
		"validity-predicates.toml",
	}
	destDir := "templates"

	for _, file := range files {
		url := fmt.Sprintf("%s/%s", baseURL, file)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("failed to download the file %s: %w", file, err)
		}
		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			return fmt.Errorf("failed to download the file %s: %w", file, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to download the file %s: %d", file, resp.StatusCode)
		}

		var buf bytes.Buffer
		if _, err := io.Copy(&buf, resp.Body); err != nil {
			return fmt.Errorf("failed to read the file: %w", err)
		}
		err = c.getNode().writeFile(ctx, filepath.Join(destDir, file), buf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to write the file %s: %w", file, err)
		}
	}

	return nil
}

func (c *NamadaChain) downloadWasms(ctx context.Context) error {
	url := fmt.Sprintf("https://github.com/anoma/namada/releases/download/%s/namada-%s-Linux-x86_64.tar.gz", c.Config().Images[0].Version, c.Config().Images[0].Version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to download the release file: %w", err)
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return fmt.Errorf("failed to download the release file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download the release file: %d", resp.StatusCode)
	}
	filePath := "release.tar.gz"
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to open the release file: %w", err)
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write the release file: %w", err)
	}

	file, err = os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open the release file: %w", err)
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	destDir := "wasm"
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar file: %w", err)
		}

		if header.Typeflag == tar.TypeReg {
			if strings.HasSuffix(header.Name, ".wasm") || strings.HasSuffix(header.Name, ".json") {
				var buf bytes.Buffer
				limitedReader := io.LimitReader(tr, 10*1024*1024)
				if _, err := io.Copy(&buf, limitedReader); err != nil {
					return fmt.Errorf("failed to read the file: %w", err)
				}
				fileName := filepath.Base(header.Name)
				err = c.getNode().writeFile(ctx, filepath.Join(destDir, fileName), buf.Bytes())
				if err != nil {
					return fmt.Errorf("failed to write the file: %w", err)
				}
			}
		}
	}

	err = os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("failed to delete the release file: %v", err)
	}

	return nil
}

func (c *NamadaChain) setValidators(ctx context.Context) error {
	transactionPath := filepath.Join(c.HomeDir(), "transactions.toml")
	destTransactionsPath := filepath.Join(c.HomeDir(), "templates", "transactions.toml")
	cmd := []string{
		"touch",
		destTransactionsPath,
	}
	_, _, err := c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return fmt.Errorf("making transactions.toml failed: %v", err)
	}

	for i := 0; i < c.NumValidators; i++ {
		alias := fmt.Sprintf("validator-%d-balance-key", i)
		validatorAlias := fmt.Sprintf("validator-%d", i)

		// Generate a validator key
		cmd := []string{
			c.cfg.Bin,
			"wallet",
			"--base-dir",
			c.HomeDir(),
			"--pre-genesis",
			"gen",
			"--alias",
			alias,
			"--unsafe-dont-encrypt",
		}
		_, _, err := c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("validator key couldn't be generated: %v", err)
		}

		// Initialize an established account of the validator
		validatorAddress, err := c.initGenesisEstablishedAccount(ctx, alias, transactionPath)
		if err != nil {
			return err
		}

		// Add the validator address
		if err := c.addAddress(ctx, validatorAlias, validatorAddress); err != nil {
			return fmt.Errorf("validator address couldn't be added: %v", err)
		}

		netAddress, err := c.Validators[i].netAddress(ctx)
		if err != nil {
			return err
		}

		// Initialize a genesis validator
		cmd = []string{
			c.cfg.Bin,
			"client",
			"--base-dir",
			c.HomeDir(),
			"utils",
			"init-genesis-validator",
			"--alias",
			validatorAlias,
			"--address",
			validatorAddress,
			"--path",
			transactionPath,
			"--net-address",
			netAddress,
			"--commission-rate",
			"0.05",
			"--max-commission-rate-change",
			"0.01",
			"--email",
			"null@null.net",
			"--self-bond-amount",
			"100000",
			"--unsafe-dont-encrypt",
		}
		output, _, err := c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("initializing a genesis validator failed: %v, %s", err, output)
		}

		cmd = []string{
			c.cfg.Bin,
			"client",
			"--base-dir",
			c.HomeDir(),
			"utils",
			"sign-genesis-txs",
			"--alias",
			validatorAlias,
			"--path",
			transactionPath,
			"--output",
			transactionPath,
		}
		_, _, err = c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("signing genesis transactions failed: %v", err)
		}

		cmd = []string{
			"sh",
			"-c",
			fmt.Sprintf(`cat %s >> %s`, transactionPath, destTransactionsPath),
		}
		_, _, err = c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("appending the transaction failed: %v", err)
		}
	}

	return nil
}

func (c *NamadaChain) initAccounts(ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	templateDir := filepath.Join(c.HomeDir(), "templates")
	balancePath := filepath.Join(templateDir, "balances.toml")

	// Initialize balances.toml
	cmd := []string{
		"sh",
		"-c",
		fmt.Sprintf(`echo [token.NAM] > %s`, balancePath),
	}
	_, _, err := c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return fmt.Errorf("initializing balances.toml failed: %v", err)
	}

	// for validators
	for i := 0; i < c.NumValidators; i++ {
		addr, err := c.GetAddress(ctx, fmt.Sprintf("validator-%d", i))
		if err != nil {
			return err
		}
		line := fmt.Sprintf(`%s = "2000000"`, string(addr))
		cmd := []string{
			"sh",
			"-c",
			fmt.Sprintf(`echo '%s' >> %s`, line, balancePath),
		}
		_, _, err = c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("appending the balance to balances.toml failed: %v", err)
		}
	}

	// for a gas payer
	gasPayer, err := c.createKeyAndMnemonic(ctx, gasPayerAlias, false)
	if err != nil {
		return err
	}
	gasPayerAmount := ibc.WalletAmount{
		Address: gasPayer.FormattedAddress(),
		Denom:   c.Config().Denom,
		Amount:  math.NewInt(1000000000),
	}
	additionalGenesisWallets = append(additionalGenesisWallets, gasPayerAmount)

	for _, wallet := range additionalGenesisWallets {
		line := fmt.Sprintf(`%s = "%s"`, wallet.Address, wallet.Amount)
		cmd := []string{
			"sh",
			"-c",
			fmt.Sprintf(`echo '%s' >> %s`, line, balancePath),
		}
		_, _, err = c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("appending the balance to balances.toml failed: %v", err)
		}
		// Add the key balance
		alias, err := c.getAlias(ctx, wallet.Address)
		if err != nil {
			return err
		}
		keyAddress, err := c.GetAddress(ctx, fmt.Sprintf("%s-key", alias))
		if err != nil {
			// skip when the account is implicit
			continue
		}
		line = fmt.Sprintf(`%s = "%s"`, keyAddress, wallet.Amount)
		cmd = []string{
			"sh",
			"-c",
			fmt.Sprintf(`echo '%s' >> %s`, line, balancePath),
		}
		_, _, err = c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("appending the balance to balances.toml failed: %v", err)
		}
	}
	destTransactionsPath := filepath.Join(templateDir, "transactions.toml")
	cmd = []string{
		"sh",
		"-c",
		fmt.Sprintf("find %s -name 'established-account-tx-*.toml' -exec cat {} + >> %s", c.HomeDir(), destTransactionsPath),
	}
	_, _, err = c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return fmt.Errorf("appending establish account tx: %w", err)
	}

	return nil
}

func (c *NamadaChain) initGenesisEstablishedAccount(ctx context.Context, keyName, transactionPath string) (string, error) {
	cmd := []string{
		c.cfg.Bin,
		"client",
		"--base-dir",
		c.HomeDir(),
		"utils",
		"init-genesis-established-account",
		"--aliases",
		keyName,
		"--path",
		transactionPath,
	}
	output, _, err := c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return "", fmt.Errorf("initializing a validator account failed: %v", err)
	}
	outputStr := string(output)
	// Trim ANSI escape sequence
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	outputStr = ansiRegex.ReplaceAllString(outputStr, "")
	re := regexp.MustCompile(`Derived established account address: (\S+)`)
	matches := re.FindStringSubmatch(outputStr)
	if len(matches) < 2 {
		return "", fmt.Errorf("no established account address found: %s", outputStr)
	}
	addr := matches[1]

	return addr, nil
}

func (c *NamadaChain) updateParameters(ctx context.Context) error {
	templateDir := filepath.Join(c.HomeDir(), "templates")
	paramPath := filepath.Join(templateDir, "parameters.toml")

	cmd := []string{
		"sed",
		"-i",
		// for enough trusting period
		"-e",
		"s/epochs_per_year = [0-9_]\\+/epochs_per_year = 365/",
		// delete steward addresses
		"-e",
		"s/\"tnam.*//",
		// IBC mint limit
		"-e",
		"s/default_mint_limit = \"[0-9]\\+\"/default_mint_limit = \"1000000000000\"/",
		// IBC throughput limit
		"-e",
		"s/default_per_epoch_throughput_limit = \"[0-9]\\+\"/default_per_epoch_throughput_limit = \"1000000000000\"/",
		paramPath,
	}
	_, _, err := c.Exec(ctx, cmd, c.Config().Env)
	return err
}

func (c *NamadaChain) initNetwork(ctx context.Context) error {
	templatesDir := filepath.Join(c.HomeDir(), "templates")
	wasmDir := filepath.Join(c.HomeDir(), "wasm")
	checksumsPath := filepath.Join(wasmDir, "checksums.json")
	genesisTime := time.Now().UTC().Format("2006-01-02T15:04:05.000000000-07:00")
	cmd := []string{
		c.cfg.Bin,
		"client",
		"--base-dir",
		c.HomeDir(),
		"utils",
		"init-network",
		"--templates-path",
		templatesDir,
		"--chain-prefix",
		"namada-test",
		"--wasm-checksums-path",
		checksumsPath,
		"--wasm-dir",
		wasmDir,
		"--genesis-time",
		genesisTime,
		"--archive-dir",
		c.HomeDir(),
	}
	output, _, err := c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return fmt.Errorf("init-network failed: %v", err)
	}
	outputStr := string(output)

	re := regexp.MustCompile(`Derived chain ID: (\S+)`)
	matches := re.FindStringSubmatch(outputStr)
	if len(matches) < 2 {
		return fmt.Errorf("no chain ID: %s", outputStr)
	}
	c.cfg.ChainID = matches[1]

	return nil
}

func (c *NamadaChain) copyGenesisFiles(ctx context.Context, n *NamadaNode) error {
	archivePath := fmt.Sprintf("%s.tar.gz", c.Config().ChainID)
	content, err := c.getNode().ReadFile(ctx, archivePath)
	if err != nil {
		return fmt.Errorf("failed to read the archive file: %w", err)
	}

	err = n.writeFile(ctx, archivePath, content)
	if err != nil {
		return fmt.Errorf("failed to write the archive file: %w", err)
	}

	walletPath := filepath.Join("pre-genesis", "wallet.toml")
	content, err = c.getNode().ReadFile(ctx, walletPath)
	if err != nil {
		return fmt.Errorf("failed to read the wallet file: %w", err)
	}
	err = n.writeFile(ctx, "wallet.toml", content)
	if err != nil {
		return fmt.Errorf("failed to write the wallet file: %w", err)
	}

	if n.Validator {
		validatorAlias := fmt.Sprintf("validator-%d", n.Index)
		validatorWalletPath := filepath.Join("pre-genesis", validatorAlias, "validator-wallet.toml")
		content, err = c.getNode().ReadFile(ctx, validatorWalletPath)
		if err != nil {
			return fmt.Errorf("failed to read the validator wallet file: %w", err)
		}
		err = n.writeFile(ctx, validatorWalletPath, content)
		if err != nil {
			return fmt.Errorf("failed to write the validator wallet file: %w", err)
		}
	}

	return nil
}
