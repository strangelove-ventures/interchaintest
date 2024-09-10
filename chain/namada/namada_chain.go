package namada

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"cosmossdk.io/math"
	"github.com/BurntSushi/toml"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
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

	baseDir string

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
	c.baseDir = tempBaseDir

	return nil
}

// Start to set up
func (c *NamadaChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	baseDir := c.HomeDir()
	c.copyTemplates(baseDir)
	c.copyWasms(baseDir)

	err := c.setValidators(baseDir)
	if err != nil {
		return fmt.Errorf("Setting validators failed: %v", err)
	}

	err = c.initAccounts(ctx, baseDir, additionalGenesisWallets...)
	if err != nil {
		return fmt.Errorf("Initializing accounts failed: %v", err)
	}

	err = c.updateParameters(baseDir)
	if err != nil {
		return fmt.Errorf("Updating parameters failed: %v", err)
	}

	err = c.initNetwork(baseDir)
	if err != nil {
		return fmt.Errorf("init-network failed: %v", err)
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for _, n := range c.Validators {
		n := n

		eg.Go(func() error {
			return n.CreateContainer(egCtx, baseDir)
		})
	}

	for _, n := range c.FullNodes {
		n := n

		eg.Go(func() error {
			return n.CreateContainer(egCtx, baseDir)
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

	if err := testutil.WaitForBlocksUtil(10, func(i int) error {
		time.Sleep(5 * time.Second)
		return c.Validators[0].CheckMaspFiles(ctx)
	}); err != nil {
		return err
	}

	if err := testutil.WaitForBlocks(ctx, 2, c.Validators[0]); err != nil {
		return err
	}

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
	return fmt.Sprintf("http://%s:27657", c.Validators[0].HostName())
}

// gRPC address
func (c *NamadaChain) GetGRPCAddress() string {
	panic("No gRPC address for Namada")
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

// Home directory
func (c *NamadaChain) HomeDir() string {
	return c.baseDir
}

// Create a test key
func (c *NamadaChain) CreateKey(ctx context.Context, keyName string) error {
	cmd := exec.Command(
		"namadaw",
		"--base-dir",
		c.HomeDir(),
		"--pre-genesis",
		"gen",
		"--alias",
		keyName,
		"--unsafe-dont-encrypt",
	)
	_, err := cmd.CombinedOutput()

	return err
}

// Recovery a test key
func (c *NamadaChain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	cmd := exec.Command(
		"echo",
		mnemonic,
		"|",
		"namadaw",
		"--base-dir",
		c.HomeDir(),
		"--pre-genesis",
		"derive",
		"--alias",
		keyName,
		"--unsafe-dont-encrypt",
	)
	_, err := cmd.CombinedOutput()

	return err
}

// Get the Namada address
func (c *NamadaChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	cmd := exec.Command(
		"namadaw",
		"--base-dir",
		c.HomeDir(),
		"--pre-genesis",
		"find",
		"--alias",
		keyName,
	)

	output, err := cmd.CombinedOutput()
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
	panic("implement me")
}

// Send funds to a wallet from a user account with a memo
func (c *NamadaChain) SendFundsWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	panic("implement me")
}

// Send on IBC transfer
func (c *NamadaChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	panic("implement me")
}

// Current block height
func (c *NamadaChain) Height(ctx context.Context) (int64, error) {
	panic("implement me")
}

// Get the balance
func (c *NamadaChain) GetBalance(ctx context.Context, address string, denom string) (math.Int, error) {
	panic("implement me")
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

// Namada wallet for pre-genesis
func (c *NamadaChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		if err := c.RecoverKey(ctx, keyName, mnemonic); err != nil {
			return nil, fmt.Errorf("failed to recover key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
	} else {
		if err := c.CreateKey(ctx, keyName); err != nil {
			return nil, fmt.Errorf("failed to generate a key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
	}

	addrBytes, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, c.cfg.Name, err)
	}

	return NewWallet(keyName, addrBytes, mnemonic, c.cfg), nil
}

// Namada wallet for a relayer
func (c *NamadaChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	// Execute the command here to get the mnemonic
	cmd := exec.Command(
		"namadaw",
		"--base-dir",
		c.HomeDir(),
		"--pre-genesis",
		"gen",
		"--alias",
		keyName,
		"--unsafe-dont-encrypt",
	)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	re := regexp.MustCompile(`[a-z]+(?:\s+[a-z]+){23}`)
	mnemonic := re.FindString(outputStr)

	addrBytes, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, c.cfg.Name, err)
	}

	return NewWallet(keyName, addrBytes, mnemonic, c.cfg), nil
}

func (c *NamadaChain) copyTemplates(baseDir string) error {
	namadaRepo := os.Getenv("ENV_NAMADA_REPO")
	if namadaRepo == "" {
		fmt.Println("ENV_NAMADA_REPO is empty")
		return errors.New("ENV_NAMADA_REPO isn't specified")
	}
	sourceDir := filepath.Join(namadaRepo, "genesis", "localnet")

	destDir := filepath.Join(baseDir, "templates")
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		fmt.Println("Making template directory failed: %v", err)
		return err
	}

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path != sourceDir && info.IsDir() {
			return filepath.SkipDir
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".toml") {
			destPath := filepath.Join(destDir, info.Name())

			err := copyFile(path, destPath)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

func (c *NamadaChain) copyWasms(baseDir string) error {
	namadaRepo := os.Getenv("ENV_NAMADA_REPO")
	if namadaRepo == "" {
		fmt.Println("ENV_NAMADA_REPO is empty")
		return errors.New("ENV_NAMADA_REPO isn't specified")
	}
	sourceDir := filepath.Join(namadaRepo, "wasm")

	destDir := filepath.Join(baseDir, "wasm")
	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		fmt.Println("Making wasm directory failed: %v", err)
		return err
	}

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".wasm") {
			destPath := filepath.Join(destDir, info.Name())

			err := copyFile(path, destPath)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	srcPath := filepath.Join(sourceDir, "checksums.json")
	destPath := filepath.Join(destDir, "checksums.json")
	return copyFile(srcPath, destPath)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}

func (c *NamadaChain) setValidators(baseDir string) error {
	transactionPath := filepath.Join(baseDir, "transactions.toml")
	for i := 0; i < c.NumValidators; i++ {
		alias := fmt.Sprintf("validator-%d-balance-key", i)
		validatorAlias := fmt.Sprintf("validator-%d", i)

		// Generate a validator key
		cmd := exec.Command("namadaw", "--base-dir", baseDir, "--pre-genesis", "gen", "--alias", alias, "--unsafe-dont-encrypt")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Validator key couldn't be generated: %v", err)
		}

		// Initialize an established account of the validator
		cmd = exec.Command("namadac", "--base-dir", baseDir, "utils", "init-genesis-established-account", "--aliases", alias, "--path", transactionPath)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Initializing an validator account failed: %v", err)
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
		cmd = exec.Command("namadaw", "--base-dir", baseDir, "--pre-genesis", "add", "--alias", validatorAlias, "--value", validatorAddress)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Validator address couldn't be added: %v", err)
		}

		// Initialize a genesis validator
		cmd = exec.Command("namadac", "--base-dir", baseDir, "utils", "init-genesis-validator", "--alias", validatorAlias, "--address", validatorAddress, "--path", transactionPath, "--net-address", netAddress(), "--commission-rate", "0.05", "--max-commission-rate-change", "0.01", "--email", "null@null.net", "--self-bond-amount", "100000", "--unsafe-dont-encrypt")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Initializing a genesis validator failed: %v", err)
		}

		cmd = exec.Command("namadac", "--base-dir", baseDir, "utils", "sign-genesis-txs", "--alias", validatorAlias, "--path", transactionPath, "--output", transactionPath)
		_, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("Signing genesis transactions failed: %v", err)
		}
	}

	destPath := filepath.Join(baseDir, "templates", "transactions.toml")
	copyFile(transactionPath, destPath)

	return nil
}

func (c *NamadaChain) initAccounts(ctx context.Context, baseDir string, additionalGenesisWallets ...ibc.WalletAmount) error {
	templateDir := filepath.Join(baseDir, "templates")
	balancePath := filepath.Join(templateDir, "balances.toml")

	// Initialize balances.toml
	balanceFile, err := os.Create(balancePath)
	if err != nil {
		return err
	}
	defer balanceFile.Close()
	_, err = balanceFile.WriteString("[token.NAM]\n")
	if err != nil {
		return err
	}

	// for validators
	for i := 0; i < c.NumValidators; i++ {
		addr, err := c.GetAddress(ctx, fmt.Sprintf("validator-%d", i))
		if err != nil {
			return err
		}
		line := fmt.Sprintf("%s = \"2000000\"\n", string(addr))
		_, err = balanceFile.WriteString(line)
		if err != nil {
			return err
		}
	}

	for _, wallet := range additionalGenesisWallets {
		line := fmt.Sprintf("%s = \"%s\"\n", wallet.Address, wallet.Amount)
		_, err := balanceFile.WriteString(line)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *NamadaChain) updateParameters(baseDir string) error {
	templateDir := filepath.Join(baseDir, "templates")
	paramPath := filepath.Join(templateDir, "parameters.toml")

	var data map[string]interface{}
	if _, err := toml.DecodeFile(paramPath, &data); err != nil {
		return err
	}
	pgfParams, ok := data["pgf_params"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("No pgf_params")
	}
	pgfParams["stewards"] = []string{}

	file, err := os.Create(paramPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	return encoder.Encode(data)
}

func (c *NamadaChain) initNetwork(baseDir string) error {
	templatesDir := filepath.Join(baseDir, "templates")
	wasmDir := filepath.Join(baseDir, "wasm")
	checksumsPath := filepath.Join(wasmDir, "checksums.json")
	cmd := exec.Command(
		"namadac",
		"--base-dir",
		baseDir,
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
		baseDir,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("init-network failed: %s", output)
	}
	outputStr := string(output)

	re := regexp.MustCompile(`Derived chain ID: (\S+)`)
	matches := re.FindStringSubmatch(outputStr)
	if len(matches) < 2 {
		return fmt.Errorf("No chain ID: %s", outputStr)
	}
	c.cfg.ChainID = matches[1]

	return err
}
