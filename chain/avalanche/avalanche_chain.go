package avalanche

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/docker/docker/api/types"
	"github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/docker/docker/client"

	"github.com/strangelove-ventures/interchaintest/v8/chain/avalanche/lib"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var (
	_                     ibc.Chain = &AvalancheChain{}
	ChainBootstrapTimeout           = 6 * time.Minute
)

type AvalancheChain struct {
	log           *zap.Logger
	testName      string
	cfg           ibc.ChainConfig
	numValidators int
	numFullNodes  int
	nodes         AvalancheNodes
}

func NewAvalancheChain(log *zap.Logger, testName string, chainConfig ibc.ChainConfig, numValidators int, numFullNodes int) (*AvalancheChain, error) {
	if numValidators < 5 {
		return nil, fmt.Errorf("numValidators must be more or equal 5, have: %d", numValidators)
	}
	return &AvalancheChain{
		log:           log,
		testName:      testName,
		cfg:           chainConfig,
		numValidators: numValidators,
		numFullNodes:  numFullNodes,
	}, nil
}

func (c *AvalancheChain) node() *AvalancheNode {
	if len(c.nodes) > c.numValidators {
		return c.nodes[c.numValidators]
	}

	if len(c.nodes) > 1 {
		return c.nodes[1]
	}
	return c.nodes[0]
}

// Config fetches the chain configuration.
func (c *AvalancheChain) Config() ibc.ChainConfig {
	return c.cfg
}

// Initialize initializes node structs so that things like initializing keys can be done before starting the chain
func (c *AvalancheChain) Initialize(ctx context.Context, testName string, cli *client.Client, networkID string) error {
	for _, image := range c.Config().Images {
		rc, err := cli.ImagePull(
			ctx,
			image.Repository+":"+image.Version,
			types.ImagePullOptions{},
		)
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

	rawChainID := c.Config().ChainID
	if rawChainID == "" {
		rawChainID = "localnet-123456"
	}
	chainId, err := lib.ParseChainID(rawChainID)
	if err != nil {
		c.log.Error("Failed to parse chain ID",
			zap.Error(err),
			zap.String("networkID", networkID),
			zap.String("chainID", rawChainID),
		)
		return err
	}

	var subnetOpts []AvalancheNodeSubnetOpts = nil
	if len(c.cfg.AvalancheSubnets) > 0 {
		subnetOpts = make([]AvalancheNodeSubnetOpts, len(c.cfg.AvalancheSubnets))
		for i := range c.cfg.AvalancheSubnets {
			subnetOpts[i].Name = c.cfg.AvalancheSubnets[i].Name
			subnetOpts[i].VM = c.cfg.AvalancheSubnets[i].VM
			subnetOpts[i].Genesis = c.cfg.AvalancheSubnets[i].Genesis
			subnetOpts[i].SCFactory = c.cfg.AvalancheSubnets[i].SubnetClientFactory
			vmName := make([]byte, 32)
			copy(vmName[:], []byte(c.cfg.AvalancheSubnets[i].Name))
			subnetOpts[i].VmID, err = ids.ToID(vmName)
			if err != nil {
				return err
			}
		}
	}

	key, err := secp256k1.NewPrivateKey()
	if err != nil {
		return err
	}

	numNodes := c.numValidators + c.numFullNodes
	credentials := make([]AvalancheNodeCredentials, numNodes)
	for i := 0; i < numNodes; i++ {
		rawTlsCert, rawTlsKey, err := staking.NewCertAndKeyBytes()
		if err != nil {
			return err
		}

		cert, err := staking.LoadTLSCertFromBytes(rawTlsKey, rawTlsCert)
		if err != nil {
			return err
		}

		//stakingCert, err := staking.ParseCertificate(cert.Leaf)
		//if err != nil {
		//	return nil, fmt.Errorf("invalid staking certificate: %w", err)
		//}

		//tlsCert, err := staking.NewTLSCert()
		//require.NoError(err)
		//cert, err := staking.ParseCertificate(tlsCert.Leaf.Raw)
		//require.NoError(err)
		//nodeID := ids.NodeIDFromCert(cert)

		credentials[i].PK = key
		credentials[i].ID = ids.NodeIDFromCert(&staking.Certificate{Raw: cert.Leaf.Raw})
		credentials[i].TLSCert = rawTlsCert
		credentials[i].TLSKey = rawTlsKey
	}

	avaxAddr, _ := address.Format("X", chainId.Name, key.Address().Bytes())
	ethAddr := crypto.PubkeyToAddress(key.ToECDSA().PublicKey).Hex()
	allocations := []GenesisAllocation{
		{
			ETHAddr:        ethAddr,
			AVAXAddr:       avaxAddr,
			InitialAmount:  4000000000,
			UnlockSchedule: []GenesisLockedAmount{{Amount: 2000000000}, {Amount: 1000000000}},
		},
		{
			ETHAddr:        ethAddr,
			AVAXAddr:       avaxAddr,
			InitialAmount:  4000000000,
			UnlockSchedule: []GenesisLockedAmount{{Amount: 2000000000}, {Amount: 1000000000}},
		},
		{
			ETHAddr:        ethAddr,
			AVAXAddr:       avaxAddr,
			InitialAmount:  4000000000,
			UnlockSchedule: []GenesisLockedAmount{},
		},
		{
			ETHAddr:        ethAddr,
			AVAXAddr:       avaxAddr,
			InitialAmount:  4000000000,
			UnlockSchedule: []GenesisLockedAmount{},
		},
		{
			ETHAddr:        ethAddr,
			AVAXAddr:       avaxAddr,
			InitialAmount:  4000000000,
			UnlockSchedule: []GenesisLockedAmount{{Amount: 4000000000, Locktime: uint32(time.Second)}},
		},
		{
			ETHAddr:        ethAddr,
			AVAXAddr:       avaxAddr,
			InitialAmount:  4000000000,
			UnlockSchedule: []GenesisLockedAmount{{Amount: 4000000000, Locktime: uint32(time.Second)}},
		},
	}
	stakedFunds := make([]string, 0, c.numValidators)
	stakes := make([]GenesisStaker, 0, c.numValidators)
	for i := 0; i < c.numValidators; i++ {
		stakes = append(stakes, GenesisStaker{
			NodeID:        credentials[i].ID.String(),
			RewardAddress: avaxAddr,
			DelegationFee: 100000000,
		})
	}
	stakedFunds = append(stakedFunds, avaxAddr)
	genesis := NewGenesis(chainId.Number, allocations, stakedFunds, stakes)
	nodes := make(AvalancheNodes, 0, numNodes)
	for i := 0; i < numNodes; i++ {
		var bootstrapOpt []*AvalancheNode = nil
		if i > 0 {
			bootstrapOpt = []*AvalancheNode{nodes[0]}
		}
		ip, err := getIP(ctx, cli, networkID, uint8(i+1))
		if err != nil {
			return err
		}
		n, err := NewAvalancheNode(ctx, c, networkID, testName, cli, c.Config().Images[0], i, c.log, genesis, &AvalancheNodeOpts{
			PublicIP:    ip,
			Bootstrap:   bootstrapOpt,
			Subnets:     subnetOpts,
			Credentials: credentials[i],
			ChainID:     *chainId,
		})
		if err != nil {
			return err
		}
		nodes = append(nodes, n)
	}
	c.nodes = nodes
	return nil
}

// Start sets up everything needed (validators, gentx, fullnodes, peering, additional accounts) for chain to start from genesis.
func (c *AvalancheChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	eg, egCtx := errgroup.WithContext(ctx)
	for _, node := range c.nodes {
		node := node
		eg.Go(func() error {
			tCtx, tCtxCancel := context.WithTimeout(egCtx, ChainBootstrapTimeout)
			defer tCtxCancel()

			return node.Start(tCtx, testName, additionalGenesisWallets)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	return c.node().StartSubnets(ctx)
}

// Exec runs an arbitrary command using Chain's docker environment.
// Whether the invoked command is run in a one-off container or execing into an already running container
// is up to the chain implementation.
//
// "env" are environment variables in the format "MY_ENV_VAR=value"
func (c *AvalancheChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	return c.node().Exec(ctx, cmd, env)
}

// ExportState exports the chain state at specific height.
func (c *AvalancheChain) ExportState(ctx context.Context, height int64) (string, error) {
	panic("ToDo: implement me")
}

// GetRPCAddress retrieves the rpc address that can be reached by other containers in the docker network.
func (c *AvalancheChain) GetRPCAddress() string {
	return fmt.Sprintf("http://%s:%s", c.node().HostName(), c.node().RPCPort())
}

// GetGRPCAddress retrieves the grpc address that can be reached by other containers in the docker network.
func (c *AvalancheChain) GetGRPCAddress() string {
	return fmt.Sprintf("http://%s:%s", c.node().HostName(), c.node().GRPCPort())
}

// GetHostRPCAddress returns the rpc address that can be reached by processes on the host machine.
// Note that this will not return a valid value until after Start returns.
func (c *AvalancheChain) GetHostRPCAddress() string {
	return fmt.Sprintf("http://127.0.0.1:%s", c.node().RPCPort())
}

// GetHostGRPCAddress returns the grpc address that can be reached by processes on the host machine.
// Note that this will not return a valid value until after Start returns.
func (c *AvalancheChain) GetHostGRPCAddress() string {
	panic("ToDo: implement me")
}

// HomeDir is the home directory of a node running in a docker container. Therefore, this maps to
// the container's filesystem (not the host).
func (c *AvalancheChain) HomeDir() string {
	panic("ToDo: implement me")
}

// CreateKey creates a test key in the "user" node (either the first fullnode or the first validator if no fullnodes).
func (c *AvalancheChain) CreateKey(ctx context.Context, keyName string) error {
	return c.node().CreateKey(ctx, keyName)
}

// RecoverKey recovers an existing user from a given mnemonic.
func (c *AvalancheChain) RecoverKey(ctx context.Context, name, mnemonic string) error {
	return c.node().RecoverKey(ctx, name, mnemonic)
}

// GetAddress fetches the bech32 address for a test key on the "user" node (either the first fullnode or the first validator if no fullnodes).
func (c *AvalancheChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	return c.node().GetAddress(ctx, keyName)
}

// SendFunds sends funds to a wallet from a user account.
func (c *AvalancheChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	return c.node().SendFunds(ctx, keyName, amount)
}

// SendIBCTransfer sends an IBC transfer returning a transaction or an error if the transfer failed.
func (c *AvalancheChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	return c.node().SendIBCTransfer(ctx, channelID, keyName, amount, options)
}

// Height returns the current block height or an error if unable to get current height.
func (c *AvalancheChain) Height(ctx context.Context) (int64, error) {
	height, err := c.node().Height(ctx)
	if err != nil {
		return 0, err
	}

	return int64(height), nil
}

// GetBalance fetches the current balance for a specific account address and denom.
func (c *AvalancheChain) GetBalance(ctx context.Context, address string, denom string) (sdkmath.Int, error) {
	balance, err := c.node().GetBalance(ctx, address, denom)
	if err != nil {
		return sdkmath.ZeroInt(), err
	}

	return sdkmath.NewInt(balance), nil
}

// GetGasFeesInNativeDenom gets the fees in native denom for an amount of spent gas.
func (c *AvalancheChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	// ToDo: ask how to calculate???
	panic("ToDo: implement me")
}

// Acknowledgements returns all acknowledgements in a block at height.
func (c *AvalancheChain) Acknowledgements(ctx context.Context, height int64) ([]ibc.PacketAcknowledgement, error) {
	panic("ToDo: implement me")
}

// Timeouts returns all timeouts in a block at height.
func (c *AvalancheChain) Timeouts(ctx context.Context, height int64) ([]ibc.PacketTimeout, error) {
	panic("ToDo: implement me")
}

// BuildWallet will return a chain-specific wallet
// If mnemonic != "", it will restore using that mnemonic
// If mnemonic == "", it will create a new key, mnemonic will not be populated
func (c *AvalancheChain) BuildWallet(ctx context.Context, keyName string, mnemonic string) (ibc.Wallet, error) {
	if mnemonic != "" {
		if err := c.RecoverKey(ctx, keyName, mnemonic); err != nil {
			return nil, fmt.Errorf("failed to recover key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
	} else {
		if err := c.CreateKey(ctx, keyName); err != nil {
			return nil, fmt.Errorf("failed to create key with name %q on chain %s: %w", keyName, c.cfg.Name, err)
		}
	}

	addrBytes, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, c.cfg.Name, err)
	}

	return NewWallet(keyName, addrBytes, mnemonic, c.cfg), nil
}

// BuildRelayerWallet will return a chain-specific wallet populated with the mnemonic so that the wallet can
// be restored in the relayer node using the mnemonic. After it is built, that address is included in
// genesis with some funds.
func (c *AvalancheChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	// ToDo: build wallet for relayer once relayer has avalanche key management.
	panic("ToDo: implement me")
}

func (c *AvalancheChain) GetHostPeerAddress() string {
	//TODO implement me
	panic("implement me")
}

func getIP(ctx context.Context, cli *client.Client, networkID string, idx uint8) (string, error) {
	network, err := cli.NetworkInspect(ctx, networkID, types.NetworkInspectOptions{})
	if err != nil {
		return "", err
	}
	ip := net.ParseIP(network.IPAM.Config[0].Gateway)
	ip = ip.To4()
	ip[3] += idx
	return ip.String(), nil
}
