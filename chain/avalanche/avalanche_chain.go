package avalanche

import (
	"context"
	"fmt"
	"io"
	big "math/big"
	"net"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/staking"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/vms/platformvm/signer"
	"github.com/docker/docker/api/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/docker/docker/client"

	"github.com/strangelove-ventures/interchaintest/v8/chain/avalanche/ics20/ics20bank"
	"github.com/strangelove-ventures/interchaintest/v8/chain/avalanche/ics20/ics20transferer"
	"github.com/strangelove-ventures/interchaintest/v8/chain/avalanche/lib"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

var (
	_                             ibc.Chain = &AvalancheChain{}
	ChainBootstrapTimeout                   = 6 * time.Minute
	AvalancheIBCPrecompileAddress           = common.HexToAddress("0x0300000000000000000000000000000000000002")
)

type AvalancheChain struct {
	log                 *zap.Logger
	testName            string
	cfg                 ibc.ChainConfig
	numValidators       int
	numFullNodes        int
	nodes               AvalancheNodes
	ChainID             uint32
	bankContractAddress common.Address
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

func (c *AvalancheChain) Node() *AvalancheNode {
	if len(c.nodes) > c.numValidators {
		return c.nodes[c.numValidators]
	}

	if len(c.nodes) > 1 {
		return c.nodes[1]
	}
	return c.nodes[0]
}

// GetDefaultChainURI returns the default chain URI for a given blockchainID
func (c *AvalancheChain) GetDefaultChainURI(blockchainID string) string {
	return fmt.Sprintf("%s/ext/bc/%s/rpc", c.GetRPCAddress(), blockchainID)
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

	c.ChainID = chainId.Number

	var subnetOpts []AvalancheNodeSubnetOpts = nil
	if len(c.cfg.AvalancheSubnets) > 0 {
		subnetOpts = make([]AvalancheNodeSubnetOpts, len(c.cfg.AvalancheSubnets))
		for i := range c.cfg.AvalancheSubnets {
			subnetOpts[i].Name = c.cfg.AvalancheSubnets[i].Name
			//subnetOpts[i].VM = c.cfg.AvalancheSubnets[i].VM
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
	nodeIds := make([]ids.NodeID, numNodes)

	for i := 0; i < numNodes; i++ {
		rawTlsCert, rawTlsKey, err := staking.NewCertAndKeyBytes()
		if err != nil {
			return err
		}

		cert, err := staking.LoadTLSCertFromBytes(rawTlsKey, rawTlsCert)
		if err != nil {
			return err
		}

		blsKey, err := bls.NewSecretKey()
		if err != nil {
			return fmt.Errorf("couldn't generate new signing key: %w", err)
		}
		blsKeyBytes := bls.SecretKeyToBytes(blsKey)

		nodeID := ids.NodeIDFromCert(&staking.Certificate{Raw: cert.Leaf.Raw})

		credentials[i].PK = key
		credentials[i].ID = nodeID
		credentials[i].TLSCert = rawTlsCert
		credentials[i].TLSKey = rawTlsKey
		credentials[i].BlsKey = blsKeyBytes

		nodeIds[i] = nodeID
	}

	for i := 0; i < numNodes; i++ {
		credentials[i].NodeIDs = nodeIds
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

		blsSk, err := bls.SecretKeyFromBytes(credentials[i].BlsKey)
		if err != nil {
			return err
		}
		proofOfPossession := signer.NewProofOfPossession(blsSk)
		pk, err := formatting.Encode(formatting.HexNC, proofOfPossession.PublicKey[:])
		if err != nil {
			return err
		}
		pop, err := formatting.Encode(formatting.HexNC, proofOfPossession.ProofOfPossession[:])
		if err != nil {
			return err
		}

		stakes = append(stakes, GenesisStaker{
			NodeID:        credentials[i].ID.String(),
			RewardAddress: avaxAddr,
			DelegationFee: 100000000,
			Signer: StakerSigner{
				PublicKey:         pk,
				ProofOfPossession: pop,
			},
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
		err := node.Start(ctx, testName, additionalGenesisWallets)
		if err != nil {
			return err
		}
	}

	for _, node := range c.nodes {
		node := node
		eg.Go(func() error {
			tCtx, tCtxCancel := context.WithTimeout(egCtx, ChainBootstrapTimeout)
			defer tCtxCancel()

			return node.WaitNode(tCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return fmt.Errorf("failed to start avalanche nodes %w", err)
	}

	if err := c.Node().StartSubnets(ctx, c.nodes); err != nil {
		return err
	}

	rpcUrl := fmt.Sprintf("http://127.0.0.1:%s/ext/bc/%s/rpc", c.Node().RPCPort(), c.Node().BlockchainID())

	if err := c.deployBankContract(ctx, rpcUrl); err != nil {
		return fmt.Errorf("failed to deploy ICS20 Bank smart contract on avalanche %w", err)
	}

	go func() {
		for {
			select {
			case <-time.After(10 * time.Second):

				if err := c.doTx(rpcUrl); err != nil {
					c.log.Error("tx error", zap.Error(err))
				}
			}
		}
	}()

	return nil
}

func (c *AvalancheChain) deployBankContract(ctx context.Context, rpcUrl string) error {
	pkey, err := crypto.HexToECDSA("56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027")
	if err != nil {
		return err
	}

	client, err := ethclient.Dial(rpcUrl)
	if err != nil {
		return err
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return err
	}

	auth, err := bind.NewKeyedTransactorWithChainID(pkey, chainID)
	if err != nil {
		return err
	}

	_, ics20bankTx, ics20bank, err := ics20bank.DeployICS20Bank(auth, client)
	if err != nil {
		return err
	}

	ics20bankAddr, err := bind.WaitDeployed(ctx, client, ics20bankTx)
	if err != nil {
		return err
	}
	c.log.Info("ICS20 Bank delpoyed", zap.String("address", ics20bankAddr.Hex()))
	c.bankContractAddress = ics20bankAddr

	_, ics20transfererTx, ics20transferer, err := ics20transferer.DeployICS20Transferer(auth, client, AvalancheIBCPrecompileAddress, ics20bankAddr)
	if err != nil {
		return err
	}

	ics20transfererAddr, err := bind.WaitDeployed(ctx, client, ics20transfererTx)
	if err != nil {
		return err
	}
	c.log.Info("ICS20 Transferer delpoyed", zap.String("address", ics20transfererAddr.Hex()))

	setOperTx1, err := ics20bank.SetOperator(auth, auth.From)
	if err != nil {
		return err
	}
	setOperRe1, err := bind.WaitMined(ctx, client, setOperTx1)
	if err != nil {
		return err
	}
	c.log.Info("SetOperator key address", zap.String("hash", setOperRe1.TxHash.Hex()), zap.String("block", setOperRe1.BlockNumber.String()))

	setOperTx2, err := ics20bank.SetOperator(auth, AvalancheIBCPrecompileAddress)
	if err != nil {
		return err
	}
	setOperRe2, err := bind.WaitMined(ctx, client, setOperTx2)
	if err != nil {
		return err
	}
	c.log.Info("SetOperator ibc address", zap.String("hash", setOperRe2.TxHash.Hex()), zap.String("block", setOperRe2.BlockNumber.String()))

	setOperTx3, err := ics20bank.SetOperator(auth, ics20transfererAddr)
	if err != nil {
		return err
	}
	setOperRe3, err := bind.WaitMined(ctx, client, setOperTx3)
	if err != nil {
		return err
	}
	c.log.Info("SetOperator ics20 transferer address", zap.String("hash", setOperRe3.TxHash.Hex()), zap.String("block", setOperRe3.BlockNumber.String()))

	setChannelEscrowAddressesTx, err := ics20transferer.SetChannelEscrowAddresses(auth, "transfer", auth.From)
	if err != nil {
		return err
	}
	setChannelEscrowAddressesRe, err := bind.WaitMined(ctx, client, setChannelEscrowAddressesTx)
	if err != nil {
		return err
	}
	c.log.Info("ics20transferer.SetChannelEscrowAddresses", zap.String("addr", auth.From.Hex()), zap.String("port", "transfer"), zap.String("block", setChannelEscrowAddressesRe.BlockNumber.String()))

	bintPortTx, err := ics20transferer.BindPort(auth, AvalancheIBCPrecompileAddress, "transfer")
	if err != nil {
		return err
	}
	bintPortRe, err := bind.WaitMined(ctx, client, bintPortTx)
	if err != nil {
		return err
	}
	c.log.Info("ics20transferer.BindPort", zap.String("addr", AvalancheIBCPrecompileAddress.Hex()), zap.String("port", "transfer"), zap.String("block", bintPortRe.BlockNumber.String()))

	return nil
}

func (c *AvalancheChain) doTx(url string) error {
	pkey, err := crypto.HexToECDSA("56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027")
	if err != nil {
		return err
	}
	addr := crypto.PubkeyToAddress(pkey.PublicKey)

	client, err := ethclient.Dial(url)
	if err != nil {
		return err
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return err
	}

	nonce, err := client.NonceAt(context.Background(), addr, nil)
	if err != nil {
		return err
	}

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return err
	}

	toAddress := common.HexToAddress("0xAB259A4830E2C7AA6EF3831BAC1590F855AE4C32")
	value := big.NewInt(1000000000000000000)
	tx := ethtypes.NewTransaction(nonce, toAddress, value, 21000, gasPrice, nil)

	signedTx, err := ethtypes.SignTx(tx, ethtypes.NewEIP155Signer(chainID), pkey)
	if err != nil {
		return err
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return err
	}
	_, err = bind.WaitMined(context.Background(), client, signedTx)
	if err != nil {
		return err
	}
	//c.log.Info("transaction mined",
	//	zap.String("hash", receipt.TxHash.Hex()),
	//	zap.String("block", receipt.BlockNumber.String()),
	//)

	return nil
}

// Exec runs an arbitrary command using Chain's docker environment.
// Whether the invoked command is run in a one-off container or execing into an already running container
// is up to the chain implementation.
//
// "env" are environment variables in the format "MY_ENV_VAR=value"
func (c *AvalancheChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	return c.Node().Exec(ctx, cmd, env)
}

// ExportState exports the chain state at specific height.
func (c *AvalancheChain) ExportState(ctx context.Context, height int64) (string, error) {
	panic("ToDo: implement me")
}

// GetRPCAddress retrieves the rpc address that can be reached by other containers in the docker network.
func (c *AvalancheChain) GetRPCAddress() string {
	return fmt.Sprintf("http://%s:9650", c.Node().HostName())
}

// GetGRPCAddress retrieves the grpc address that can be reached by other containers in the docker network.
func (c *AvalancheChain) GetGRPCAddress() string {
	return c.GetRPCAddress()
}

// GetHostRPCAddress returns the rpc address that can be reached by processes on the host machine.
// Note that this will not return a valid value until after Start returns.
func (c *AvalancheChain) GetHostRPCAddress() string {
	return fmt.Sprintf("http://127.0.0.1:%s", c.Node().RPCPort())
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
	return nil
	//return c.node().CreateKey(ctx, keyName)
}

// RecoverKey recovers an existing user from a given mnemonic.
func (c *AvalancheChain) RecoverKey(ctx context.Context, name, mnemonic string) error {
	return c.Node().RecoverKey(ctx, name, mnemonic)
}

// GetAddress fetches the bech32 address for a test key on the "user" node (either the first fullnode or the first validator if no fullnodes).
func (c *AvalancheChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	return c.Node().GetAddress(ctx, keyName)
}

// SendFunds sends funds to a wallet from a user account.
func (c *AvalancheChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	return c.Node().SendFunds(ctx, keyName, amount)
}

func (c *AvalancheChain) SendFundsWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	panic("AvalancheChain SendFundsWithNote Unimplemented")
}

// SendIBCTransfer sends an IBC transfer returning a transaction or an error if the transfer failed.
func (c *AvalancheChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	return c.Node().SendIBCTransfer(ctx, channelID, keyName, amount, options)
}

// Height returns the current block height or an error if unable to get current height.
func (c *AvalancheChain) Height(ctx context.Context) (int64, error) {
	height, err := c.Node().Height(ctx)
	if err != nil {
		return 0, err
	}

	return int64(height), nil
}

// GetBalance fetches the current balance for a specific account address and denom.
func (c *AvalancheChain) GetBalance(ctx context.Context, address string, denom string) (sdkmath.Int, error) {
	balance, err := c.Node().GetBalance(ctx, address, denom)
	if err != nil {
		return sdkmath.ZeroInt(), err
	}

	return sdkmath.NewInt(balance), nil
}

// GetBankBalance fetches the current balance for a specific account address and denom from Bank Smart Contract
func (c *AvalancheChain) GetBankBalance(ctx context.Context, address string, denom string) (sdkmath.Int, error) {
	balance, err := c.Node().GetBankBalance(ctx, c.bankContractAddress.String(), address, denom)
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

	return NewWallet(keyName, common.BytesToAddress(addrBytes), mnemonic, c.cfg), nil
}

// BuildRelayerWallet will return a chain-specific wallet populated with the mnemonic so that the wallet can
// be restored in the relayer node using the mnemonic. After it is built, that address is included in
// genesis with some funds.
func (c *AvalancheChain) BuildRelayerWallet(ctx context.Context, keyName string) (ibc.Wallet, error) {
	pk := "56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027"
	pkey, err := crypto.HexToECDSA(pk)
	if err != nil {
		return nil, err
	}
	addr := crypto.PubkeyToAddress(pkey.PublicKey)

	return NewWallet(keyName, addr, pk, c.cfg), nil
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
