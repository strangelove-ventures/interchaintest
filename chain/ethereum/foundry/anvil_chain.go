package foundry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/mount"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/strangelove-ventures/interchaintest/v8/chain/ethereum"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"go.uber.org/zap"
)

var _ ibc.Chain = &AnvilChain{}

type AnvilChain struct {
	*ethereum.EthereumChain

	keystoreMap map[string]*NodeWallet
}

func NewAnvilChain(testName string, chainConfig ibc.ChainConfig, log *zap.Logger) *AnvilChain {
	return &AnvilChain{
		EthereumChain: ethereum.NewEthereumChain(testName, chainConfig, log),
		keystoreMap:   make(map[string]*NodeWallet),
	}
}

func (c *AnvilChain) KeystoreDir() string {
	return path.Join(c.HomeDir(), ".foundry", "keystores")
}

func (c *AnvilChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	cmd := []string{c.Config().Bin,
		"--host", "0.0.0.0", // Anyone can call
		"--no-cors",
		"--gas-price", c.Config().GasPrices,
	}

	cmd = append(cmd, c.Config().AdditionalStartArgs...)

	var mounts []mount.Mount
	if loadState, ok := c.Config().ConfigFileOverrides["--load-state"].(string); ok {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		localJsonFile := filepath.Join(pwd, loadState)
		dockerJsonFile := path.Join(c.HomeDir(), path.Base(loadState))
		mounts = []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: localJsonFile,
				Target: dockerJsonFile,
			},
		}
		cmd = append(cmd, "--load-state", dockerJsonFile)
	}

	return c.EthereumChain.Start(ctx, cmd, mounts)
}

type NewWalletOutput struct {
	Address string `json:"address"`
	Path    string `json:"path"`
}

func (c *AnvilChain) MakeKeystoreDir(ctx context.Context) error {
	cmd := []string{"mkdir", "-p", c.KeystoreDir()}
	_, _, err := c.Exec(ctx, cmd, nil)
	return err
}

func (c *AnvilChain) CreateKey(ctx context.Context, keyName string) error {
	err := c.MakeKeystoreDir(ctx) // Ensure keystore directory is created
	if err != nil {
		return err
	}

	_, ok := c.keystoreMap[keyName]
	if ok {
		return fmt.Errorf("keyname (%s) already used", keyName)
	}

	cmd := []string{"cast", "wallet", "new", c.KeystoreDir(), "--unsafe-password", "", "--json"}
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}

	newWallet := []NewWalletOutput{}
	err = json.Unmarshal(stdout, &newWallet)
	if err != nil {
		return err
	}

	c.keystoreMap[keyName] = &NodeWallet{
		keystore: newWallet[0].Path,
	}

	return nil
}

func (c *AnvilChain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	err := c.MakeKeystoreDir(ctx) // Ensure keystore directory is created
	if err != nil {
		return err
	}

	cmd := []string{"cast", "wallet", "import", keyName, "--keystore-dir", c.KeystoreDir(), "--mnemonic", mnemonic, "--unsafe-password", ""}
	_, _, err = c.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}

	// This is needed for CreateKey() since that keystore path does not use the keyname
	c.keystoreMap[keyName] = &NodeWallet{
		keystore: path.Join(c.KeystoreDir(), keyName),
	}

	return nil
}

// Get address of account, cast to a string to use
func (c *AnvilChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	account, ok := c.keystoreMap[keyName]
	if !ok {
		return nil, fmt.Errorf("keyname (%s) not found", keyName)
	}

	if account.address != "" {
		return hexutil.MustDecode(account.address), nil
	}

	cmd := []string{"cast", "wallet", "address", "--keystore", account.keystore, "--password", ""}
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return nil, err
	}

	addr := strings.TrimSpace(string(stdout))
	account.address = addr
	return hexutil.MustDecode(addr), nil
}

func (c *AnvilChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	_, err := c.SendFundsWithNote(ctx, keyName, amount, "")
	return err
}

type TransactionReceipt struct {
	TxHash string `json:"transactionHash"`
}

func (c *AnvilChain) SendFundsWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	var cmd []string
	if len(note) > 0 {
		cmd = []string{"cast", "send", amount.Address, hexutil.Encode([]byte(note)), "--value", amount.Amount.String(), "--json"}
	} else {
		cmd = []string{"cast", "send", amount.Address, "--value", amount.Amount.String(), "--json"}
	}

	account, ok := c.keystoreMap[keyName]
	if !ok {
		return "", fmt.Errorf("keyname (%s) not found", keyName)
	}
	cmd = append(cmd,
		"--keystore", account.keystore,
		"--password", "",
		"--rpc-url", c.GetRPCAddress(),
	)

	account.txLock.Lock()
	defer account.txLock.Unlock()
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return "", fmt.Errorf("send funds, exec, %w", err)
	}

	var txReceipt TransactionReceipt
	if err = json.Unmarshal([]byte(strings.TrimSpace(string(stdout))), &txReceipt); err != nil {
		return "", fmt.Errorf("tx receipt unmarshal:\n %s\nerror: %w", string(stdout), err)
	}

	return txReceipt.TxHash, nil
}

func (c *AnvilChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		err := c.RecoverKey(ctx, keyName, mnemonic)
		if err != nil {
			return nil, err
		}
	} else {
		// Use the genesis account
		if keyName == "faucet" {
			mnemonic = "test test test test test test test test test test test junk"
			err := c.RecoverKey(ctx, keyName, mnemonic)
			if err != nil {
				return nil, err
			}
		} else {
			// Create new account
			err := c.CreateKey(ctx, keyName)
			if err != nil {
				return nil, err
			}
		}
	}

	address, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, err
	}
	return ethereum.NewWallet(keyName, address, mnemonic), nil
}

type NodeWallet struct {
	address  string
	keystore string
	txLock   sync.Mutex
}
