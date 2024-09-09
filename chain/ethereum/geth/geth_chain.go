package geth

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/hd"

	"github.com/docker/docker/api/types/mount"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/strangelove-ventures/interchaintest/v9/chain/ethereum"
	"github.com/strangelove-ventures/interchaintest/v9/ibc"
	"github.com/strangelove-ventures/interchaintest/v9/testutil"
	"go.uber.org/zap"
)

var _ ibc.Chain = &GethChain{}

type GethChain struct {
	*ethereum.EthereumChain

	keynameToAccountMap map[string]*NodeWallet
	nextAcctNum         int
}

func NewGethChain(testName string, chainConfig ibc.ChainConfig, log *zap.Logger) *GethChain {
	return &GethChain{
		EthereumChain: ethereum.NewEthereumChain(testName, chainConfig, log),
		keynameToAccountMap: map[string]*NodeWallet{
			"faucet": {
				accountNum: 0,
			},
		},
		nextAcctNum: 1,
	}
}

func (c *GethChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	cmd := []string{c.Config().Bin,
		"--dev", "--datadir", c.HomeDir(), "-http", "--http.addr", "0.0.0.0", "--http.port", "8545", "--allow-insecure-unlock",
		"--http.api", "eth,net,web3,miner,personal,txpool,debug", "--http.corsdomain", "*", "-nodiscover", "--http.vhosts=*",
		"--miner.gasprice", c.Config().GasPrices,
		"--rpc.allow-unprotected-txs",
	}

	cmd = append(cmd, c.Config().AdditionalStartArgs...)

	return c.EthereumChain.Start(ctx, cmd, []mount.Mount{})
}

// JavaScriptExec() - Execute web3 code via geth's attach command
func (c *GethChain) JavaScriptExec(ctx context.Context, jsCmd string) (stdout, stderr []byte, err error) {
	cmd := []string{c.Config().Bin, "--exec", jsCmd, "--datadir", c.HomeDir(), "attach"}
	return c.Exec(ctx, cmd, nil)
}

// JavaScriptExecTx() - Execute a tx via web3, function ensures account is unlocked and blocks multiple txs
func (c *GethChain) JavaScriptExecTx(ctx context.Context, account *NodeWallet, jsCmd string) (stdout, stderr []byte, err error) {
	if err := c.UnlockAccount(ctx, account); err != nil {
		return nil, nil, err
	}

	account.txLock.Lock()
	defer account.txLock.Unlock()
	stdout, stderr, err = c.JavaScriptExec(ctx, jsCmd)
	if err != nil {
		return nil, nil, err
	}

	err = testutil.WaitForBlocks(ctx, 2, c)
	return stdout, stderr, err
}

func (c *GethChain) CreateKey(ctx context.Context, keyName string) error {
	_, ok := c.keynameToAccountMap[keyName]
	if ok {
		return fmt.Errorf("keyname (%s) already used", keyName)
	}

	cmd := []string{
		"sh",
		"-c",
		fmt.Sprintf(`cat <<EOF | geth account new --datadir %s


EOF
`, c.HomeDir())}
	_, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}

	c.keynameToAccountMap[keyName] = &NodeWallet{
		accountNum: c.nextAcctNum,
	}
	c.nextAcctNum++

	return nil
}

func (c *GethChain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	_, ok := c.keynameToAccountMap[keyName]
	if ok {
		return fmt.Errorf("keyname (%s) already used", keyName)
	}

	derivedPriv, err := hd.Secp256k1.Derive()(mnemonic, "", hd.CreateHDPath(60, 0, 0).String())
	if err != nil {
		return err
	}

	privKey := hd.Secp256k1.Generate()(derivedPriv)

	_, _, err = c.JavaScriptExec(ctx, fmt.Sprintf("personal.importRawKey(%q, \"\")", hex.EncodeToString(privKey.Bytes())))
	if err != nil {
		return err
	}

	c.keynameToAccountMap[keyName] = &NodeWallet{
		accountNum: c.nextAcctNum,
	}
	c.nextAcctNum++

	return nil
}

// Get address of account, cast to a string to use
func (c *GethChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	account, found := c.keynameToAccountMap[keyName]
	if !found {
		return nil, fmt.Errorf("GetAddress(): Keyname (%s) not found", keyName)
	}

	if account.address != "" {
		return hexutil.MustDecode(account.address), nil
	}

	stdout, _, err := c.JavaScriptExec(ctx, fmt.Sprintf("eth.accounts[%d]", account.accountNum))
	if err != nil {
		return nil, err
	}

	// it can take a second or two for the web3 interface to get access to a new account
	for count := 0; strings.TrimSpace(string(stdout)) == "undefined"; count++ {
		time.Sleep(time.Second)
		stdout, _, err = c.JavaScriptExec(ctx, fmt.Sprintf("eth.accounts[%d]", account.accountNum))
		if err != nil {
			return nil, err
		}
		if count > 3 {
			return nil, fmt.Errorf("getAddress(): Keyname (%s) with account (%d) not found",
				keyName, account.accountNum)
		}
	}

	return hexutil.MustDecode(strings.Trim(strings.TrimSpace(string(stdout)), "\"")), nil
}

// UnlockAccount() unlocks a non-default account for use. We will unlock when sending funds and deploying contracts.
// Accounts are unlocked for 100 seconds which is plenty of time for the transaction.
func (c *GethChain) UnlockAccount(ctx context.Context, account *NodeWallet) error {
	// shouldn't need to unlock the default account
	if account.accountNum == 0 {
		return nil
	}

	_, _, err := c.JavaScriptExec(ctx, fmt.Sprintf("personal.unlockAccount(eth.accounts[%d], \"\", 100)", account.accountNum))

	return err
}

func (c *GethChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	_, err := c.SendFundsWithNote(ctx, keyName, amount, "")
	return err
}

func (c *GethChain) SendFundsWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	account, found := c.keynameToAccountMap[keyName]
	if !found {
		return "", fmt.Errorf("keyname (%s) not found", keyName)
	}

	var cmd string
	if len(note) > 0 {
		cmd = fmt.Sprintf("eth.sendTransaction({from: eth.accounts[%d],to: %q,value: %s,data: \"%s\"});",
			account.accountNum, amount.Address, amount.Amount, hexutil.Encode([]byte(note)))
	} else {
		cmd = fmt.Sprintf("eth.sendTransaction({from: eth.accounts[%d],to: %q,value: %s});",
			account.accountNum, amount.Address, amount.Amount)
	}
	stdout, _, err := c.JavaScriptExecTx(ctx, account, cmd)
	if err != nil {
		return "", err
	}
	return strings.Trim(strings.TrimSpace(string(stdout)), "\""), nil
}

// DeployContract creates a new contract on-chain, returning the contract address
// Constructor params are appended to the byteCode
func (c *GethChain) DeployContract(ctx context.Context, keyName string, abi []byte, byteCode []byte) (string, error) {
	account, found := c.keynameToAccountMap[keyName]
	if !found {
		return "", fmt.Errorf("SendFundsWithNote(): Keyname (%s) not found", keyName)
	}

	stdout, _, err := c.JavaScriptExecTx(ctx, account,
		fmt.Sprintf("eth.contract(abi=%s).new({from: eth.accounts[%d], data: %q, gas: 20000000}).transactionHash",
			abi, account.accountNum, byteCode),
	)
	if err != nil {
		return "", err
	}

	txHash := strings.TrimSpace(string(stdout))
	status, _, err := c.JavaScriptExec(ctx, fmt.Sprintf("eth.getTransactionReceipt(%s).status", txHash))
	if err != nil {
		return "", err
	}

	if strings.Trim(strings.TrimSpace(string(status)), "\"") == "0x0" {
		return "", fmt.Errorf("contract deployment failed")
	}

	stdout, _, err = c.JavaScriptExec(ctx, fmt.Sprintf("eth.getTransactionReceipt(%s).contractAddress", txHash))
	if err != nil {
		return "", err
	}

	return strings.Trim(strings.TrimSpace(string(stdout)), "\""), nil
}

func (c *GethChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		err := c.RecoverKey(ctx, keyName, mnemonic)
		if err != nil {
			return nil, err
		}
	} else {
		// faucet is created when the chain starts and will be account #0
		if keyName == "faucet" {
			return ethereum.NewWallet(keyName, []byte{}, mnemonic), nil
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
	accountNum int
	address    string
	txLock     sync.Mutex
}
