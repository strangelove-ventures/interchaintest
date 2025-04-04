package cardano

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"cosmossdk.io/math"
	ouroboros "github.com/blinklabs-io/gouroboros"
	"github.com/blinklabs-io/gouroboros/ledger"
	"github.com/blinklabs-io/gouroboros/protocol/chainsync"
	ouroboroscommon "github.com/blinklabs-io/gouroboros/protocol/common"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/kocubinski/gardano/address"
	gtx "github.com/kocubinski/gardano/tx"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/utxorpc/go-codegen/utxorpc/v1alpha/cardano"
	"go.uber.org/zap"
)

var _ ibc.Chain = &AdaChain{}

const (
	n2cPort = "7007/tcp"
	n2nPort = "3001/tcp"
)

var natPorts = nat.PortMap{
	nat.Port(n2cPort): {},
	nat.Port(n2nPort): {},
}

type AdaChain struct {
	testName           string
	cfg                ibc.ChainConfig
	log                *zap.Logger
	clientConn         *ouroboros.Connection
	containerLifecycle *dockerutil.ContainerLifecycle
	networkError       chan error
	protocolParameters *cardano.PParams
	faucetAddress      address.Address

	blocks *blockDB

	keys     map[string]ed25519.PrivateKey
	keysLock sync.Mutex

	addrLocks     map[string]*sync.Mutex
	addrLocksLock sync.Mutex

	txWaiters     map[string]chan struct{}
	txWaitersLock sync.Mutex

	VolumeName   string
	NetworkID    string
	DockerClient *dockerclient.Client
}

func NewAdaChain(testName string, chainConfig ibc.ChainConfig, log *zap.Logger) *AdaChain {
	return &AdaChain{
		testName:     testName,
		cfg:          chainConfig,
		log:          log,
		networkError: make(chan error),
		blocks:       &blockDB{},
		keys:         make(map[string]ed25519.PrivateKey),
		addrLocks:    make(map[string]*sync.Mutex),
		txWaiters:    make(map[string]chan struct{}),
	}
}

func (a *AdaChain) Config() ibc.ChainConfig {
	return a.cfg
}

func (a *AdaChain) Initialize(ctx context.Context, testName string, cli *dockerclient.Client, networkID string) error {
	chainCfg := a.Config()
	a.pullImages(ctx, cli)
	image := chainCfg.Images[0]

	a.containerLifecycle = dockerutil.NewContainerLifecycle(a.log, cli, a.Name())

	v, err := cli.VolumeCreate(ctx, volume.CreateOptions{
		Labels: map[string]string{
			dockerutil.CleanupLabel:   testName,
			dockerutil.NodeOwnerLabel: a.Name(),
		},
	})
	if err != nil {
		return fmt.Errorf("creating volume for chain node: %w", err)
	}
	a.VolumeName = v.Name
	a.NetworkID = networkID
	a.DockerClient = cli

	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: a.log,

		Client: cli,

		VolumeName: v.Name,
		ImageRef:   image.Ref(),
		TestName:   testName,
		UidGid:     image.UIDGID,
	}); err != nil {
		return fmt.Errorf("set volume owner: %w", err)
	}

	return nil
}

func (a *AdaChain) Name() string {
	return fmt.Sprintf("ada-%s-%s", a.cfg.ChainID, dockerutil.SanitizeContainerName(a.testName))
}

func (a *AdaChain) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", a.VolumeName, a.HomeDir())}
}

func (a *AdaChain) HostName() string {
	return dockerutil.CondenseHostName(a.Name())
}

func (a *AdaChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	usingPorts := nat.PortMap{}
	for k, v := range natPorts {
		usingPorts[k] = v
	}
	if a.cfg.HostPortOverride != nil {
		var fields []zap.Field

		i := 0
		for intP, extP := range a.cfg.HostPortOverride {
			port := nat.Port(fmt.Sprintf("%d/tcp", intP))

			usingPorts[port] = []nat.PortBinding{
				{
					HostPort: fmt.Sprintf("%d", extP),
				},
			}

			fields = append(fields, zap.String(fmt.Sprintf("port_overrides_%d", i), fmt.Sprintf("%s:%d", port, extP)))
			i++
		}

		a.log.Info("Port overrides", fields...)
	}

	if len(a.faucetAddress) > 0 {
		a.cfg.Env = append(a.cfg.Env, fmt.Sprintf("FUND_ACCOUNT=%s", a.faucetAddress.String()))
	}
	err := a.containerLifecycle.CreateContainer(ctx, a.testName, a.NetworkID, a.cfg.Images[0],
		usingPorts, "", a.Bind(), []mount.Mount{}, a.HostName(), nil, a.cfg.Env, nil)
	if err != nil {
		return err
	}

	a.log.Info("Starting container", zap.String("container", a.Name()))

	if err = a.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	networkLogger := NewSlogWrapper(a.log)
	networkMagic := uint32(42)

	a.log.Info("waiting for cardano node to start")
	time.Sleep(15 * time.Second)

	// create n2c connection
	n2cConn, err := net.Dial("tcp", a.GetHostRPCAddress())
	if err != nil {
		return fmt.Errorf("fail to dial n2c host: %w", err)
	}
	a.clientConn, err = ouroboros.NewConnection(
		ouroboros.WithConnection(n2cConn),
		ouroboros.WithErrorChan(a.networkError),
		ouroboros.WithLogger(networkLogger),
		ouroboros.WithNetworkMagic(networkMagic),
		ouroboros.WithKeepAlive(true),
		ouroboros.WithChainSyncConfig(chainsync.NewConfig(
			chainsync.WithRollBackwardFunc(a.chainSyncRollBackwardHandler),
			chainsync.WithRollForwardFunc(a.chainSyncRollForwardHandler),
		)),
	)
	if err != nil {
		return fmt.Errorf("fail to create ouroboros n2c connection: %w", err)
	}

	a.log.Info("Starting ouroboros n2c connection", zap.String("host", a.GetRPCAddress()))
	err = a.clientConn.ChainSync().Client.Sync([]ouroboroscommon.Point{ouroboroscommon.NewPointOrigin()})
	if err != nil {
		return fmt.Errorf("fail to start chain sync: %w", err)
	}

	start := time.Now()
	for {
		if time.Since(start) > 1*time.Minute {
			return fmt.Errorf("timeout waiting for cardano node to start")
		}
		b, ok := a.blocks.last()
		if ok && b.BlockNumber >= 3 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	pparams, err := a.clientConn.LocalStateQuery().Client.GetCurrentProtocolParams()
	if err != nil {
		return fmt.Errorf("fail to get protocol params: %w", err)
	}
	a.protocolParameters = pparams.Utxorpc()

	return nil
}

func (a *AdaChain) Stop() error {
	if a.clientConn != nil {
		if err := a.clientConn.ChainSync().Client.Stop(); err != nil {
			return fmt.Errorf("fail to stop chain sync: %w", err)
		}
		a.clientConn.Close()
	}
	return nil
}

func (a *AdaChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	job := dockerutil.NewImage(a.logger(), a.DockerClient, a.NetworkID, a.testName, a.cfg.Images[0].Repository, a.cfg.Images[0].Version)
	opts := dockerutil.ContainerOptions{
		Env:   env,
		Binds: a.Bind(),
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}

func (a *AdaChain) GetRPCAddress() string {
	rpcPortNumber := strings.Split(n2cPort, "/")
	return fmt.Sprintf("%s:%s", a.HostName(), rpcPortNumber[0])
}

func (a *AdaChain) GetHostRPCAddress() string {
	rpcPortNumber := strings.Split(n2cPort, "/")
	return fmt.Sprintf("127.0.0.1:%s", rpcPortNumber[0])
}

func (a *AdaChain) GetHostPeerAddress() string {
	rpcPortNumber := strings.Split(n2nPort, "/")
	return fmt.Sprintf("127.0.0.1:%s", rpcPortNumber[0])
}

func (a *AdaChain) HomeDir() string {
	return "/app"
}

func (a *AdaChain) CreateKey(ctx context.Context, keyName string) error {
	seed := sha256.Sum256([]byte(keyName))
	privKey := ed25519.NewKeyFromSeed(seed[:])

	a.keysLock.Lock()
	defer a.keysLock.Unlock()
	a.keys[keyName] = privKey

	return nil
}

func (a *AdaChain) RecoverKey(ctx context.Context, name, mnemonic string) error {
	seed, err := mnemonicToEddKey(mnemonic, name)
	if err != nil {
		return fmt.Errorf("failed to recover key: %w", err)
	}
	privKey := ed25519.NewKeyFromSeed(seed)
	a.keysLock.Lock()
	defer a.keysLock.Unlock()
	a.keys[name] = privKey
	return nil
}

func (a *AdaChain) SetFaucet(ctx context.Context) (err error) {
	const keyName = "faucet"
	if err := a.CreateKey(ctx, keyName); err != nil {
		return err
	}
	key, ok := a.keys[keyName]
	if !ok {
		return fmt.Errorf("key not found: %s", keyName)
	}
	a.faucetAddress, err = address.PaymentOnlyTestnetAddressFromPubkey(key.Public().(ed25519.PublicKey))
	return err
}

func (a *AdaChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	a.keysLock.Lock()
	defer a.keysLock.Unlock()
	privKey, ok := a.keys[keyName]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", keyName)
	}
	return address.PaymentOnlyTestnetAddressFromPubkey(privKey.Public().(ed25519.PublicKey))
}

func (a *AdaChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	_, err := a.SendFundsWithNote(ctx, keyName, amount, "")
	return err
}

func (a *AdaChain) SendFundsWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	// fetch key and address
	senderKey, ok := a.keys[keyName]
	if !ok {
		return "", fmt.Errorf("key not found: %s", keyName)
	}
	fromAddr, err := address.PaymentOnlyTestnetAddressFromPubkey(senderKey.Public().(ed25519.PublicKey))
	if err != nil {
		return "", fmt.Errorf("failed to get address from public key: %w", err)
	}
	toAddr, err := address.NewAddressFromBech32(amount.Address)
	if err != nil {
		return "", fmt.Errorf("invalid address: %w", err)
	}

	// lock address
	addrLock := a.getAddressLock(keyName)
	addrLock.Lock()
	defer addrLock.Unlock()

	// create new n2c connection
	conn, err := a.newQueryConnection()
	if err != nil {
		return "", fmt.Errorf("failed to create connection: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); err != nil {
			a.log.Error("failed to close n2c connection", zap.Error(closeErr))
		}
	}()
	if err = a.clientConn.LocalStateQuery().Client.AcquireVolatileTip(); err != nil {
		return "", fmt.Errorf("failed to acquire volatile tip: %w", err)
	}
	defer func() {
		if err := a.clientConn.LocalStateQuery().Client.Release(); err != nil {
			a.log.Error("failed to release volatile tip", zap.Error(err))
		}
	}()

	// find tx inputs
	var txInputs []gtx.TxInput
	ledgerAddr, err := ledger.NewAddress(fromAddr.String())
	if err != nil {
		return "", fmt.Errorf("invalid address: %w", err)
	}
	utxoRes, err := a.clientConn.LocalStateQuery().Client.GetUTxOByAddress([]ledger.Address{ledgerAddr})
	if err != nil {
		return "", fmt.Errorf("failed to get utxo: %w", err)
	}
	amt := amount.Amount
	for txID, utxo := range utxoRes.Results {
		txInputs = append(txInputs, gtx.NewTxInput(txID.Hash.String(), uint16(txID.Idx), utxo.Amount()))
		amt = amt.Sub(math.NewIntFromUint64(utxo.Amount()))
		if amt.LTE(math.ZeroInt()) {
			break
		}
	}
	if amt.GT(math.ZeroInt()) {
		return "", fmt.Errorf("not enough funds, short by %d", amt)
	}

	// build tx
	txBuilder := gtx.NewTxBuilder(a.protocolParameters, []ed25519.PrivateKey{senderKey})
	txBuilder.AddOutputs(gtx.NewTxOutput(toAddr, amount.Amount.Uint64()))
	if err := txBuilder.SetMemo(note); err != nil {
		return "", fmt.Errorf("failed to set memo: %w", err)
	}

	txBuilder.AddInputs(txInputs...)
	if err := txBuilder.AddChangeIfNeeded(fromAddr); err != nil {
		return "", fmt.Errorf("failed to add change: %w", err)
	}
	tx, err := txBuilder.Build()
	if err != nil {
		return "", fmt.Errorf("failed to build tx: %w", err)
	}
	txBytes, err := tx.Bytes()
	if err != nil {
		return "", fmt.Errorf("failed to get tx bytes: %w", err)
	}

	// submit tx
	txHash, err := tx.Hash()
	if err != nil {
		return "", fmt.Errorf("failed to get tx hash: %w", err)
	}
	txHashHex := hex.EncodeToString(txHash[:])
	waiter := make(chan struct{})
	a.txWaitersLock.Lock()
	a.txWaiters[txHashHex] = waiter
	a.txWaitersLock.Unlock()

	era, err := a.clientConn.LocalStateQuery().Client.GetCurrentEra()
	if err != nil {
		return "", fmt.Errorf("failed to get current era: %w", err)
	}
	err = a.clientConn.LocalTxSubmission().Client.SubmitTx(uint16(era), txBytes)
	if err != nil {
		return "", fmt.Errorf("failed to submit tx: %w", err)
	}

	// wait for tx to be seen
	select {
	case <-waiter:
	case <-time.After(30 * time.Second):
		return "", fmt.Errorf("timeout waiting for tx to be seen")
	}

	a.log.Info("seen tx", zap.String("tx_hash", txHashHex))

	return txHashHex, nil
}

func (a *AdaChain) Height(ctx context.Context) (int64, error) {
	tip, err := a.clientConn.ChainSync().Client.GetCurrentTip()
	if err != nil {
		return 0, fmt.Errorf("failed to get current tip: %w", err)
	}
	return int64(tip.BlockNumber), nil
}

func (a *AdaChain) GetBalance(ctx context.Context, address string, denom string) (math.Int, error) {
	ledgerAddr, err := ledger.NewAddress(address)
	if err != nil {
		return math.Int{}, fmt.Errorf("invalid address: %w", err)
	}
	utxoRes, err := a.clientConn.LocalStateQuery().Client.GetUTxOByAddress([]ledger.Address{ledgerAddr})
	if err != nil {
		return math.Int{}, fmt.Errorf("failed to get utxo: %w", err)
	}
	if len(utxoRes.Results) == 0 {
		return math.Int{}, fmt.Errorf("no utxo found for address: %s", address)
	}
	amount := math.ZeroInt()
	for _, utxo := range utxoRes.Results {
		amount = amount.Add(math.NewIntFromUint64(utxo.Amount()))
	}
	return amount, nil
}

func (a *AdaChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		if err := a.RecoverKey(ctx, keyName, mnemonic); err != nil {
			return nil, fmt.Errorf("failed to recover key: %w", err)
		}
	} else {
		if err := a.CreateKey(ctx, keyName); err != nil {
			return nil, fmt.Errorf("failed to create key: %w", err)
		}
	}

	a.keysLock.Lock()
	defer a.keysLock.Unlock()
	privKey, ok := a.keys[keyName]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", keyName)
	}
	addr, err := address.PaymentOnlyTestnetAddressFromPubkey(privKey.Public().(ed25519.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("failed to get address from public key: %w", err)
	}

	return &wallet{
		keyName:  keyName,
		address:  addr,
		mnemonic: mnemonic,
	}, nil
}

func (a *AdaChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	panic(errNotImplemented())
}

func (a *AdaChain) pullImages(ctx context.Context, cli *dockerclient.Client) {
	for _, image := range a.Config().Images {
		rc, err := cli.ImagePull(
			ctx,
			image.Repository+":"+image.Version,
			dockertypes.ImagePullOptions{},
		)
		if err != nil {
			a.log.Error("Failed to pull image",
				zap.Error(err),
				zap.String("repository", image.Repository),
				zap.String("tag", image.Version),
			)
		} else {
			_, _ = io.Copy(io.Discard, rc)
			_ = rc.Close()
		}
	}
}

func (a *AdaChain) logger() *zap.Logger {
	return a.log.With(
		zap.String("chain_name", a.cfg.Name),
		zap.String("chain_id", a.cfg.ChainID),
		zap.String("test", a.testName),
	)
}

func (a *AdaChain) getAddressLock(keyname string) *sync.Mutex {
	a.addrLocksLock.Lock()
	defer a.addrLocksLock.Unlock()
	lock, ok := a.addrLocks[keyname]
	if !ok {
		lock = &sync.Mutex{}
		a.addrLocks[keyname] = lock
	}
	return lock
}

func (a *AdaChain) newQueryConnection() (*ouroboros.Connection, error) {
	n2cConn, err := net.Dial("tcp", a.GetHostRPCAddress())
	if err != nil {
		return nil, fmt.Errorf("fail to dial n2c host: %w", err)
	}
	return ouroboros.NewConnection(
		ouroboros.WithConnection(n2cConn),
		ouroboros.WithLogger(NewSlogWrapper(a.log)),
		ouroboros.WithNetworkMagic(42),
		ouroboros.WithKeepAlive(true),
	)
}
