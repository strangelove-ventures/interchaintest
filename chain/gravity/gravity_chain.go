package gravity

import (
	"context"
	"fmt"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/ethereum/go-ethereum/common"
	"github.com/strangelove-ventures/ibctest/v6/chain/cosmos"
	"github.com/strangelove-ventures/ibctest/v6/chain/evm"
	"github.com/strangelove-ventures/ibctest/v6/dockerutil"
	"github.com/strangelove-ventures/ibctest/v6/ibc"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"os"
	"path"
	"strconv"
)

type GravityChain struct {
	cosmosChain *cosmos.CosmosChain

	numOrchestrators int
	Orchestrators    OrchestratorNodes

	evmChains           []*evm.Chain
	evmGravityContracts []common.Address
}

func NewGravityChain(testname string, chainConfig ibc.ChainConfig, numValidators int, numFullNodes int, numOrchestrators int, log *zap.Logger) *GravityChain {
	cosmosChain := cosmos.NewCosmosChain(testname, chainConfig, numValidators, numFullNodes, log)

	return &GravityChain{
		cosmosChain:      cosmosChain,
		numOrchestrators: numOrchestrators,
	}
}

func (g *GravityChain) Config() ibc.ChainConfig {
	return g.cosmosChain.Config()
}

func (g *GravityChain) NewOrchestratorNode(
	ctx context.Context,
	testName string,
	cli *client.Client,
	networkID string,
	image ibc.DockerImage,
) (*OrchestratorNode, error) {
	on := &OrchestratorNode{
		log: g.cosmosChain.Log,

		Chain:        g.cosmosChain,
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
		Log: g.cosmosChain.Log,

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
	if err := g.cosmosChain.Initialize(ctx, testName, cli, networkID); err != nil {
		return err
	}

	// initialize orchestrators
	image := g.Config().Images[1]
	newOrchs := make(OrchestratorNodes, g.numOrchestrators)
	eg, egCtx := errgroup.WithContext(ctx)
	for i := len(g.Orchestrators); i < g.numOrchestrators; i++ {
		i := i
		eg.Go(func() error {
			orch, err := g.NewOrchestratorNode(egCtx, testName, cli, networkID, image)
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
	//g.cosmosChain.findTxMu.Lock()
	//defer g.cosmosChain.findTxMu.Unlock()
	g.Orchestrators = newOrchs
	return nil
}

func (g *GravityChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	if err := g.cosmosChain.Start(testName, ctx, additionalGenesisWallets...); err != nil {
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
				g.cosmosChain.Config().Denom,
				evmChain.Node.Name(),
				g.cosmosChain.Validators[i].Name(),
				g.cosmosChain.Config().GasPrices,
				g.cosmosChain.Config().Denom,
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
}

func (g *GravityChain) Exec(ctx context.Context, cmd []string, env []string) (stdout, stderr []byte, err error) {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) ExportState(ctx context.Context, height int64) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) GetRPCAddress() string {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) GetGRPCAddress() string {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) GetHostRPCAddress() string {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) GetHostGRPCAddress() string {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) HomeDir() string {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) CreateKey(ctx context.Context, keyName string) error {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) RecoverKey(ctx context.Context, name, mnemonic string) error {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) SendFunds(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) SendIBCTransfer(ctx context.Context, channelID, keyName string, amount ibc.WalletAmount, options ibc.TransferOptions) (ibc.Tx, error) {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) Height(ctx context.Context) (uint64, error) {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) GetBalance(ctx context.Context, address string, denom string) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) GetGasFeesInNativeDenom(gasPaid int64) int64 {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) Acknowledgements(ctx context.Context, height uint64) ([]ibc.PacketAcknowledgement, error) {
	//TODO implement me
	panic("implement me")
}

func (g *GravityChain) Timeouts(ctx context.Context, height uint64) ([]ibc.PacketTimeout, error) {
	//TODO implement me
	panic("implement me")
}
