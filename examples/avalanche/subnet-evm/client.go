package subnetevm

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

type SubnetEvmClient struct {
	client *ethclient.Client
}

func NewSubnetEvmClient(rpcHost string, pk string) (ibc.AvalancheSubnetClient, error) {
	client, err := ethclient.Dial(fmt.Sprintf("%s/rpc", rpcHost))
	if err != nil {
		return nil, err
	}
	return &SubnetEvmClient{client: client}, nil
}

func (sec SubnetEvmClient) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
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

func (sec SubnetEvmClient) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
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
