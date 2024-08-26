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
	"github.com/avast/retry-go/v4"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	ethcommon "github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/sync/errgroup"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"

	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"

	"github.com/strangelove-ventures/interchaintest/v8/chain/avalanche/lib"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

const (
	rpcPort     = "9650/tcp"
	stakingPort = "9651/tcp"
)

var (
	portBindings = nat.PortMap{
		nat.Port(rpcPort):     {},
		nat.Port(stakingPort): {},
	}
)

type (
	AvalancheNode struct {
		chain *AvalancheChain

		logger             *zap.Logger
		containerLifecycle *dockerutil.ContainerLifecycle
		dockerClient       *dockerclient.Client
		image              ibc.DockerImage
		volume             volume.Volume

		networkID string
		testName  string
		index     int
		options   AvalancheNodeOpts
	}

	AvalancheNodes []*AvalancheNode

	AvalancheNodeCredentials struct {
		PK      *secp256k1.PrivateKey
		ID      ids.NodeID
		NodeIDs []ids.NodeID
		TLSCert []byte
		TLSKey  []byte
		BlsKey  []byte
	}

	AvalancheNodeSubnetOpts struct {
		Name      string
		VmID      ids.ID
		VM        []byte
		Genesis   []byte
		SCFactory ibc.AvalancheSubnetClientFactory

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

	// delete old volume
	err := dockerClient.VolumeRemove(ctx, name, true)
	if err != nil {
		return nil, err
	}

	vlm, err := dockerClient.VolumeCreate(ctx, volume.CreateOptions{
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

	node.volume = vlm

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

	if err := node.WriteFile(ctx, options.Credentials.BlsKey, "signer.key"); err != nil {
		return nil, fmt.Errorf("failed to write TLS key: %w", err)
	}

	if err := node.WriteFile(ctx, vmaliasesData, "configs/vms/aliases.json"); err != nil {
		return nil, fmt.Errorf("failed to write TLS key: %w", err)
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
	return info.HostConfig.PortBindings[nat.Port(stakingPort)][0].HostPort
}

func (n *AvalancheNode) RPCPort() string {
	info, err := n.dockerClient.ContainerInspect(context.Background(), n.containerLifecycle.ContainerID())
	if err != nil {
		panic(err)
	}
	return info.HostConfig.PortBindings[nat.Port(rpcPort)][0].HostPort
}

func (n *AvalancheNode) GRPCPort() string {
	panic(errors.New("doesn't support grpc"))
}

func (n *AvalancheNode) SubnetID() string {
	return n.options.Subnets[0].subnet.String()
}

func (n *AvalancheNode) BlockchainID() string {
	return n.options.Subnets[0].chain.String()
}

func (n *AvalancheNode) chainClient(id string) (ibc.AvalancheSubnetClient, error) {
	addr := fmt.Sprintf("http://127.0.0.1:%s", n.RPCPort())
	strpk := n.options.Credentials.PK.String()
	switch id {
	case "x", "p", "c":
		host := fmt.Sprintf("%s/ext/bc/%s", addr, strings.ToUpper(id))
		if id == "x" {
			return NewXChainClient(host, strpk)
		} else if id == "p" {
			return NewPChainClient(host, strpk)
		} else {
			return NewCChainClient(host, strpk)
		}
	default:
		subnetID, err := strconv.Atoi(id)
		if err != nil || subnetID >= len(n.options.Subnets) {
			return nil, fmt.Errorf("not valid subnet id: %w", err)
		}
		return n.options.Subnets[subnetID].SCFactory(
			fmt.Sprintf("%s/ext/bc/%s", addr, n.options.Subnets[subnetID].chain),
			strpk,
		)
	}
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
	return ethcommon.FromHex("0x8db97C7cEcE249c2b98bDC0226Cc4C2A57BF52FC"), nil
}

func (n *AvalancheNode) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	rawSubnetID, ok := ctx.Value("subnet").(string)
	rawSubnetID = strings.ToLower(rawSubnetID)
	if !ok {
		if strings.HasPrefix(amount.Address, "X-") {
			rawSubnetID = "x"
		} else if strings.HasPrefix(amount.Address, "P-") {
			rawSubnetID = "p"
		} else if strings.HasPrefix(amount.Address, "0x") {
			rawSubnetID = "c"
		} else {
			return fmt.Errorf("address have uknown format: %s", amount.Address)
		}
		rawSubnetID = "x"
	}
	n.logger.Info("create client", zap.String("subnet", rawSubnetID))
	client, err := n.chainClient(rawSubnetID)
	if err != nil {
		return fmt.Errorf("subnet client creation error: %w", err)
	}
	return client.SendFunds(ctx, keyName, amount)
}

func (n *AvalancheNode) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	pkey, err := crypto.HexToECDSA(keyName)
	if err != nil {
		return ibc.Tx{}, err
	}
	addr := crypto.PubkeyToAddress(pkey.PublicKey)

	packetData, _ := json.Marshal(FungibleTokenPacketData{
		Denom:    amount.Denom,
		Amount:   amount.Amount.String(),
		Sender:   addr.String(),
		Receiver: amount.Address,
	})

	msg, err := packSendPacket(MsgSendPacket{
		ChannelCapability: big.NewInt(0),
		SourcePort:        "transfer",
		SourceChannel:     channelID,
		TimeoutHeight: Height{
			RevisionHeight: big.NewInt(3000),
			RevisionNumber: big.NewInt(3000),
		},
		TimeoutTimestamp: big.NewInt(0),
		Data:             packetData,
	})
	if err != nil {
		return ibc.Tx{}, err
	}

	rpcUrl := fmt.Sprintf("http://127.0.0.1:%s/ext/bc/%s/rpc", n.RPCPort(), n.BlockchainID())

	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return ibc.Tx{}, err
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return ibc.Tx{}, err
	}
	nonce, err := client.NonceAt(context.Background(), addr, nil)
	if err != nil {
		return ibc.Tx{}, err
	}
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return ibc.Tx{}, err
	}

	value := big.NewInt(0)
	tx := ethtypes.NewTransaction(nonce, AvalancheIBCPrecompileAddress, value, 2100000, gasPrice, msg)

	signedTx, err := ethtypes.SignTx(tx, ethtypes.NewEIP155Signer(chainID), pkey)
	if err != nil {
		return ibc.Tx{}, err
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return ibc.Tx{}, err
	}

	receipt, err := bind.WaitMined(context.Background(), client, signedTx)
	if err != nil {
		return ibc.Tx{}, err
	}

	return ibc.Tx{
		Height:   receipt.BlockNumber.Int64(),
		TxHash:   signedTx.Hash().String(),
		GasSpent: int64(receipt.GasUsed),
		Packet: ibc.Packet{
			Sequence:         0,
			SourcePort:       "",
			SourceChannel:    "",
			DestPort:         "",
			DestChannel:      "",
			Data:             nil,
			TimeoutHeight:    "",
			TimeoutTimestamp: 0,
		},
	}, nil
}

func (n *AvalancheNode) Height(ctx context.Context) (uint64, error) {
	rawSubnetID, ok := ctx.Value("subnet").(string)
	rawSubnetID = strings.ToLower(rawSubnetID)
	if !ok {
		return platformvm.NewClient(fmt.Sprintf("http://127.0.0.1:%s", n.RPCPort())).GetHeight(ctx)
	}
	client, err := n.chainClient(rawSubnetID)
	if err != nil {
		return 0, fmt.Errorf("subnet client creation error: %w", err)
	}
	return client.Height(ctx)
}

func (n *AvalancheNode) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
	rawSubnetID, ok := ctx.Value("subnet").(string)
	rawSubnetID = strings.ToLower(rawSubnetID)
	if !ok {
		if strings.HasPrefix(address, "X-") {
			rawSubnetID = "x"
		} else if strings.HasPrefix(address, "P-") {
			rawSubnetID = "p"
		} else if strings.HasPrefix(address, "0x") {
			rawSubnetID = "c"
		} else {
			return 0, fmt.Errorf("address have uknown format: %s", address)
		}
	}
	client, err := n.chainClient(rawSubnetID)
	if err != nil {
		return 0, fmt.Errorf("subnet client creation error: %w", err)
	}
	return client.GetBalance(ctx, address)
}

func (n *AvalancheNode) GetBankBalance(ctx context.Context, bank string, address string, denom string) (int64, error) {
	rawSubnetID, ok := ctx.Value("subnet").(string)
	rawSubnetID = strings.ToLower(rawSubnetID)
	if !ok {
		return 0, fmt.Errorf("bank balance available only for subnet: %s", address)
	}
	client, err := n.chainClient(rawSubnetID)
	if err != nil {
		return 0, fmt.Errorf("subnet client creation error: %w", err)
	}

	return client.GetBankBalance(ctx, bank, address, denom)
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
		"--http-allowed-hosts", "*",
		"--data-dir", n.HomeDir(),
		"--public-ip", n.options.PublicIP,
		"--network-id", n.options.ChainID.String(),
		"--genesis-file", filepath.Join(n.HomeDir(), "genesis.json"),
		"--staking-tls-cert-file", filepath.Join(n.HomeDir(), "tls.cert"),
		"--staking-tls-key-file", filepath.Join(n.HomeDir(), "tls.key"),
		"--staking-signer-key-file", filepath.Join(n.HomeDir(), "signer.key"),
		"--plugin-dir", "/avalanchego/build/plugins",
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
		n.image,
		portBindings,
		n.Bind(),
		nil,
		n.HostName(),
		cmd,
		n.chain.cfg.Env,
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				netinfo.Name: {
					NetworkID: netinfo.ID,
					IPAddress: n.options.PublicIP,
					IPAMConfig: &network.EndpointIPAMConfig{
						IPv4Address: n.options.PublicIP,
					},
				},
			},
		},
	)
}

func (n *AvalancheNode) StartContainer(ctx context.Context, testName string, additionalGenesisWallets []ibc.WalletAmount) error {
	return n.containerLifecycle.StartContainer(ctx)
}

func (n *AvalancheNode) createSubnetChain(ctx context.Context) (string, string, error) {
	if len(n.options.Subnets) == 0 {
		return "", "", nil
	}

	kc := secp256k1fx.NewKeychain(n.options.Credentials.PK)
	ownerAddr := n.options.Credentials.PK.Address()

	wallet, err := primary.MakeWallet(ctx, &primary.WalletConfig{
		URI:          fmt.Sprintf("http://127.0.0.1:%s", n.RPCPort()),
		AVAXKeychain: kc,
		EthKeychain:  kc,
	})
	if err != nil {
		return "", "", err
	}

	// Get the P-chain and the X-chain wallets
	pWallet := wallet.P()
	xWallet := wallet.X()

	xBuilder := xWallet.Builder()
	xContext := xBuilder.Context()

	// Pull out useful constants to use when issuing transactions.
	xChainID := xContext.BlockchainID
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs:     []ids.ShortID{ownerAddr},
	}

	// Send AVAX to the P-chain.
	exportStartTime := time.Now()
	exportTx, err := xWallet.IssueExportTx(
		constants.PlatformChainID,
		[]*avax.TransferableOutput{
			{
				Asset: avax.Asset{
					ID: xContext.AVAXAssetID,
				},
				Out: &secp256k1fx.TransferOutput{
					Amt:          2 * uint64(len(n.options.Subnets)+1) * pWallet.Builder().Context().CreateSubnetTxFee,
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
		return "", "", err
	}

	n.logger.Info(
		"issued X->P export",
		zap.String("exportTxID", exportTx.ID().String()),
		zap.Duration("duration", time.Since(exportStartTime)),
	)

	// Import AVAX from the X-chain into the P-chain.
	importStartTime := time.Now()
	importTx, err := pWallet.IssueImportTx(xChainID, owner)
	if err != nil {
		n.logger.Error(
			"failed to issue X->P import transaction",
			zap.Error(err),
		)
		return "", "", err
	}
	n.logger.Info(
		"issued X->P import",
		zap.String("importTxID", importTx.ID().String()),
		zap.Duration("duration", time.Since(importStartTime)),
	)

	time.Sleep(2 * time.Second)

	var chainID string
	var subnetID string

	for i, subnet := range n.options.Subnets {
		createSubnetStartTime := time.Now()
		createSubnetTx, err := pWallet.IssueCreateSubnetTx(owner, common.WithContext(ctx), common.WithAssumeDecided())
		if err != nil {
			n.logger.Error(
				"failed to issue create subnet transaction",
				zap.Error(err),
				zap.String("name", subnet.Name),
			)
			return "", "", err
		}
		n.logger.Info(
			"issued create subnet transaction",
			zap.String("name", subnet.Name),
			zap.String("createSubnetTxID", createSubnetTx.ID().String()),
			zap.Duration("duration", time.Since(createSubnetStartTime)),
		)

		time.Sleep(4 * time.Second)

		startTime := time.Now().Add(20 * time.Second)
		duration := 2 * 7 * 24 * time.Hour // 2 weeks
		weight := units.Schmeckle
		addValidatorStartTime := time.Now()

		// add all nodes as a validator
		for _, nodeID := range n.options.Credentials.NodeIDs {
			addValidatorTx, err := pWallet.IssueAddSubnetValidatorTx(&txs.SubnetValidator{
				Validator: txs.Validator{
					NodeID: nodeID,
					Start:  uint64(startTime.Unix()),
					End:    uint64(startTime.Add(duration).Unix()),
					Wght:   weight,
				},
				Subnet: createSubnetTx.ID(),
			})
			if err != nil {
				n.logger.Error(
					"failed to issue add subnet validator transaction:",
					zap.Error(err),
					zap.String("name", subnet.Name),
					zap.String("nodeID", nodeID.String()),
				)
				return "", "", err
			}
			n.logger.Info(
				"added new subnet validator",
				zap.String("nodeID", nodeID.String()),
				zap.String("subnetID", createSubnetTx.ID().String()),
				zap.String("addValidatorTxID", addValidatorTx.ID().String()),
				zap.Duration("duration", time.Since(addValidatorStartTime)),
			)

			time.Sleep(4 * time.Second)
		}

		createChainStartTime := time.Now()
		createChainTx, err := pWallet.IssueCreateChainTx(createSubnetTx.ID(), subnet.Genesis, subnet.VmID, nil, subnet.Name)
		if err != nil {
			n.logger.Error(
				"failed to issue create chain transaction",
				zap.Error(err),
				zap.String("name", subnet.Name),
			)
			return "", "", err
		}

		chainID = createChainTx.ID().String()
		subnetID = createSubnetTx.ID().String()

		n.logger.Info(
			"created new chain",
			zap.String("name", subnet.Name),
			zap.String("chainID", chainID),
			zap.Duration("duration", time.Since(createChainStartTime)),
		)

		n.options.Subnets[i].subnet = createSubnetTx.ID()
		n.options.Subnets[i].chain = createChainTx.ID()

		time.Sleep(30 * time.Second)
	}

	return subnetID, chainID, nil
}

func (n *AvalancheNode) StartSubnets(ctx context.Context, nodes AvalancheNodes) error {
	_, chainID, err := n.createSubnetChain(ctx)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		// enable Warp API
		node.WriteFile(ctx, []byte(`{"warp-api-enabled": true}`), fmt.Sprintf("configs/chains/%s/config.json", chainID))

		n.logger.Info("stopping container", zap.Int("index", node.index), zap.String("NodeID", node.NodeId()))
		if err := node.containerLifecycle.StopContainer(ctx); err != nil {
			return err
		}

		n.logger.Info("removing container", zap.Int("index", node.index), zap.String("NodeID", node.NodeId()))
		if err := node.containerLifecycle.RemoveContainer(ctx); err != nil {
			return err
		}

		n.logger.Info("creating new container", zap.Int("index", node.index), zap.String("NodeID", node.NodeId()))
		if err := node.CreateContainer(ctx); err != nil {
			return fmt.Errorf("failed to re-create container for subnet", err)
		}

		n.logger.Info("starting new container", zap.Int("index", node.index), zap.String("NodeID", node.NodeId()))
		if err := node.containerLifecycle.StartContainer(ctx); err != nil {
			return fmt.Errorf("failed to start container for subnet", err)
		}
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for _, node := range nodes {
		node := node
		eg.Go(func() error {
			tCtx, tCtxCancel := context.WithTimeout(egCtx, ChainBootstrapTimeout)
			defer tCtxCancel()

			return lib.WaitNode(tCtx, "127.0.0.1", node.RPCPort(), n.logger, node.index, chainID)
		})
	}
	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to start avalanche nodes %w", err)
	}

	return nil
}

func (n *AvalancheNode) Start(ctx context.Context, testName string, additionalGenesisWallets []ibc.WalletAmount) error {
	err := retry.Do(func() error {
		return n.StartContainer(ctx, testName, additionalGenesisWallets)
	},
		// retry for total of 3 seconds
		retry.Attempts(15),
		retry.Delay(2*time.Second),
		retry.DelayType(retry.FixedDelay),
		retry.LastErrorOnly(true),
	)

	return err
}

func (n *AvalancheNode) WaitNode(ctx context.Context) error {
	return lib.WaitNode(ctx, "127.0.0.1", n.RPCPort(), n.logger, n.index, "")
}
