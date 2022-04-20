package penumbra

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/strangelove-ventures/ibc-test-framework/chain/cosmos"
	"github.com/strangelove-ventures/ibc-test-framework/dockerutil"
	"github.com/strangelove-ventures/ibc-test-framework/ibc"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"golang.org/x/sync/errgroup"
)

type PenumbraNode struct {
	TendermintNode  *cosmos.ChainNode
	PenumbraAppNode *PenumbraAppNode
}

type PenumbraNodes []PenumbraNode

type PenumbraChain struct {
	testName      string
	cfg           ibc.ChainConfig
	numValidators int
	numFullNodes  int
	PenumbraNodes PenumbraNodes
}

func NewPenumbraChainConfig() ibc.ChainConfig {
	return ibc.ChainConfig{
		Type:           "penumbra",
		Name:           "penumbra",
		Bech32Prefix:   "penumbra",
		Denom:          "upenumbra",
		GasPrices:      "0.0upenumbra",
		GasAdjustment:  1.3,
		TrustingPeriod: "672h",
		Repository:     "ghcr.io/strangelove-ventures/heighliner/tendermint",
		Bin:            "tendermint",
		Meta:           []string{"ghcr.io/strangelove-ventures/heighliner/penumbra"},
	}
}

func NewPenumbraChain(testName string, chainConfig ibc.ChainConfig, numValidators int, numFullNodes int) *PenumbraChain {
	return &PenumbraChain{
		testName:      testName,
		cfg:           chainConfig,
		numValidators: numValidators,
		numFullNodes:  numFullNodes,
	}
}

// Implements Chain interface
func (c *PenumbraChain) Config() ibc.ChainConfig {
	return c.cfg
}

// Implements Chain interface
func (c *PenumbraChain) Initialize(testName string, homeDirectory string, dockerPool *dockertest.Pool, networkID string) error {
	c.initializeChainNodes(testName, homeDirectory, dockerPool, networkID)
	return nil
}

func (c *PenumbraChain) getRelayerNode() PenumbraNode {
	if len(c.PenumbraNodes) > c.numValidators {
		// use first full node
		return c.PenumbraNodes[c.numValidators]
	}
	// use first validator
	return c.PenumbraNodes[0]
}

// Implements Chain interface
func (c *PenumbraChain) GetRPCAddress() string {
	return fmt.Sprintf("http://%s:26657", c.getRelayerNode().TendermintNode.Name())
}

// Implements Chain interface
func (c *PenumbraChain) GetGRPCAddress() string {
	return fmt.Sprintf("%s:9090", c.getRelayerNode().TendermintNode.Name())
}

// GetHostRPCAddress returns the address of the RPC server accessible by the host.
// This will not return a valid address until the chain has been started.
func (c *PenumbraChain) GetHostRPCAddress() string {
	return "http://" + dockerutil.GetHostPort(c.getRelayerNode().TendermintNode.Container, rpcPort)
}

// GetHostGRPCAddress returns the address of the gRPC server accessible by the host.
// This will not return a valid address until the chain has been started.
func (c *PenumbraChain) GetHostGRPCAddress() string {
	return dockerutil.GetHostPort(c.getRelayerNode().TendermintNode.Container, grpcPort)
}

// Implements Chain interface
func (c *PenumbraChain) CreateKey(ctx context.Context, keyName string) error {
	return c.getRelayerNode().PenumbraAppNode.CreateKey(ctx, keyName)
}

// Implements Chain interface
func (c *PenumbraChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	return c.getRelayerNode().PenumbraAppNode.GetAddress(ctx, keyName)
}

// Implements Chain interface
func (c *PenumbraChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	return c.getRelayerNode().PenumbraAppNode.SendFunds(ctx, keyName, amount)
}

// Implements Chain interface
func (c *PenumbraChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, timeout *ibc.IBCTimeout) (string, error) {
	return c.getRelayerNode().PenumbraAppNode.SendIBCTransfer(ctx, channelID, keyName, amount, timeout)
}

// Implements Chain interface
func (c *PenumbraChain) InstantiateContract(ctx context.Context, keyName string, amount ibc.WalletAmount, fileName, initMessage string, needsNoAdminFlag bool) (string, error) {
	// NOOP
	return "", nil
}

// Implements Chain interface
func (c *PenumbraChain) ExecuteContract(ctx context.Context, keyName string, contractAddress string, message string) error {
	// NOOP
	return nil
}

// Implements Chain interface
func (c *PenumbraChain) DumpContractState(ctx context.Context, contractAddress string, height int64) (*ibc.DumpContractStateResponse, error) {
	// NOOP
	return nil, nil
}

// Implements Chain interface
func (c *PenumbraChain) ExportState(ctx context.Context, height int64) (string, error) {
	return c.getRelayerNode().TendermintNode.ExportState(ctx, height)
}

// Implements Chain interface
func (c *PenumbraChain) CreatePool(ctx context.Context, keyName string, contractAddress string, swapFee float64, exitFee float64, assets []ibc.WalletAmount) error {
	// NOOP
	return nil
}

// Implements Chain interface
func (c *PenumbraChain) WaitForBlocks(number int64) (int64, error) {
	return c.getRelayerNode().TendermintNode.WaitForBlocks(number)
}

func (c *PenumbraChain) Height() (int64, error) {
	return c.getRelayerNode().TendermintNode.Height()
}

// Implements Chain interface
func (c *PenumbraChain) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
	return -1, errors.New("not yet implemented")
}

// Implements Chain interface
func (c *PenumbraChain) GetTransaction(ctx context.Context, txHash string) (*types.TxResponse, error) {
	return nil, errors.New("not yet implemented")
}

// Implements Chain interface
func (c *PenumbraChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	gasPrice, _ := strconv.ParseFloat(strings.Replace(c.cfg.GasPrices, c.cfg.Denom, "", 1), 64)
	fees := float64(gasPaid) * gasPrice
	return int64(fees)
}

// creates the test node objects required for bootstrapping tests
func (c *PenumbraChain) initializeChainNodes(testName, home string,
	pool *dockertest.Pool, networkID string) {
	penumbraNodes := []PenumbraNode{}
	count := c.numValidators + c.numFullNodes
	chainCfg := c.Config()
	err := pool.Client.PullImage(docker.PullImageOptions{
		Repository: chainCfg.Repository,
		Tag:        chainCfg.Version,
	}, docker.AuthConfiguration{})
	if err != nil {
		fmt.Printf("error pulling image: %v", err)
	}
	for i := 0; i < count; i++ {
		tn := &cosmos.ChainNode{Home: home, Index: i, Chain: c,
			Pool: pool, NetworkID: networkID, TestName: testName}
		tn.MkDir()
		pn := &PenumbraAppNode{Home: home, Index: i, Chain: c,
			Pool: pool, NetworkID: networkID, TestName: testName}
		pn.MkDir()
		penumbraNodes = append(penumbraNodes, PenumbraNode{TendermintNode: tn, PenumbraAppNode: pn})
	}
	c.PenumbraNodes = penumbraNodes
}

type GenesisValidatorPubKey struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
type GenesisValidators struct {
	Address string                 `json:"address"`
	Name    string                 `json:"name"`
	Power   string                 `json:"power"`
	PubKey  GenesisValidatorPubKey `json:"pub_key"`
}
type GenesisFile struct {
	Validators []GenesisValidators `json:"validators"`
}

type ValidatorWithIntPower struct {
	Address      string
	Power        int64
	PubKeyBase64 string
}

// Bootstraps the chain and starts it from genesis
func (c *PenumbraChain) StartWithGenesisFile(testName string, ctx context.Context, home string, pool *dockertest.Pool, networkID string, genesisFilePath string) error {
	// copy genesis file to tmp path for modification
	genesisTmpFilePath := path.Join(c.getRelayerNode().TendermintNode.Dir(), "genesis_tmp.json")
	if _, err := dockerutil.Copy(genesisFilePath, genesisTmpFilePath); err != nil {
		return err
	}

	chainCfg := c.Config()

	genesisJsonBytes, err := ioutil.ReadFile(genesisTmpFilePath)
	if err != nil {
		return err
	}

	genesisFile := GenesisFile{}
	if err := json.Unmarshal(genesisJsonBytes, &genesisFile); err != nil {
		return err
	}

	genesisValidators := genesisFile.Validators
	totalPower := int64(0)

	validatorsWithPower := make([]ValidatorWithIntPower, 0)

	for _, genesisValidator := range genesisValidators {
		power, err := strconv.ParseInt(genesisValidator.Power, 10, 64)
		if err != nil {
			return err
		}
		totalPower += power
		validatorsWithPower = append(validatorsWithPower, ValidatorWithIntPower{
			Address:      genesisValidator.Address,
			Power:        power,
			PubKeyBase64: genesisValidator.PubKey.Value,
		})
	}

	sort.Slice(validatorsWithPower, func(i, j int) bool {
		return validatorsWithPower[i].Power > validatorsWithPower[j].Power
	})

	twoThirdsConsensus := int64(math.Ceil(float64(totalPower) * 2 / 3))
	totalConsensus := int64(0)

	c.PenumbraNodes = []PenumbraNode{}

	for i, validator := range validatorsWithPower {
		tn := &cosmos.ChainNode{Home: home, Index: i, Chain: c,
			Pool: pool, NetworkID: networkID, TestName: testName}
		tn.MkDir()
		pn := &PenumbraAppNode{Home: home, Index: i, Chain: c,
			Pool: pool, NetworkID: networkID, TestName: testName}
		pn.MkDir()
		c.PenumbraNodes = append(c.PenumbraNodes, PenumbraNode{
			TendermintNode:  tn,
			PenumbraAppNode: pn,
		})

		// just need to get pubkey here
		// don't care about what goes into this node's genesis file since it will be overwritten with the modified one
		if err := tn.InitHomeFolder(ctx); err != nil {
			return err
		}

		testNodePubKeyJsonBytes, err := ioutil.ReadFile(tn.PrivValKeyFilePath())
		if err != nil {
			return err
		}

		testNodePrivValFile := cosmos.PrivValidatorKeyFile{}
		if err := json.Unmarshal(testNodePubKeyJsonBytes, &testNodePrivValFile); err != nil {
			return err
		}

		// modify genesis file overwriting validators address with the one generated for this test node
		genesisJsonBytes = bytes.ReplaceAll(genesisJsonBytes, []byte(validator.Address), []byte(testNodePrivValFile.Address))

		// modify genesis file overwriting validators base64 pub_key.value with the one generated for this test node
		genesisJsonBytes = bytes.ReplaceAll(genesisJsonBytes, []byte(validator.PubKeyBase64), []byte(testNodePrivValFile.PubKey.Value))

		existingValAddressBytes, err := hex.DecodeString(validator.Address)
		if err != nil {
			return err
		}

		testNodeAddressBytes, err := hex.DecodeString(testNodePrivValFile.Address)
		if err != nil {
			return err
		}

		valConsPrefix := fmt.Sprintf("%svalcons", chainCfg.Bech32Prefix)

		existingValBech32ValConsAddress, err := bech32.ConvertAndEncode(valConsPrefix, existingValAddressBytes)
		if err != nil {
			return err
		}

		testNodeBech32ValConsAddress, err := bech32.ConvertAndEncode(valConsPrefix, testNodeAddressBytes)
		if err != nil {
			return err
		}

		genesisJsonBytes = bytes.ReplaceAll(genesisJsonBytes, []byte(existingValBech32ValConsAddress), []byte(testNodeBech32ValConsAddress))

		totalConsensus += validator.Power

		if totalConsensus > twoThirdsConsensus {
			break
		}
	}

	for i := 0; i < len(c.PenumbraNodes); i++ {
		if err := ioutil.WriteFile(c.PenumbraNodes[i].TendermintNode.GenesisFilePath(), genesisJsonBytes, 0644); err != nil { //nolint
			return err
		}
	}

	chainNodes := []*cosmos.ChainNode{}
	for _, pn := range c.PenumbraNodes {
		chainNodes = append(chainNodes, pn.TendermintNode)
	}

	if err := cosmos.ChainNodes(chainNodes).LogGenesisHashes(); err != nil {
		return err
	}

	var eg errgroup.Group

	for _, n := range c.PenumbraNodes {
		n := n
		eg.Go(func() error {
			return n.TendermintNode.CreateNodeContainer()
		})
		eg.Go(func() error {
			return n.PenumbraAppNode.CreateNodeContainer()
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	peers := cosmos.ChainNodes(chainNodes).PeerString()

	for _, n := range c.PenumbraNodes {
		n.TendermintNode.SetValidatorConfigAndPeers(peers)
	}

	for _, n := range c.PenumbraNodes {
		fmt.Printf("{%s} => starting container...\n", n.TendermintNode.Name())
		if err := n.TendermintNode.StartContainer(ctx); err != nil {
			return err
		}
		fmt.Printf("{%s} => starting container...\n", n.PenumbraAppNode.Name())
		if err := n.PenumbraAppNode.StartContainer(ctx); err != nil {
			return err
		}
	}

	time.Sleep(2 * time.Hour)

	// Wait for 5 blocks before considering the chains "started"
	_, err = c.getRelayerNode().TendermintNode.WaitForBlocks(5)
	return err
}

// Bootstraps the chain and starts it from genesis
func (c *PenumbraChain) Start(testName string, ctx context.Context, additionalGenesisWallets []ibc.WalletAmount) error {
	var eg errgroup.Group

	chainCfg := c.Config()

	genesisAmount := types.Coin{
		Amount: types.NewInt(1000000000000),
		Denom:  chainCfg.Denom,
	}

	genesisStakeAmount := types.Coin{
		Amount: types.NewInt(1000000000000),
		Denom:  "stake",
	}

	genesisSelfDelegation := types.Coin{
		Amount: types.NewInt(100000000000),
		Denom:  "stake",
	}

	genesisAmounts := []types.Coin{genesisAmount, genesisStakeAmount}

	validators := c.PenumbraNodes[:c.numValidators]
	fullnodes := c.PenumbraNodes[c.numValidators:]

	chainNodes := []*cosmos.ChainNode{}

	// sign gentx for each validator
	for _, v := range validators {
		v := v
		chainNodes = append(chainNodes, v.TendermintNode)
		eg.Go(func() error {
			return v.TendermintNode.InitValidatorFiles(ctx, &chainCfg, genesisAmounts, genesisSelfDelegation)
		})
	}

	// just initialize folder for any full nodes
	for _, n := range fullnodes {
		n := n
		chainNodes = append(chainNodes, n.TendermintNode)
		eg.Go(func() error { return n.TendermintNode.InitFullNodeFiles(ctx) })
	}

	// wait for this to finish
	if err := eg.Wait(); err != nil {
		return err
	}

	// for the validators we need to collect the gentxs and the accounts
	// to the first node's genesis file
	validator0 := validators[0]
	for i := 1; i < len(validators); i++ {
		validatorN := validators[i]
		bech64, err := validatorN.PenumbraAppNode.GetAddressBech64(ctx, valKey)
		if err != nil {
			return err
		}

		if err := validator0.TendermintNode.AddGenesisAccount(ctx, bech64, genesisAmounts); err != nil {
			return err
		}
		nNid, err := validatorN.TendermintNode.NodeID()
		if err != nil {
			return err
		}
		oldPath := path.Join(validatorN.TendermintNode.Dir(), "config", "gentx", fmt.Sprintf("gentx-%s.json", nNid))
		newPath := path.Join(validator0.TendermintNode.Dir(), "config", "gentx", fmt.Sprintf("gentx-%s.json", nNid))
		if err := os.Rename(oldPath, newPath); err != nil {
			return err
		}
	}

	for _, wallet := range additionalGenesisWallets {
		if err := validator0.TendermintNode.AddGenesisAccount(ctx, wallet.Address, []types.Coin{types.Coin{Denom: wallet.Denom, Amount: types.NewInt(wallet.Amount)}}); err != nil {
			return err
		}
	}

	if err := validator0.TendermintNode.CollectGentxs(ctx); err != nil {
		return err
	}

	genbz, err := ioutil.ReadFile(validator0.TendermintNode.GenesisFilePath())
	if err != nil {
		return err
	}

	for i := 1; i < len(c.PenumbraNodes); i++ {
		if err := ioutil.WriteFile(c.PenumbraNodes[i].TendermintNode.GenesisFilePath(), genbz, 0644); err != nil { //nolint
			return err
		}
	}

	if err := cosmos.ChainNodes(chainNodes).LogGenesisHashes(); err != nil {
		return err
	}

	for _, n := range c.PenumbraNodes {
		n := n
		eg.Go(func() error {
			return n.TendermintNode.CreateNodeContainer()
		})
		eg.Go(func() error {
			return n.PenumbraAppNode.CreateNodeContainer()
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	peers := cosmos.ChainNodes(chainNodes).PeerString()

	for _, n := range c.PenumbraNodes {
		n := n
		fmt.Printf("{%s} => starting container...\n", n.TendermintNode.Name())
		eg.Go(func() error {
			n.TendermintNode.SetValidatorConfigAndPeers(peers)
			return n.TendermintNode.StartContainer(ctx)
		})
		fmt.Printf("{%s} => starting container...\n", n.PenumbraAppNode.Name())
		eg.Go(func() error {
			return n.PenumbraAppNode.StartContainer(ctx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// Wait for 5 blocks before considering the chains "started"
	_, err = c.getRelayerNode().TendermintNode.WaitForBlocks(5)
	return err
}
