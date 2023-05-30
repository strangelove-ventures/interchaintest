package avalanche

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"

	"github.com/strangelove-ventures/interchaintest/v7/chain/avalanche/lib"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/internal/dockerutil"
)

var (
	RPCPort      = "9650/tcp"
	StakingPort  = "9651/tcp"
	portBindings = nat.PortSet{
		nat.Port(RPCPort):     {},
		nat.Port(StakingPort): {},
	}
)

type (
	AvalancheNode struct {
		chain *AvalancheChain

		logger             *zap.Logger
		containerLifecycle *dockerutil.ContainerLifecycle
		dockerClient       *dockerclient.Client
		image              ibc.DockerImage
		volume             types.Volume

		networkID string
		testName  string
		index     int
		options   AvalancheNodeOpts
	}

	AvalancheNodes []*AvalancheNode

	AvalancheNodeCredentials struct {
		PK      *secp256k1.PrivateKey
		ID      ids.NodeID
		TLSCert []byte
		TLSKey  []byte
	}

	AvalancheNodeSubnetOpts struct {
		Name    string
		VmID    ids.ID
		VM      []byte
		Genesis []byte

		subnet ids.ID
		chain  ids.ID
	}

	AvalancheNodeOpts struct {
		PublicIP    string
		Subnets     []AvalancheNodeSubnetOpts
		Bootstrap   []*AvalancheNode
		Credentials AvalancheNodeCredentials
		ChainID     lib.ChainID
	}
)

func NewAvalancheNode(
	ctx context.Context,
	chain *AvalancheChain,
	networkID string,
	testName string,
	dockerClient *dockerclient.Client,
	image ibc.DockerImage,
	containerIdx int,
	log *zap.Logger,
	genesis Genesis,
	options *AvalancheNodeOpts,
) (*AvalancheNode, error) {
	node := &AvalancheNode{
		chain:        chain,
		index:        containerIdx,
		logger:       log,
		dockerClient: dockerClient,
		image:        image,
		networkID:    networkID,
		testName:     testName,
		options:      *options,
	}

	name := node.Name()

	volume, err := dockerClient.VolumeCreate(ctx, volume.VolumeCreateBody{
		Name: name,
		Labels: map[string]string{
			dockerutil.CleanupLabel:   testName,
			dockerutil.NodeOwnerLabel: name,
		},
	})
	if err != nil {
		return nil, err
	}

	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log:        log,
		Client:     dockerClient,
		VolumeName: name,
		ImageRef:   image.Ref(),
		TestName:   testName,
		UidGid:     image.UidGid,
	}); err != nil {
		return nil, fmt.Errorf("set volume owner: %w", err)
	}

	node.volume = volume

	fmt.Printf("creating container lifecycle, name: %s\n", name)

	node.containerLifecycle = dockerutil.NewContainerLifecycle(log, dockerClient, name)

	genesisBz, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return nil, err
	}

	vmaliases := make(map[ids.ID][]string)
	for i := range node.options.Subnets {
		vmaliases[node.options.Subnets[i].VmID] = []string{node.options.Subnets[i].Name}
	}
	vmaliasesData, err := json.MarshalIndent(vmaliases, "", "  ")
	if err != nil {
		return nil, err
	}

	if err := node.WriteFile(ctx, genesisBz, "genesis.json"); err != nil {
		return nil, fmt.Errorf("failed to write genesis file: %w", err)
	}

	if err := node.WriteFile(ctx, options.Credentials.TLSCert, "tls.cert"); err != nil {
		return nil, fmt.Errorf("failed to write TLS certificate: %w", err)
	}

	if err := node.WriteFile(ctx, options.Credentials.TLSKey, "tls.key"); err != nil {
		return nil, fmt.Errorf("failed to write TLS key: %w", err)
	}

	if err := node.WriteFile(ctx, vmaliasesData, "configs/vms/aliases.json"); err != nil {
		return nil, fmt.Errorf("failed to write TLS key: %w", err)
	}

	for _, subnet := range node.options.Subnets {
		if err := node.WriteFile(ctx, subnet.VM, fmt.Sprintf("plugins/%s", subnet.VmID)); err != nil {
			return nil, fmt.Errorf("failed to write vm body [%s]: %w", subnet.Name, err)
		}
	}

	return node, node.CreateContainer(ctx)
}

func (n *AvalancheNode) HomeDir() string {
	return "/home/heighliner/ava"
}

func (n *AvalancheNode) Bind() []string {
	return []string{
		fmt.Sprintf("%s:%s", n.volume.Name, n.HomeDir()),
	}
}

func (n *AvalancheNode) WriteFile(ctx context.Context, content []byte, relPath string) error {
	fw := dockerutil.NewFileWriter(n.logger, n.dockerClient, n.testName)
	return fw.WriteFile(ctx, n.volume.Name, relPath, content)
}

func (n *AvalancheNode) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutil.NewImage(n.logger, n.dockerClient, n.networkID, n.testName, n.image.Repository, n.image.Version)
	opts := dockerutil.ContainerOptions{
		Binds: n.Bind(),
		Env:   env,
		User:  n.image.UidGid,
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}

func (n *AvalancheNode) NodeId() string {
	return n.options.Credentials.ID.String()
}

func (n *AvalancheNode) Name() string {
	return fmt.Sprintf(
		"av-%s-%d",
		dockerutil.SanitizeContainerName(n.testName),
		n.index,
	)
}

func (n *AvalancheNode) HostName() string {
	return dockerutil.CondenseHostName(n.Name())
}

func (n *AvalancheNode) PublicStakingAddr(ctx context.Context) (string, error) {
	netinfo, err := n.dockerClient.NetworkInspect(ctx, n.networkID, types.NetworkInspectOptions{})
	if err != nil {
		return "", err
	}
	info, err := n.dockerClient.ContainerInspect(ctx, n.containerLifecycle.ContainerID())
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"%s:9651",
		info.NetworkSettings.Networks[netinfo.Name].IPAddress,
	), nil
}

func (n *AvalancheNode) StakingPort() string {
	info, err := n.dockerClient.ContainerInspect(context.Background(), n.containerLifecycle.ContainerID())
	if err != nil {
		panic(err)
	}
	return info.HostConfig.PortBindings[nat.Port(StakingPort)][0].HostPort
}

func (n *AvalancheNode) RPCPort() string {
	info, err := n.dockerClient.ContainerInspect(context.Background(), n.containerLifecycle.ContainerID())
	if err != nil {
		panic(err)
	}
	return info.HostConfig.PortBindings[nat.Port(RPCPort)][0].HostPort
}

func (n *AvalancheNode) GRPCPort() string {
	panic(errors.New("doesn't support grpc"))
}

func (n *AvalancheNode) CreateKey(ctx context.Context, keyName string) error {
	// ToDo: create key
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/multisig-utxos-with-avalanchejs.md#setup-keychains-with-private-keys
	panic("ToDo: implement me")
}

func (n *AvalancheNode) RecoverKey(ctx context.Context, name, mnemonic string) error {
	// ToDo: recover key from mnemonic
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/multisig-utxos-with-avalanchejs.md#setup-keychains-with-private-keys
	panic("ToDo: implement me")
}

func (n *AvalancheNode) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	// ToDo: get address for keyname
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md
	panic("ToDo: implement me")
}

func (n *AvalancheNode) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	// ToDo: send some amount to keyName from rootAddress
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md
	// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/cross-chain-transfers.md
	// IF allocated chain subnet config:
	//   - Blockchain Handlers: /ext/bc/[chainID]
	//   - VM Handlers: /ext/vm/[vmID]
	// panic("ToDo: implement me")

	rawSubnet, ok := ctx.Value("subnet").(string)
	if !ok {
		return fmt.Errorf("can't read subnet from context")
	}

	subnet, err := strconv.Atoi(rawSubnet)
	if err != nil {
		return fmt.Errorf("can't parse subnet idx from context: %w", err)
	}

	chain := n.options.Subnets[subnet].chain
	rpcUrl := fmt.Sprintf("http://127.0.0.1:%s/ext/bc/%s/rpc", n.RPCPort(), chain)

	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return fmt.Errorf("can't create client for subnet[%s]: %w", chain, err)
	}

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return fmt.Errorf("can't connect to subnet[%s][%s]: %w", chain, rpcUrl, err)
	}

	n.logger.Info(
		"connected to subnet",
		zap.String("subnet", chain.String()),
		zap.Uint64("chainID", chainID.Uint64()),
	)

	privateKey, err := crypto.HexToECDSA(keyName)
	if err != nil {
		return fmt.Errorf("can't parse private key: %s", err)
	}

	senderAddr := crypto.PubkeyToAddress(privateKey.PublicKey)

	senderNonce, err := client.PendingNonceAt(ctx, senderAddr)
	if err != nil {
		return fmt.Errorf("can't get nonce: %w", err)
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return fmt.Errorf("can't get gas price: %w", err)
	}

	toAddress := ethcommon.HexToAddress(amount.Address)

	utx := ethtypes.NewTransaction(senderNonce, toAddress, big.NewInt(amount.Amount), 21000, gasPrice, nil)

	signedTx, err := ethtypes.SignTx(utx, ethtypes.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return fmt.Errorf("can't sign transaction: %w", err)
	}

	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return fmt.Errorf("can't send EVM tx: %w", err)
	}

	n.logger.Info(
		"successfully sent EVM tx to subnet",
		zap.Any("hash", signedTx.Hash()),
	)

	return nil
}

func (n *AvalancheNode) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	return ibc.Tx{}, errors.New("not yet implemented")
}

func (n *AvalancheNode) Height(ctx context.Context) (uint64, error) {
	rawSubnet, ok := ctx.Value("subnet").(string)
	// we have subnet passed via context
	if ok {
		subnet, err := strconv.Atoi(rawSubnet)
		if err != nil {
			return 0, fmt.Errorf("can't parse subnet idx from context: %w", err)
		}

		chain := n.options.Subnets[subnet].chain
		rpcUrl := fmt.Sprintf("http://127.0.0.1:%s/ext/bc/%s/rpc", n.RPCPort(), chain)

		client, err := ethclient.Dial(rpcUrl)
		if err != nil {
			return 0, fmt.Errorf("can't create client for subnet[%s]: %w", chain, err)
		}

		return client.BlockNumber(ctx)
	}

	return platformvm.NewClient(fmt.Sprintf("http://127.0.0.1:%s", n.RPCPort())).GetHeight(ctx)
}

func (n *AvalancheNode) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
	if strings.HasPrefix(address, "X-") {
		// ToDo: call /ext/bc/X (method avm.getBalance)
		// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md#check-x-chain-balance
		panic("ToDo: implement me")
	} else if strings.HasPrefix(address, "P-") {
		// ToDo: call /ext/bc/P (method platform.getBalance)
		// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md#check-p-chain-balance
		panic("ToDo: implement me")
	} else if strings.HasPrefix(address, "0x") {
		// ToDo: call /ext/bc/C/rpc (method eth_getBalance)
		// https://github.com/ava-labs/avalanche-docs/blob/c136e8752af23db5214ff82c2153aac55542781b/docs/quickstart/fund-a-local-test-network.md#check-the-c-chain-balance
		panic("ToDo: implement me")
	}
	// if allocated subnet, we must call /ext/bc/[chainID]
	return 0, fmt.Errorf("address should be have prefix X, P, 0x. current address: %s", address)
}

func (n *AvalancheNode) IP() string {
	return n.options.PublicIP
}

func (n *AvalancheNode) CreateContainer(ctx context.Context) error {
	netinfo, err := n.dockerClient.NetworkInspect(ctx, n.networkID, types.NetworkInspectOptions{})
	if err != nil {
		return fmt.Errorf("failed to inspect network: %w", err)
	}

	bootstrapIps, bootstrapIds := "", ""
	if len(n.options.Bootstrap) > 0 {
		for i := range n.options.Bootstrap {
			sep := ""
			if i > 0 {
				sep = ","
			}
			stakingAddr, err := n.options.Bootstrap[i].PublicStakingAddr(ctx)
			if err != nil {
				return fmt.Errorf("failed to get public staking address for index %d: %w", i, err)
			}
			bootstrapIps += sep + stakingAddr
			bootstrapIds += sep + n.options.Bootstrap[i].NodeId()
		}
	}

	trackSubnets := ""
	if len(n.options.Subnets) > 0 {
		for i := range n.options.Subnets {
			sep := ""
			if i > 0 {
				sep = ","
			}
			if n.options.Subnets[i].subnet != ids.Empty {
				trackSubnets += sep + n.options.Subnets[i].subnet.String()
			}
		}
	}

	cmd := []string{
		n.chain.cfg.Bin,
		"--http-host", "0.0.0.0",
		"--data-dir", n.HomeDir(),
		"--public-ip", n.options.PublicIP,
		"--network-id", n.options.ChainID.String(),
		"--genesis", filepath.Join(n.HomeDir(), "genesis.json"),
		"--staking-tls-cert-file", filepath.Join(n.HomeDir(), "tls.cert"),
		"--staking-tls-key-file", filepath.Join(n.HomeDir(), "tls.key"),
	}
	if bootstrapIps != "" && bootstrapIds != "" {
		cmd = append(
			cmd,
			"--bootstrap-ips", bootstrapIps,
			"--bootstrap-ids", bootstrapIds,
		)
	}
	if trackSubnets != "" {
		cmd = append(cmd, "--track-subnets", trackSubnets)
	}
	return n.containerLifecycle.CreateContainerInNetwork(
		ctx,
		n.testName,
		n.networkID,
		n.image,
		portBindings,
		n.Bind(),
		&network.NetworkingConfig{
			EndpointsConfig: map[string](*network.EndpointSettings){
				netinfo.Name: &network.EndpointSettings{
					NetworkID: netinfo.ID,
					IPAddress: n.options.PublicIP,
				},
			},
		},
		n.HostName(),
		cmd,
	)
}

func (n *AvalancheNode) StartContainer(ctx context.Context, testName string, additionalGenesisWallets []ibc.WalletAmount) error {
	return n.containerLifecycle.StartContainer(ctx)
}

func (n *AvalancheNode) StartSubnets(ctx context.Context) error {
	if len(n.options.Subnets) == 0 {
		return nil
	}

	kc := secp256k1fx.NewKeychain(n.options.Credentials.PK)
	ownerAddr := n.options.Credentials.PK.Address()

	wallet, err := primary.NewWalletFromURI(ctx, fmt.Sprintf("http://127.0.0.1:%s", n.RPCPort()), kc)
	if err != nil {
		return err
	}

	// Get the P-chain and the X-chain wallets
	pWallet := wallet.P()
	xWallet := wallet.X()

	// Pull out useful constants to use when issuing transactions.
	xChainID := xWallet.BlockchainID()
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{ownerAddr},
	}

	// Send AVAX to the P-chain.
	exportStartTime := time.Now()
	exportTxID, err := xWallet.IssueExportTx(
		constants.PlatformChainID,
		[]*avax.TransferableOutput{
			{
				Asset: avax.Asset{
					ID: xWallet.AVAXAssetID(),
				},
				Out: &secp256k1fx.TransferOutput{
					Amt:          2 * uint64(len(n.options.Subnets)+1) * pWallet.CreateSubnetTxFee(),
					OutputOwners: *owner,
				},
			},
		},
	)
	if err != nil {
		n.logger.Error(
			"failed to issue X->P export transaction",
			zap.Error(err),
		)
		return err
	}
	n.logger.Info(
		"issued X->P export",
		zap.String("exportTxID", exportTxID.String()),
		zap.Duration("duration", time.Since(exportStartTime)),
	)

	// Import AVAX from the X-chain into the P-chain.
	importStartTime := time.Now()
	importTxID, err := pWallet.IssueImportTx(xChainID, owner)
	if err != nil {
		n.logger.Error(
			"failed to issue X->P import transaction",
			zap.Error(err),
		)
		return err
	}
	n.logger.Info(
		"issued X->P import",
		zap.String("importTxID", importTxID.String()),
		zap.Duration("duration", time.Since(importStartTime)),
	)

	time.Sleep(2 * time.Second)

	for i, subnet := range n.options.Subnets {
		createSubnetStartTime := time.Now()
		createSubnetTxID, err := pWallet.IssueCreateSubnetTx(owner, common.WithContext(ctx), common.WithAssumeDecided())
		if err != nil {
			n.logger.Error(
				"failed to issue create subnet transaction",
				zap.Error(err),
				zap.String("name", subnet.Name),
			)
			return err
		}
		n.logger.Info(
			"issued create subnet transaction",
			zap.String("name", subnet.Name),
			zap.String("createSubnetTxID", createSubnetTxID.String()),
			zap.Duration("duration", time.Since(createSubnetStartTime)),
		)

		time.Sleep(4 * time.Second)

		startTime := time.Now().Add(20 * time.Second)
		duration := 2 * 7 * 24 * time.Hour // 2 weeks
		weight := units.Schmeckle
		addValidatorStartTime := time.Now()
		addValidatorTxID, err := pWallet.IssueAddSubnetValidatorTx(&txs.SubnetValidator{
			Validator: txs.Validator{
				NodeID: n.options.Credentials.ID,
				Start:  uint64(startTime.Unix()),
				End:    uint64(startTime.Add(duration).Unix()),
				Wght:   weight,
			},
			Subnet: createSubnetTxID,
		})
		if err != nil {
			n.logger.Error(
				"failed to issue add subnet validator transaction:",
				zap.Error(err),
				zap.String("name", subnet.Name),
			)
			return err
		}
		n.logger.Info(
			"added new subnet validator",
			zap.String("nodeID", n.options.Credentials.ID.String()),
			zap.String("subnetID", createSubnetTxID.String()),
			zap.String("addValidatorTxID", addValidatorTxID.String()),
			zap.Duration("duration", time.Since(addValidatorStartTime)),
		)

		time.Sleep(4 * time.Second)

		createChainStartTime := time.Now()
		createChainTxID, err := pWallet.IssueCreateChainTx(createSubnetTxID, subnet.Genesis, subnet.VmID, nil, subnet.Name)
		if err != nil {
			n.logger.Error(
				"failed to issue create chain transaction",
				zap.Error(err),
				zap.String("name", subnet.Name),
			)
			return err
		}
		n.logger.Info(
			"created new chain",
			zap.String("name", subnet.Name),
			zap.String("chainID", createChainTxID.String()),
			zap.Duration("duration", time.Since(createChainStartTime)),
		)

		n.options.Subnets[i].subnet = createSubnetTxID
		n.options.Subnets[i].chain = createChainTxID

		time.Sleep(30 * time.Second)
	}

	n.logger.Info("stopping container")
	if err := n.containerLifecycle.StopContainer(ctx); err != nil {
		return err
	}

	n.logger.Info("removing container")
	if err := n.containerLifecycle.RemoveContainer(ctx); err != nil {
		return err
	}

	n.logger.Info("creating new container")
	if err := n.CreateContainer(ctx); err != nil {
		return err
	}

	n.logger.Info("starting new container")
	if err := n.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	return lib.WaitNode(ctx, "127.0.0.1", n.RPCPort(), n.logger)
}

func (n *AvalancheNode) Start(ctx context.Context, testName string, additionalGenesisWallets []ibc.WalletAmount) error {
	err := n.StartContainer(ctx, testName, additionalGenesisWallets)
	if err != nil {
		return err
	}

	return lib.WaitNode(ctx, "127.0.0.1", n.RPCPort(), n.logger)
}
