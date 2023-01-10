package gravity

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"

	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/ethereum/go-ethereum/common"
	"github.com/strangelove-ventures/ibctest/v6/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v6/chain/evm"
	"github.com/strangelove-ventures/ibctest/v6/dockerutil"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type EVM struct {
	ID   uint32
	Name string
}

type GravityChain struct {
	CosmosChain *cosmos.CosmosChain

	numOrchestrators int
	Orchestrators    OrchestratorNodes

	evmChains             []*evm.Chain
	evmValidatorMnemonics []string
	evmGravityContracts   []common.Address
}

func NewGravityChain(testname string, chainConfig ibc.ChainConfig, numValidators int, numFullNodes int, numOrchestrators int, evms []*evm.Chain, evmMnemonics []string, log *zap.Logger) *GravityChain {
	cosmosChain := cosmos.NewCosmosChain(testname, chainConfig, numValidators, numFullNodes, log)

	return &GravityChain{
		CosmosChain:           cosmosChain,
		evmValidatorMnemonics: evmMnemonics,
		numOrchestrators:      numOrchestrators,
		evmChains:             evms,
	}
}

func (g *GravityChain) Config() ibc.ChainConfig {
	return g.CosmosChain.Config()
}

func (g *GravityChain) NewOrchestratorNode(
	ctx context.Context,
	testName string,
	cli *client.Client,
	networkID string,
	evmMnemonic string,
	image ibc.DockerImage,
) (*OrchestratorNode, error) {
	on := &OrchestratorNode{
		log: g.CosmosChain.Log,

		mnemonic: evmMnemonic,

		Chain:        g,
		DockerClient: cli,
		NetworkID:    networkID,
		TestName:     testName,
		Image:        image,
	}

	v, err := cli.VolumeCreate(ctx, volumetypes.VolumeCreateBody{
		Labels: map[string]string{
			dockerutil.CleanupLabel: testName,

			dockerutil.NodeOwnerLabel: on.Name(),
		}})
	if err != nil {
		return nil, fmt.Errorf("creating volume for orchestrator node: %w", err)
	}

	on.VolumeName = v.Name
	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: g.CosmosChain.Log,

		Client: cli,

		VolumeName: v.Name,
		ImageRef:   image.Ref(),
		TestName:   testName,
		UidGid:     image.UidGid,
	}); err != nil {
		return nil, fmt.Errorf("set volume owner: %w", err)
	}
	return on, nil
}

func (g *GravityChain) Initialize(ctx context.Context, testName string, cli *client.Client, networkID string) error {
	if err := g.CosmosChain.Initialize(ctx, testName, cli, networkID); err != nil {
		return err
	}

	// initialize orchestrators
	image := g.Config().Images[1]
	newOrchs := make(OrchestratorNodes, g.numOrchestrators)
	eg, egCtx := errgroup.WithContext(ctx)
	for i := len(g.Orchestrators); i < g.numOrchestrators; i++ {
		i := i
		eg.Go(func() error {
			orch, err := g.NewOrchestratorNode(egCtx, testName, cli, networkID, g.evmValidatorMnemonics[i], image)
			if err != nil {
				return err
			}
			orch.Index = i
			newOrchs[i] = orch
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	//todo: is this lock necessary?
	//g.CosmosChain.findTxMu.Lock()
	//defer g.CosmosChain.findTxMu.Unlock()
	g.Orchestrators = newOrchs
	return nil
}

func (g *GravityChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	if err := g.CosmosChain.Start(testName, ctx, additionalGenesisWallets...); err != nil {
		return err
	}

	bootstrap, err := os.ReadFile(path.Join("chain", "gravity", "gorc_bootstrap.sh"))
	if err != nil {
		return err
	}

	for _, o := range g.Orchestrators {
		o := o
		configPath := "gorc"
		for i, evmChain := range g.evmChains {
			gorcCfg := fmt.Sprintf(`keystore = "/root/gorc/%d/keystore/"

[gravity]
contract = "%s"
fees_denom = "%s"

[ethereum]
key_derivation_path = "m/44'/60'/0'/0/0"
rpc = "http://%s:8545"

[cosmos]
key_derivation_path = "m/44'/118'/1'/0/0"
grpc = "http://%s:9090"
gas_price = { amount = %s, denom = "%s" }
prefix = "cosmos"
gas_adjustment = 2.0
msg_batch_size = 5

[metrics]
listen_addr = "127.0.0.1:300%d"
`,
				evmChain.ID,
				g.evmGravityContracts[i].String(),
				g.CosmosChain.Config().Denom,
				"empty-evm-name",
				//evmChain.Node.Name(),
				g.CosmosChain.Validators[i].Name(),
				g.CosmosChain.Config().GasPrices,
				g.CosmosChain.Config().Denom,
				i,
			)

			fw := dockerutil.NewFileWriter(o.logger(), o.DockerClient, testName)
			filepath := path.Join(configPath, strconv.Itoa(int(evmChain.ID)), "config.toml")
			if err := fw.WriteFile(ctx, o.VolumeName, filepath, []byte(gorcCfg)); err != nil {
				return fmt.Errorf("error writing config.toml for orchestrator %d, chain %d, filepath %s, err %w", o.Index, evmChain.ID, filepath, err)
			}

			// make sure the bootstrap file exists so keys can be prepopulated
			if err := fw.WriteFile(ctx, o.VolumeName, path.Join(configPath, "gorc_bootstrap.sh"), bootstrap); err != nil {
				return err
			}

			if err := o.StartContainer(ctx); err != nil {
				return err
			}

			// check for orchestrator health
		}
	}

	return nil
}

func (g *GravityChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	//TODO implement me
	panic("implement me - exec")
}

func (g *GravityChain) ExportState(ctx context.Context, height int64) (string, error) {
	//TODO implement me
	panic("implement me - export state")
}

func (g *GravityChain) GetRPCAddress() string {
	//TODO implement me
	panic("implement me - get rpc address")
}

func (g *GravityChain) GetGRPCAddress() string {
	//TODO implement me
	panic("implement me - get grpc address")
}

func (g *GravityChain) GetHostRPCAddress() string {
	//TODO implement me
	panic("implement me - get host rpc address")
}

func (g *GravityChain) GetHostGRPCAddress() string {
	//TODO implement me
	panic("implement me - get host grpc address")
}

func (g *GravityChain) HomeDir() string {
	//TODO implement me
	panic("implement me - home dir")
}

func (g *GravityChain) CreateKey(ctx context.Context, keyName string) error {
	return g.CosmosChain.CreateKey(ctx, keyName)
}

func (g *GravityChain) RecoverKey(ctx context.Context, name, mnemonic string) error {
	//TODO implement me
	panic("implement me - recover key")
}

func (g *GravityChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	//TODO implement me
	panic("implement me - get address")
}

func (g *GravityChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	//TODO implement me
	panic("implement me - send funds")
}

func (g *GravityChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	//TODO implement me
	panic("implement me - send ibc transfer")
}

func (g *GravityChain) Height(ctx context.Context) (uint64, error) {
	//TODO implement me
	panic("implement me - height")
}

func (g *GravityChain) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
	//TODO implement me
	panic("implement me - get balance")
}

func (g *GravityChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	//TODO implement me
	panic("implement me - get gas fees in native denom")
}

func (g *GravityChain) Acknowledgements(ctx context.Context, height uint64) ([]ibc.PacketAcknowledgement, error) {
	//TODO implement me
	panic("implement me - acknowledgements")
}

func (g *GravityChain) Timeouts(ctx context.Context, height uint64) ([]ibc.PacketTimeout, error) {
	//TODO implement me
	panic("implement me - timeouts")
}

// StopAllNodes stops and removes all long-running containers (validators and full nodes)
func (g *GravityChain) StopAllNodes(ctx context.Context) error {
	var eg errgroup.Group
	for _, n := range g.CosmosChain.Nodes() {
		n := n
		eg.Go(func() error {
			if err := n.StopContainer(ctx); err != nil {
				return err
			}
			return n.RemoveContainer(ctx)
		})
	}

	for _, o := range g.Orchestrators {
		o := o
		eg.Go(func() error {
			if err := o.StopContainer(ctx); err != nil {
				return err
			}
			return o.RemoveContainer(ctx)
		})
	}

	return eg.Wait()
}

// StartAllNodes creates and starts new containers for each node.
// Should only be used if the chain has previously been started with .Start.
func (g *GravityChain) StartAllNodes(ctx context.Context) error {
	// prevent client calls during this time
	var eg errgroup.Group
	if err := g.CosmosChain.StartAllNodes(ctx); err != nil {

	}
	eg.Go(func() error {
		return g.CosmosChain.StartAllNodes(ctx)
	})
	for _, o := range g.Orchestrators {
		o := o
		eg.Go(func() error {
			if err := o.CreateContainer(ctx); err != nil {
				return err
			}
			return o.StartContainer(ctx)
		})
	}
	return eg.Wait()
}

func (g *GravityChain) UpgradeVersion(ctx context.Context, cli *client.Client, version string) {
	g.CosmosChain.UpgradeVersion(ctx, cli, version)
}
