package cardano

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand/v2"
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
	rnd                *rand.Rand
	protocolParameters *cardano.PParams

	blocks     map[uint64]ouroboroscommon.Point
	blocksLock sync.Mutex

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
		rnd:          rand.New(rand.NewPCG(uint64(time.Now().Unix()), 0)),
		blocks:       make(map[uint64]ouroboroscommon.Point),
		keys:         make(map[string]ed25519.PrivateKey),
		addrLocks:    make(map[string]*sync.Mutex),
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
	return fmt.Sprintf("xrp-%s-%s", a.cfg.ChainID, dockerutil.SanitizeContainerName(a.testName))
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

	// create n2c connection
	n2cConn, err := net.Dial("tcp", a.GetHostPeerAddress())
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

	err = a.clientConn.ChainSync().Client.Sync([]ouroboroscommon.Point{ouroboroscommon.NewPointOrigin()})
	if err != nil {
		return fmt.Errorf("fail to start chain sync: %w", err)
	}

	start := time.Now()
	for {
		if time.Since(start) > 1*time.Minute {
			return fmt.Errorf("timeout waiting for cardano node to start")
		}
		a.blocksLock.Lock()
		if _, ok := a.blocks[3]; ok {
			break
		}
		time.Sleep(200 * time.Millisecond)
		a.blocksLock.Unlock()
	}

	pparams, err := a.clientConn.LocalStateQuery().Client.GetCurrentProtocolParams()
	if err != nil {
		return fmt.Errorf("fail to get protocol params: %w", err)
	}
	a.protocolParameters = pparams.Utxorpc()

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

func (a *AdaChain) ExportState(ctx context.Context, height int64) (string, error) {
	panic(errNotImplemented())
}

func (a *AdaChain) GetRPCAddress() string {
	rpcPortNumber := strings.Split(n2cPort, "/")
	return fmt.Sprintf("http://%s:%s", a.HostName(), rpcPortNumber[0])
}

func (a *AdaChain) GetGRPCAddress() string {
	panic(errNotImplemented())
}

func (a *AdaChain) GetHostRPCAddress() string {
	rpcPortNumber := strings.Split(n2cPort, "/")
	return fmt.Sprintf("http://127.0.0.1:%s", rpcPortNumber[0])
}

func (a *AdaChain) GetHostPeerAddress() string {
	rpcPortNumber := strings.Split(n2nPort, "/")
	return fmt.Sprintf("http://127.0.0.1:%s", rpcPortNumber[0])
}

func (a *AdaChain) GetHostGRPCAddress() string {
	panic(errNotImplemented())
}

func (a *AdaChain) HomeDir() string {
	return "/app"
}

func (a *AdaChain) CreateKey(ctx context.Context, keyName string) error {
	seed := sha256.New().Sum([]byte(keyName))
	privKey := ed25519.NewKeyFromSeed(seed)

	a.keysLock.Lock()
	defer a.keysLock.Unlock()
	a.keys[keyName] = privKey

	return nil
}

func (a *AdaChain) RecoverKey(ctx context.Context, name, mnemonic string) error {
	panic(errNotImplemented())
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

	// find tx inputs
	ledgerAddr, err := ledger.NewAddress(amount.Address)
	if err != nil {
		return "", fmt.Errorf("invalid address: %w", err)
	}
	utxoRes, err := a.clientConn.LocalStateQuery().Client.GetUTxOByAddress([]ledger.Address{ledgerAddr})
	if err != nil {
		return "", fmt.Errorf("failed to get utxo: %w", err)
	}
	amt := amount.Amount.Uint64()
	var txInputs []gtx.TxInput
	for txID, utxo := range utxoRes.Results {
		txInputs = append(txInputs, gtx.NewTxInput(txID.Hash.String(), uint16(txID.Idx), utxo.Amount()))
		amt -= utxo.Amount()
		if amt <= 0 {
			break
		}
	}
	if amt > 0 {
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

	return txHashHex, nil
}

func (a *AdaChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	//TODO implement me
	panic("implement me")
}

func (a *AdaChain) Height(ctx context.Context) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (a *AdaChain) GetBalance(ctx context.Context, address string, denom string) (math.Int, error) {
	//TODO implement me
	panic("implement me")
}

func (a *AdaChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	//TODO implement me
	panic("implement me")
}

func (a *AdaChain) Acknowledgements(ctx context.Context, height int64) ([]ibc.PacketAcknowledgement, error) {
	//TODO implement me
	panic("implement me")
}

func (a *AdaChain) Timeouts(ctx context.Context, height int64) ([]ibc.PacketTimeout, error) {
	//TODO implement me
	panic("implement me")
}

func (a *AdaChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	//TODO implement me
	panic("implement me")
}

func (a *AdaChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	//TODO implement me
	panic("implement me")
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
