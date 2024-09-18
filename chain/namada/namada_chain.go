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
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"cosmossdk.io/math"
	cometbft "github.com/cometbft/cometbft/abci/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v8/chain/internal/tendermint"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
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

	mutex sync.Mutex
}

// New instance of NamadaChain
func NewNamadaChain(testName string, chainConfig ibc.ChainConfig, numValidators int, numFullNodes int, log *zap.Logger) *NamadaChain {
	return &NamadaChain{
		log:           log,
		testName:      testName,
		cfg:           chainConfig,
		NumValidators: numValidators,
		numFullNodes:  numFullNodes,
	}
}

// Chain config
func (c *NamadaChain) Config() ibc.ChainConfig {
	return c.cfg
}

// Initialize the chain
func (c *NamadaChain) Initialize(ctx context.Context, testName string, cli *client.Client, networkID string) error {
	chainCfg := c.Config()
	for _, image := range chainCfg.Images {
		rc, err := cli.ImagePull(
			ctx,
			image.Repository+":"+image.Version,
			types.ImagePullOptions{
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

	fmt.Println("Temporary base directory:", tempBaseDir)
	c.isRunning = false

	return nil
}

// Start to set up
func (c *NamadaChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	err := c.downloadTemplates(ctx)
	if err != nil {
		return fmt.Errorf("Downloading template files failed: %v", err)
	}
	err = c.downloadWasms(ctx)
	if err != nil {
		return fmt.Errorf("Downloading wasm files failed: %v", err)
	}

	err = c.setValidators(ctx)
	if err != nil {
		return fmt.Errorf("Setting validators failed: %v", err)
	}

	err = c.initAccounts(ctx, additionalGenesisWallets...)
	if err != nil {
		return fmt.Errorf("Initializing accounts failed: %v", err)
	}

	err = c.updateParameters(ctx)
	if err != nil {
		return fmt.Errorf("Updating parameters failed: %v", err)
	}

	err = c.initNetwork(ctx)
	if err != nil {
		return fmt.Errorf("init-network failed: %v", err)
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for _, n := range c.Validators {
		n := n

		eg.Go(func() error {
			c.copyGenesisFiles(egCtx, n)
			return n.CreateContainer(egCtx)
		})
	}

	for _, n := range c.FullNodes {
		n := n

		eg.Go(func() error {
			c.copyGenesisFiles(egCtx, n)
			return n.CreateContainer(egCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	eg, egCtx = errgroup.WithContext(ctx)
	for _, n := range c.Validators {
		n := n

		eg.Go(func() error {
			return n.StartContainer(egCtx)
		})
	}

	for _, n := range c.FullNodes {
		n := n

		eg.Go(func() error {
			return n.StartContainer(egCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	if err := testutil.WaitForBlocks(ctx, 2, c.Validators[0]); err != nil {
		return err
	}

	c.isRunning = true

	return nil
}

// Execut a command
func (c *NamadaChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	return c.Validators[0].Exec(ctx, cmd, env)
}

// / Exports the chain state at the specific height
func (c *NamadaChain) ExportState(ctx context.Context, height int64) (string, error) {
	panic("implement me")
}

// RPC address
func (c *NamadaChain) GetRPCAddress() string {
	return fmt.Sprintf("http://%s:26657", c.Validators[0].HostName())
}

// gRPC address
func (c *NamadaChain) GetGRPCAddress() string {
	// Returns a dummy address because Namada doesn't support gRPC
	return fmt.Sprintf("http://%s:9090", c.Validators[0].HostName())
}

// Host RPC
func (c *NamadaChain) GetHostRPCAddress() string {
	return "http://" + c.Validators[0].hostRPCPort
}

// Host peer address
func (c *NamadaChain) GetHostPeerAddress() string {
	return c.Validators[0].hostP2PPort
}

// Host gRPC address
func (c *NamadaChain) GetHostGRPCAddress() string {
	panic("No gRPC address for Namada")
}

// Host Namada home directory
func (c *NamadaChain) HomeDir() string {
	return c.Validators[0].HomeDir()
}

// Create a test key
func (c *NamadaChain) CreateKey(ctx context.Context, keyName string) error {
	var err error
	cmd := []string{
		"namadaw",
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

// Recovery a test key
func (c *NamadaChain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	cmd := []string{
		"echo",
		mnemonic,
		"|",
		"namadaw",
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

// Get the Namada address
func (c *NamadaChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	cmd := []string{
		"namadaw",
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
		return nil, fmt.Errorf("Getting an address failed with name %q: %w", keyName, err)
	}
	outputStr := string(output)
	re := regexp.MustCompile(`tnam[0-9a-z]+`)
	address := re.FindString(outputStr)

	if address == "" {
		return nil, fmt.Errorf("No address with name %q: %w", keyName, err)
	}

	return []byte(address), nil
}

// Send funds to a wallet from a user account
func (c *NamadaChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	cmd := []string{
		"namadac",
		"--base-dir",
		c.HomeDir(),
		"transparent-transfer",
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

// Send funds to a wallet from a user account with a memo
func (c *NamadaChain) SendFundsWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	cmd := []string{
		"namadac",
		"--base-dir",
		c.HomeDir(),
		"transparent-transfer",
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

// Send on IBC transfer
func (c *NamadaChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	cmd := []string{
		"namadac",
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

	if options.Port != "" {
		cmd = append(cmd, "--port-id", options.Port)
	}

	if options.Memo != "" {
		cmd = append(cmd, "--ibc-memo", options.Memo)
	}

	// TODO timeout

	output, _, err := c.Exec(ctx, cmd, c.Config().Env)
	outputStr := string(output)
	fmt.Println("DEBUG:", outputStr)

	re := regexp.MustCompile(`Transaction hash: ([0-9A-F]+)`)
	matches := re.FindStringSubmatch(outputStr)
	var txHash string
	if len(matches) > 1 {
		txHash = matches[1]
	} else {
		return ibc.Tx{}, fmt.Errorf("The transaction failed: %s", outputStr)
	}

	re = regexp.MustCompile(`Transaction ([0-9A-F]+) was successfully applied at height (\d+), consuming (\d+) gas units`)
	matchesAll := re.FindAllStringSubmatch(outputStr, -1)
	if len(matches) == 0 {
		return ibc.Tx{}, fmt.Errorf("The transaction failed: %s", outputStr)
	}

	var txHashes []string
	var height int64
	var gas int64
	for _, match := range matchesAll {
		if len(match) == 4 {
			txHashes = append(txHashes, match[1])
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

	results, err := c.Validators[0].Client.BlockResults(ctx, &height)
	if err != nil {
		return ibc.Tx{}, fmt.Errorf("Checking the events failed: %v", err)
	}
	tendermintEvents := results.EndBlockEvents
	jsonEvents, err := json.Marshal(tendermintEvents)
	if err != nil {
		return ibc.Tx{}, fmt.Errorf("Converting events failed: %v", err)
	}
	var events []cometbft.Event
	json.Unmarshal(jsonEvents, events)

	const evType = "send_packet"
	fmt.Println("DEBUG:", events)
	var (
		seq, _           = tendermint.AttributeValue(events, evType, "packet_sequence")
		srcPort, _       = tendermint.AttributeValue(events, evType, "packet_src_port")
		srcChan, _       = tendermint.AttributeValue(events, evType, "packet_src_channel")
		dstPort, _       = tendermint.AttributeValue(events, evType, "packet_dst_port")
		dstChan, _       = tendermint.AttributeValue(events, evType, "packet_dst_channel")
		timeoutHeight, _ = tendermint.AttributeValue(events, evType, "packet_timeout_height")
		timeoutTs, _     = tendermint.AttributeValue(events, evType, "packet_timeout_timestamp")
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

	timeoutNano, err := strconv.ParseUint(timeoutTs, 10, 64)
	if err != nil {
		return tx, fmt.Errorf("invalid packet timestamp timeout %s: %w", timeoutTs, err)
	}
	tx.Packet.TimeoutTimestamp = ibc.Nanoseconds(timeoutNano)

	return tx, err
}

// Current block height
func (c *NamadaChain) Height(ctx context.Context) (int64, error) {
	return c.Validators[0].Height(ctx)
}

// Get the balance
func (c *NamadaChain) GetBalance(ctx context.Context, address string, denom string) (math.Int, error) {
	cmd := []string{
		"namadac",
		"--base-dir",
		c.HomeDir(),
		"balance",
		"--token",
		denom,
		"--owner",
		address,
		"--node",
		c.GetRPCAddress(),
	}
	output, _, err := c.Exec(ctx, cmd, c.Config().Env)
	if err != nil {
		return math.NewInt(0), fmt.Errorf("GetBalance failed: error %s, output %s", err, output)
	}
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	re := regexp.MustCompile(`:\s*(\d+)$`)

	var ret math.Int
	for _, line := range lines {
		if strings.Contains(line, "Last committed masp epoch") {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			amount, ok := math.NewIntFromString(matches[1])
			if !ok {
				return math.NewInt(0), fmt.Errorf("Parsing the amount failed: %s", outputStr)
			}
			ret = amount
		}
	}

	return ret, err
}

// Get the gas fees
func (c *NamadaChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	panic("implement me")
}

// All acks at the height
func (c *NamadaChain) Acknowledgements(ctx context.Context, height int64) ([]ibc.PacketAcknowledgement, error) {
	panic("implement me")
}

// All timeouts at the height
func (c *NamadaChain) Timeouts(ctx context.Context, height int64) ([]ibc.PacketTimeout, error) {
	panic("implement me")
}

// Namada wallet
// Generates a spending key when the keyName prefixed with "shielded"
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
	} else {
		return c.createKeyAndMnemonic(ctx, keyName, strings.HasPrefix(keyName, "shielded"))
	}
}

// Namada wallet for a relayer
func (c *NamadaChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	return c.createKeyAndMnemonic(ctx, keyName, false)
}

func (c *NamadaChain) createKeyAndMnemonic(ctx context.Context, keyName string, isShielded bool) (ibc.Wallet, error) {
	cmd := []string{
		"namadaw",
		"--base-dir",
		c.HomeDir(),
		"gen",
		"--alias",
		keyName,
		"--unsafe-dont-encrypt",
	}
	if isShielded {
		cmd = append(cmd, "--shielded")
	}
	if !c.isRunning {
		cmd = append(cmd, "--pre-genesis")
	}
	output, _, err := c.Exec(ctx, cmd, c.Config().Env)
	outputStr := string(output)
	re := regexp.MustCompile(`[a-z]+(?:\s+[a-z]+){23}`)
	mnemonic := re.FindString(outputStr)

	addrBytes, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, c.cfg.Name, err)
	}

	return NewWallet(keyName, addrBytes, mnemonic, c.cfg), nil
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
		resp, err := http.Get(url)
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
		err = c.Validators[0].writeFile(ctx, filepath.Join(destDir, file), buf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to write the file %s: %w", file, err)
		}
	}

	return nil
}

func (c *NamadaChain) downloadWasms(ctx context.Context) error {
	// TODO replace when releasing Namada v0.44.0
	//url := fmt.Sprintf("https://github.com/anoma/namada/releases/download/%s/namada-%s-Linux-x86_64.tar.gz", c.Config().Images[0].Version, c.Config().Images[0].Version)
	url := "https://github.com/anoma/namada/releases/download/v0.43.0/namada-v0.43.0-Linux-x86_64.tar.gz"

	resp, err := http.Get(url)
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

		switch header.Typeflag {
		case tar.TypeReg:
			if strings.HasSuffix(header.Name, ".wasm") || strings.HasSuffix(header.Name, ".json") {
				var buf bytes.Buffer
				if _, err := io.Copy(&buf, tr); err != nil {
					return fmt.Errorf("failed to read the file: %w", err)
				}
				fileName := filepath.Base(header.Name)
				err = c.Validators[0].writeFile(ctx, filepath.Join(destDir, fileName), buf.Bytes())
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
		return fmt.Errorf("Making transactions.toml failed: %v", err)
	}

	for i := 0; i < c.NumValidators; i++ {
		alias := fmt.Sprintf("validator-%d-balance-key", i)
		validatorAlias := fmt.Sprintf("validator-%d", i)

		// Generate a validator key
		cmd := []string{
			"namadaw",
			"--base-dir",
			c.HomeDir(),
			"--pre-genesis",
			"gen",
			"--alias",
			alias,
			"--unsafe-dont-encrypt",
		}
		output, _, err := c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("Validator key couldn't be generated: %v", err)
		}

		// Initialize an established account of the validator
		cmd = []string{
			"namadac",
			"--base-dir",
			c.HomeDir(),
			"utils",
			"init-genesis-established-account",
			"--aliases",
			alias,
			"--path",
			transactionPath,
		}
		output, _, err = c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("Initializing a validator account failed: %v", err)
		}
		outputStr := string(output)
		// Trim ANSI escape sequence
		ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
		outputStr = ansiRegex.ReplaceAllString(outputStr, "")
		re := regexp.MustCompile(`Derived established account address: (\S+)`)
		matches := re.FindStringSubmatch(outputStr)
		if len(matches) < 2 {
			return fmt.Errorf("No established account adrress found: %s", outputStr)
		}
		validatorAddress := matches[1]

		// Add the validator address
		cmd = []string{
			"namadaw",
			"--base-dir",
			c.HomeDir(),
			"--pre-genesis",
			"add",
			"--alias",
			validatorAlias,
			"--value",
			validatorAddress,
		}
		output, _, err = c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("Validator address couldn't be added: %v", err)
		}

		// Initialize a genesis validator
		cmd = []string{
			"namadac",
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
			c.Validators[i].netAddress(),
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
		output, _, err = c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("Initializing a genesis validator failed: %v, %s", err, output)
		}

		cmd = []string{
			"namadac",
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
			return fmt.Errorf("Signing genesis transactions failed: %v", err)
		}

		cmd = []string{
			"sh",
			"-c",
			fmt.Sprintf(`cat %s >> %s`, transactionPath, destTransactionsPath),
		}
		_, _, err = c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("Appending the transaction failed: %v", err)
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
		return fmt.Errorf("Initializing balances.toml failed: %v", err)
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
			return fmt.Errorf("Appending the balance to balances.toml failed: %v", err)
		}
	}

	for _, wallet := range additionalGenesisWallets {
		line := fmt.Sprintf(`%s = "%s"`, wallet.Address, wallet.Amount)
		cmd := []string{
			"sh",
			"-c",
			fmt.Sprintf(`echo '%s' >> %s`, line, balancePath),
		}
		_, _, err = c.Exec(ctx, cmd, c.Config().Env)
		if err != nil {
			return fmt.Errorf("Appending the balance to balances.toml failed: %v", err)
		}
	}

	return nil
}

func (c *NamadaChain) updateParameters(ctx context.Context) error {
	templateDir := filepath.Join(c.HomeDir(), "templates")
	paramPath := filepath.Join(templateDir, "parameters.toml")

	cmd := []string{
		"sed",
		"-i",
		// for enough trusting period
		"-e",
		"s/epochs_per_year = [0-9_]\\+/epochs_per_year = 31_536/",
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
	cmd := []string{
		"namadac",
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
		"2023-08-30T00:00:00.000000000+00:00",
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
		return fmt.Errorf("No chain ID: %s", outputStr)
	}
	c.cfg.ChainID = matches[1]

	return nil
}

func (c *NamadaChain) copyGenesisFiles(ctx context.Context, n *NamadaNode) error {
	archivePath := fmt.Sprintf("%s.tar.gz", c.Config().ChainID)
	content, err := c.Validators[0].ReadFile(ctx, archivePath)
	if err != nil {
		return fmt.Errorf("failed to read the archive file: %w", err)
	}

	n.writeFile(ctx, archivePath, content)
	if err != nil {
		return fmt.Errorf("failed to write the archive file: %w", err)
	}

	walletPath := filepath.Join("pre-genesis", "wallet.toml")
	content, err = c.Validators[0].ReadFile(ctx, walletPath)
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
		content, err = c.Validators[0].ReadFile(ctx, validatorWalletPath)
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
