package penumbra

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/big"
	"strings"

	"cosmossdk.io/math"
	"github.com/BurntSushi/toml"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	clientv1alpha1 "github.com/strangelove-ventures/interchaintest/v7/chain/penumbra/client/v1alpha1"
	cryptov1alpha1 "github.com/strangelove-ventures/interchaintest/v7/chain/penumbra/core/crypto/v1alpha1"
	custodyv1alpha1 "github.com/strangelove-ventures/interchaintest/v7/chain/penumbra/custody/v1alpha1"
	viewv1alpha1 "github.com/strangelove-ventures/interchaintest/v7/chain/penumbra/view/v1alpha1"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/strangelove-ventures/interchaintest/v7/internal/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v7/testutil"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PenumbraClientNode struct {
	log *zap.Logger

	KeyName      string
	Index        int
	VolumeName   string
	Chain        ibc.Chain
	TestName     string
	NetworkID    string
	DockerClient *client.Client
	Image        ibc.DockerImage

	address    []byte
	addrString string

	containerLifecycle *dockerutil.ContainerLifecycle

	// Set during StartContainer.
	hostGRPCPort string
}

func NewClientNode(
	ctx context.Context,
	log *zap.Logger,
	chain *PenumbraChain,
	keyName string,
	index int,
	testName string,
	image ibc.DockerImage,
	dockerClient *client.Client,
	networkID string,
	address []byte,
	addrString string,
) (*PenumbraClientNode, error) {
	p := &PenumbraClientNode{
		log:          log,
		KeyName:      keyName,
		Index:        index,
		Chain:        chain,
		TestName:     testName,
		Image:        image,
		DockerClient: dockerClient,
		NetworkID:    networkID,
		address:      address,
		addrString:   addrString,
	}

	p.containerLifecycle = dockerutil.NewContainerLifecycle(log, dockerClient, p.Name())

	tv, err := dockerClient.VolumeCreate(ctx, volumetypes.CreateOptions{
		Labels: map[string]string{
			dockerutil.CleanupLabel:   testName,
			dockerutil.NodeOwnerLabel: p.Name(),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating pclientd volume: %w", err)
	}
	p.VolumeName = tv.Name
	if err := dockerutil.SetVolumeOwner(ctx, dockerutil.VolumeOwnerOptions{
		Log: log,

		Client: dockerClient,

		VolumeName: p.VolumeName,
		ImageRef:   image.Ref(),
		TestName:   testName,
		UidGid:     image.UidGid,
	}); err != nil {
		return nil, fmt.Errorf("set pclientd volume owner: %w", err)
	}

	return p, nil
}

const (
	pclientdPort = "8081/tcp"
)

var pclientdPorts = nat.PortSet{
	nat.Port(pclientdPort): {},
}

// Name of the test node container
func (p *PenumbraClientNode) Name() string {
	return fmt.Sprintf("pclientd-%d-%s-%s-%s", p.Index, p.KeyName, p.Chain.Config().ChainID, p.TestName)
}

// the hostname of the test node container
func (p *PenumbraClientNode) HostName() string {
	return dockerutil.CondenseHostName(p.Name())
}

// Bind returns the home folder bind point for running the node
func (p *PenumbraClientNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", p.VolumeName, p.HomeDir())}
}

func (p *PenumbraClientNode) HomeDir() string {
	return "/home/heighliner"
}

func (p *PenumbraClientNode) GetAddress(ctx context.Context) ([]byte, error) {
	// TODO make grpc call to pclientd to get address
	panic("not yet implemented")
}

func (p *PenumbraClientNode) SendFunds(ctx context.Context, amount ibc.WalletAmount) error {
	channel, err := grpc.Dial(p.hostGRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer channel.Close()

	hi, lo := translateBigInt(amount.Amount)

	// 5.1. Generate a transaction plan sending funds to an address.
	tpr := &viewv1alpha1.TransactionPlannerRequest{
		//XAccountGroupId: nil,
		Outputs: []*viewv1alpha1.TransactionPlannerRequest_Output{{
			Value: &cryptov1alpha1.Value{
				Amount: &cryptov1alpha1.Amount{
					Lo: lo,
					Hi: hi,
				},
				AssetId: &cryptov1alpha1.AssetId{AltBaseDenom: amount.Denom},
			},
			Address: &cryptov1alpha1.Address{AltBech32M: amount.Address},
		}},
	}

	viewClient := viewv1alpha1.NewViewProtocolServiceClient(channel)

	resp, err := viewClient.TransactionPlanner(ctx, tpr)
	if err != nil {
		return err
	}

	// 5.2. Get authorization data for the transaction from pclientd (signing).
	custodyClient := custodyv1alpha1.NewCustodyProtocolServiceClient(channel)

	authorizeReq := &custodyv1alpha1.AuthorizeRequest{
		Plan: resp.Plan,
		//AccountGroupId:    nil,
		//PreAuthorizations: nil,
	}

	authData, err := custodyClient.Authorize(ctx, authorizeReq)
	if err != nil {
		return err
	}

	// 5.3. Have pclientd build and sign the planned transaction.
	wbr := &viewv1alpha1.WitnessAndBuildRequest{
		TransactionPlan:   resp.Plan,
		AuthorizationData: authData.Data,
	}

	tx, err := viewClient.WitnessAndBuild(ctx, wbr)
	if err != nil {
		return err
	}

	// 5.4. Have pclientd broadcast and await confirmation of the built transaction.
	btr := &viewv1alpha1.BroadcastTransactionRequest{
		Transaction:    tx.Transaction,
		AwaitDetection: true,
	}

	_, err = viewClient.BroadcastTransaction(ctx, btr)
	if err != nil {
		return err
	}

	return nil
}

func (p *PenumbraClientNode) SendIBCTransfer(
	ctx context.Context,
	channelID string,
	amount ibc.WalletAmount,
	options ibc.TransferOptions,
) (ibc.Tx, error) {
	// TODO make grpc call to pclientd to send ibc transfer
	panic("not yet implemented")
}

func (p *PenumbraClientNode) GetBalance(ctx context.Context, denom string) (math.Int, error) {
	fmt.Println("Entering GetBalance function from client perspective...")
	pclientd_addr := p.hostGRPCPort
	fmt.Printf("Dialing pclientd(?) grpc address at %v\n", pclientd_addr)
	channel, err := grpc.Dial(
		pclientd_addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return math.Int{}, err
	}
	defer channel.Close()

	viewClient := viewv1alpha1.NewViewProtocolServiceClient(channel)

	fmt.Println("In GetBalance, building BalancesRequest...")
	balanceRequest := &viewv1alpha1.BalancesRequest{
		AccountFilter: &cryptov1alpha1.AddressIndex{
			Account: 0,
		},
		AssetIdFilter: &cryptov1alpha1.AssetId{
			AltBaseDenom: denom,
		},
	}

	// The BalanceByAddress method returns a stream response, containing
	// zero-or-more balances, including denom and amount info per balance.
	fmt.Println("In GetBalance, submitting BalancesRequest...")
	balanceStream, err := viewClient.Balances(ctx, balanceRequest)
	if err != nil {
		return math.Int{}, err
	}

	var balances []*viewv1alpha1.BalancesResponse
	for {
		balance, err := balanceStream.Recv()
		if err != nil {
			// A gRPC streaming response will return EOF when it's done.
			if err == io.EOF {
				break
			} else {
				return math.Int{}, err
			}
		}
		balances = append(balances, balance)
	}

	//fmt.Println("In GetBalance, dumping all wallet contents...")
	//for _, b := range balances {
	//	metadata, err := p.GetDenomMetadata(ctx, b.Balance.AssetId)
	//	if err != nil {
	//		p.log.Error(
	//			"Failed to retrieve DenomMetadata",
	//			zap.String("asset_id", b.Balance.String()),
	//			zap.Error(err),
	//		)
	//
	//		continue
	//	}
	//
	//	if metadata.Base == denom {
	//		return translateHiAndLo(b.Balance.Amount.Hi, b.Balance.Amount.Lo), nil
	//	}
	//}

	return translateHiAndLo(balances[0].Balance.Amount.Hi, balances[0].Balance.Amount.Lo), nil
}

// translateHiAndLo takes the high and low order bytes and decodes the two uint64 values into the single int128 value
// they represent. Since Go does not support native uint128 we make use of the big.Int type.
// see: https://github.com/penumbra-zone/penumbra/blob/4d175986f385e00638328c64d729091d45eb042a/crates/core/crypto/src/asset/amount.rs#L220-L240
func translateHiAndLo(hi, lo uint64) math.Int {
	hiBig := big.NewInt(0).SetUint64(hi)
	loBig := big.NewInt(0).SetUint64(lo)

	// Shift hi 8 bytes to the left
	hiBig.Lsh(hiBig, 64)

	// Add the lower order bytes
	i := big.NewInt(0).Add(hiBig, loBig)
	return math.NewIntFromBigInt(i)
}

// translateBigInt converts a Cosmos SDK Int, which is a wrapper around Go's big.Int, into two uint64 values
func translateBigInt(i math.Int) (uint64, uint64) {
	bz := i.BigInt().Bytes()

	// Pad the byte slice with leading zeros to ensure it's 16 bytes long
	paddedBytes := make([]byte, 16)
	copy(paddedBytes[16-len(bz):], bz)

	// Extract the high and low parts from the padded byte slice
	var hi uint64
	var lo uint64

	for j := 0; j < 8; j++ {
		hi <<= 8
		hi |= uint64(paddedBytes[j])
	}

	for j := 8; j < 16; j++ {
		lo <<= 8
		lo |= uint64(paddedBytes[j])
	}

	return hi, lo
}

// GetDenomMetadata invokes a gRPC request to obtain the DenomMetadata for a specified asset ID.
func (p *PenumbraClientNode) GetDenomMetadata(ctx context.Context, assetId *cryptov1alpha1.AssetId) (*cryptov1alpha1.DenomMetadata, error) {
	channel, err := grpc.Dial(
		p.hostGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	defer channel.Close()

	queryClient := clientv1alpha1.NewSpecificQueryServiceClient(channel)
	req := &clientv1alpha1.DenomMetadataByIdRequest{
		ChainId: p.Chain.Config().ChainID,
		AssetId: assetId,
	}

	resp, err := queryClient.DenomMetadataById(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.DenomMetadata, nil
}

// WriteFile accepts file contents in a byte slice and writes the contents to
// the docker filesystem. relPath describes the location of the file in the
// docker volume relative to the home directory
func (p *PenumbraClientNode) WriteFile(ctx context.Context, content []byte, relPath string) error {
	fw := dockerutil.NewFileWriter(p.log, p.DockerClient, p.TestName)
	return fw.WriteFile(ctx, p.VolumeName, relPath, content)
}

// Initialize loads the view and spend keys into the pclientd config.
func (p *PenumbraClientNode) Initialize(ctx context.Context, spendKey, fullViewingKey string) error {
	c := make(testutil.Toml)

	kmsConfig := make(testutil.Toml)
	kmsConfig["spend_key"] = spendKey
	c["kms_config"] = kmsConfig
	c["fvk"] = fullViewingKey

	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(c); err != nil {
		return err
	}

	return p.WriteFile(ctx, buf.Bytes(), "config.toml")
}

func (p *PenumbraClientNode) CreateNodeContainer(ctx context.Context, pdAddress string) error {
	cmd := []string{
		"pclientd",
		"--home", p.HomeDir(),
		"--node", pdAddress,
		"start",
		"--bind-addr", "0.0.0.0:" + strings.Split(pclientdPort, "/")[0],
	}

	// TODO: we should be able to remove this once a patch release has been tagged for pclientd
	env := []string{
		"RUST_LOG=info",
	}

	return p.containerLifecycle.CreateContainer(ctx, p.TestName, p.NetworkID, p.Image, pclientdPorts, p.Bind(), p.HostName(), cmd, env)
}

func (p *PenumbraClientNode) StopContainer(ctx context.Context) error {
	return p.containerLifecycle.StopContainer(ctx)
}

func (p *PenumbraClientNode) StartContainer(ctx context.Context) error {
	if err := p.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	hostPorts, err := p.containerLifecycle.GetHostPorts(ctx, pclientdPort)
	if err != nil {
		return err
	}

	p.hostGRPCPort = hostPorts[0]

	return nil
}

// Exec run a container for a specific job and block until the container exits
func (p *PenumbraClientNode) Exec(ctx context.Context, cmd []string, env []string) ([]byte, []byte, error) {
	job := dockerutil.NewImage(p.log, p.DockerClient, p.NetworkID, p.TestName, p.Image.Repository, p.Image.Version)
	opts := dockerutil.ContainerOptions{
		Binds: p.Bind(),
		Env:   env,
		User:  p.Image.UidGid,
	}
	res := job.Run(ctx, cmd, opts)
	return res.Stdout, res.Stderr, res.Err
}
