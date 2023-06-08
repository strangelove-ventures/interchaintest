package penumbra

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/pactus-project/pactus/util/bech32m"
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

	tv, err := dockerClient.VolumeCreate(ctx, volumetypes.VolumeCreateBody{
		Labels: map[string]string{
			dockerutil.CleanupLabel: testName,

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
	channel, err := grpc.Dial(fmt.Sprintf("tcp://%s:%s", p.HostName(), strings.Split(grpcPort, "/")))
	if err != nil {
		return err
	}
	defer channel.Close()

	// 5.1. Generate a transaction plan sending funds to an address.
	tpr := &viewv1alpha1.TransactionPlannerRequest{
		//XAccountGroupId: nil,
		Outputs: []*viewv1alpha1.TransactionPlannerRequest_Output{{
			Value: &cryptov1alpha1.Value{
				Amount: &cryptov1alpha1.Amount{
					Lo: uint64(amount.Amount),
					Hi: uint64(amount.Amount),
				},
				AssetId: &cryptov1alpha1.AssetId{Inner: []byte(amount.Denom)},
			},
			Address: &cryptov1alpha1.Address{Inner: []byte(amount.Address)},
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

	_, err = viewClient.BroadcastTransaction(ctx, btr, nil)
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

func (p *PenumbraClientNode) GetBalance(ctx context.Context, _ string) (int64, error) {
	channel, err := grpc.Dial(
		p.hostGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return 0, err
	}
	defer channel.Close()

	viewClient := viewv1alpha1.NewViewProtocolServiceClient(channel)

	fmt.Println("Before GetAddress")

	addressReq := &viewv1alpha1.AddressByIndexRequest{
		AddressIndex: &cryptov1alpha1.AddressIndex{
			Account:    0,
			Randomizer: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		}}

	res, err := viewClient.AddressByIndex(ctx, addressReq)
	if err != nil {
		return 0, err
	}

	fmt.Println("After GetAddress")

	address := res.GetAddress()

	bzSliced := address.Inner[12:]

	_, bz, err := bech32m.DecodeNoLimit(p.addrString)
	if err != nil {
		return 0, err
	}

	equals := bytes.Compare(bz, bzSliced)
	if equals != 0 {
		fmt.Printf("address bytes are not equal, expected(%v) got(%v)", bz, bzSliced)
	}

	/*
		address bytes are not equal,
		expected([27 2 12 27 15 29 16 29 0 14 13 19 22 13 13 13 13 9 20 0 4 23 16 17 18 3 0 17 29 8 31 8 5 10 26 28 21 4 22 28 18 23 11 30 24 19 29 9 3 19 27 24 4 14 20 2 30 14 18 11 25 21 3 13 16 3 30 14 24 1 1 16 4 21 28 21 11 11 20 17 26 14 6 17 18 22 29 1 31 22 23 21 27 0 13 30 18 31 24 6 16 25 15 11 17 2 14 14 30 24 28 22 22 16 30 5 7 30 5 9 6 24 13 27 11 1 28 22])
		got([2 94 17 144 193 30 163 232 42 181 202 146 220 149 215 236 79 169 28 247 130 58 130 243 164 188 212 109 128 252 236 4 48 37 121 85 174 145 211 141 25 91 161 253 175 93 129 190 151 240 104 101 235 136 156 239 99 150 180 60 83 248 169 54 27 181 135 150])failed to encode address bz, err: invalid data byte: 94

	*/

	encodedAddr, err := bech32m.Encode("penumbrav2t", bzSliced)
	if err != nil {
		fmt.Printf("failed to encode address bz, err: %v \n", err)
	}

	fmt.Printf("Encoded addr: %s \n", encodedAddr)

	penAddress := &cryptov1alpha1.Address{
		Inner: p.address,
	}

	bar := &viewv1alpha1.BalanceByAddressRequest{
		Address: penAddress,
	}

	resp, err := viewClient.BalanceByAddress(ctx, bar)
	if err != nil {
		return 0, err
	}

	fmt.Println("After GET BAL")

	bal, err := resp.Recv()
	if err != nil {
		return 0, err
	}

	fmt.Println("After RECV")

	fmt.Printf("BAL: %+v \b", bal)
	return int64(bal.Amount.Hi), nil
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
		"RUST_LOG=debug",
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
