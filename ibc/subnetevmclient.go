package ibc

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/strangelove-ventures/interchaintest/v8/chain/avalanche/ics20/ics20bank"
)

type SubnetEvmClient struct {
	client *ethclient.Client
}

type AvalancheSubnetClient interface {
	// SendFunds sends funds to a wallet from a user account.
	SendFunds(ctx context.Context, keyName string, amount WalletAmount) error

	// Height returns the current block height or an error if unable to get current height.
	Height(ctx context.Context) (uint64, error)

	// GetBankBalance returns balance from Bank Smart contract
	GetBankBalance(ctx context.Context, bank, address, denom string) (int64, error)

	// GetBalance fetches the current balance for a specific account address
	GetBalance(ctx context.Context, address string) (int64, error)
}

type AvalancheSubnetClientFactory func(string, string) (AvalancheSubnetClient, error)

type AvalancheSubnetConfig struct {
	Name                string
	ChainID             string
	Genesis             []byte
	SubnetClientFactory AvalancheSubnetClientFactory
}

func (sec SubnetEvmClient) SendFunds(ctx context.Context, keyName string, amount WalletAmount) error {
	chainID, err := sec.client.NetworkID(context.Background())
	if err != nil {
		return fmt.Errorf("can't get chainID: %w", err)
	}

	privateKey, err := crypto.HexToECDSA(keyName)
	if err != nil {
		return fmt.Errorf("can't parse private key: %s", err)
	}

	senderAddr := crypto.PubkeyToAddress(privateKey.PublicKey)

	senderNonce, err := sec.client.PendingNonceAt(ctx, senderAddr)
	if err != nil {
		return fmt.Errorf("can't get nonce: %w", err)
	}

	gasPrice, err := sec.client.SuggestGasPrice(context.Background())
	if err != nil {
		return fmt.Errorf("can't get gas price: %w", err)
	}

	toAddress := common.HexToAddress(amount.Address)
	utx := types.NewTransaction(senderNonce, toAddress, big.NewInt(amount.Amount.Int64()), 21000, gasPrice, nil)
	signedTx, err := types.SignTx(utx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return fmt.Errorf("can't sign transaction: %w", err)
	}

	return sec.client.SendTransaction(ctx, signedTx)
}

func (sec SubnetEvmClient) Height(ctx context.Context) (uint64, error) {
	return sec.client.BlockNumber(ctx)
}

func (sec SubnetEvmClient) GetBankBalance(ctx context.Context, bank, address, denom string) (int64, error) {
	if !common.IsHexAddress(bank) {
		return 0, fmt.Errorf("bad bank address '%s'", bank)
	}

	if !common.IsHexAddress(address) {
		return 0, fmt.Errorf("bad user address '%s'", address)
	}

	bankContract, err := ics20bank.NewICS20Bank(common.HexToAddress(bank), sec.client)
	if err != nil {
		return 0, err
	}

	balance, err := bankContract.BalanceOf(nil, common.HexToAddress(address), denom)
	if err != nil {
		return 0, err
	}

	return balance.Int64(), nil
}

func (sec SubnetEvmClient) GetBalance(ctx context.Context, address string) (int64, error) {
	if !common.IsHexAddress(address) {
		return 0, fmt.Errorf("bad address format")
	}
	balance, err := sec.client.BalanceAt(ctx, common.HexToAddress(address), nil)
	if err != nil {
		return 0, err
	}
	return balance.Int64(), nil
}

func (sec SubnetEvmClient) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	gprice, err := sec.client.SuggestGasPrice(context.Background())
	if err != nil {
		panic(err)
	}
	return gprice.Int64() * gasPaid
}
