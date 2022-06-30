package cosmos

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/strangelove-ventures/ibctest/broadcast"
	"github.com/strangelove-ventures/ibctest/internal/dockerutil"
)

var _ broadcast.Broadcaster = &Broadcaster{}

type Broadcaster struct {
	buf *bytes.Buffer
	kr  keyring.Keyring

	// chain is a reference to the CosmosChain instance which will be the target of the messages.
	chain *CosmosChain
	// t is the testing.T for the current test.
	t *testing.T

	// factoryOptions is a slice of broadcast.FactoryOpt which enables arbitrary configuration of the tx.Factory.
	factoryOptions []broadcast.FactoryOpt
	// clientContextOptions is a slice of broadcast.ClientContextOpt which enables arbitrary configuration of the client.Context.
	clientContextOptions []broadcast.ClientContextOpt
}

func (b *Broadcaster) GetTxResponseBytes(ctx context.Context, user broadcast.User) ([]byte, error) {
	if b.buf == nil || b.buf.Len() == 0 {
		return nil, fmt.Errorf("empty buffer, transaction has not be executed yet")
	}

	return b.buf.Bytes(), nil
}

func (b *Broadcaster) UnmarshalTxResponseBytes(ctx context.Context, bytes []byte) (sdk.TxResponse, error) {
	resp := sdk.TxResponse{}
	if err := defaultEncoding.Marshaler.UnmarshalJSON(bytes, &resp); err != nil {
		return sdk.TxResponse{}, err
	}
	return resp, nil
}

func (b *Broadcaster) ConfigureFactoryOptions(opts ...broadcast.FactoryOpt) {
	b.factoryOptions = append(b.factoryOptions, opts...)
}

func (b *Broadcaster) ConfigureClientContextOptions(opts ...broadcast.ClientContextOpt) {
	b.clientContextOptions = append(b.clientContextOptions, opts...)
}

func NewBroadcaster(t *testing.T, chain *CosmosChain) *Broadcaster {
	return &Broadcaster{
		t:     t,
		chain: chain,
	}
}

func (b *Broadcaster) GetFactory(ctx context.Context, user broadcast.User) (tx.Factory, error) {
	clientContext, err := b.GetClientContext(ctx, user)
	if err != nil {
		return tx.Factory{}, err
	}

	sdkAdd, err := sdk.AccAddressFromBech32(user.Bech32Address(b.chain.Config().Bech32Prefix))
	if err != nil {
		return tx.Factory{}, err
	}

	accNumber, err := clientContext.AccountRetriever.GetAccount(clientContext, sdkAdd)
	if err != nil {
		return tx.Factory{}, err
	}

	f := defaultTxFactory(clientContext, factoryOptions{
		accNum:    accNumber.GetAccountNumber(),
		gasAdj:    b.chain.Config().GasAdjustment,
		memo:      "ibc-test",
		gasPrices: b.chain.Config().GasPrices,
	})

	for _, opt := range b.factoryOptions {
		f = opt(f)
	}
	return f, nil
}

func (b *Broadcaster) GetClientContext(ctx context.Context, user broadcast.User) (client.Context, error) {
	chain := b.chain
	cn := chain.getFullNode()

	if b.kr == nil {
		localDir := b.t.TempDir()
		containerKeyringDir := fmt.Sprintf("%s/keyring-test", cn.NodeHome())
		kr, err := dockerutil.NewDockerKeyring(ctx, cn.Pool.Client, localDir, containerKeyringDir, cn.Container.ID)
		if err != nil {
			return client.Context{}, err
		}
		b.kr = kr
	}

	sdkAdd, err := sdk.AccAddressFromBech32(user.Bech32Address(chain.Config().Bech32Prefix))
	if err != nil {
		return client.Context{}, err
	}

	clientContext, buf, err := defaultClientContext(chain, user, b.kr, sdkAdd)
	b.buf = buf
	for _, opt := range b.clientContextOptions {
		clientContext = opt(clientContext)
	}
	return clientContext, nil
}

type factoryOptions struct {
	gasPrices     string
	accNum        uint64
	accSeq        uint64
	gasAdj        float64
	memo          string
	timeoutHeight uint64
}

func defaultClientContext(chain *CosmosChain, fromUser broadcast.User, kr keyring.Keyring, sdkAdd sdk.AccAddress) (client.Context, *bytes.Buffer, error) {
	var buf bytes.Buffer
	cn := chain.getFullNode()
	return cn.CliContext().
		WithOutput(&buf).
		WithFrom(fromUser.Bech32Address(chain.Config().Bech32Prefix)).
		WithFromAddress(sdkAdd).
		WithFromName(fromUser.GetKeyName()).
		WithSkipConfirmation(true).
		WithAccountRetriever(authtypes.AccountRetriever{}).
		WithKeyring(kr).
		WithBroadcastMode(flags.BroadcastBlock).
		WithCodec(defaultEncoding.Marshaler).
		WithHomeDir(cn.Home), &buf, nil
}

// defaultTxFactory creates a new Factory.
func defaultTxFactory(clientCtx client.Context, opts factoryOptions) tx.Factory {
	signMode := signing.SignMode_SIGN_MODE_DIRECT
	return tx.Factory{}.
		WithAccountNumber(opts.accSeq).
		WithSequence(opts.accSeq).
		WithSignMode(signMode).
		WithGasAdjustment(opts.gasAdj).
		WithGas(flags.DefaultGasLimit).
		WithGasPrices(opts.gasPrices).
		WithMemo(opts.memo).
		WithTimeoutHeight(opts.timeoutHeight).
		WithTxConfig(clientCtx.TxConfig).
		WithAccountRetriever(clientCtx.AccountRetriever).
		WithKeybase(clientCtx.Keyring).
		WithChainID(clientCtx.ChainID).
		WithSimulateAndExecute(false)
}
