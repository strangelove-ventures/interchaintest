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
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	transactionv1 "github.com/strangelove-ventures/interchaintest/v8/chain/penumbra/core/transaction/v1"

	//nolint:staticcheck
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"

	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	asset "github.com/strangelove-ventures/interchaintest/v8/chain/penumbra/core/asset/v1"
	ibcv1 "github.com/strangelove-ventures/interchaintest/v8/chain/penumbra/core/component/ibc/v1"
	pool "github.com/strangelove-ventures/interchaintest/v8/chain/penumbra/core/component/shielded_pool/v1"
	keys "github.com/strangelove-ventures/interchaintest/v8/chain/penumbra/core/keys/v1"
	num "github.com/strangelove-ventures/interchaintest/v8/chain/penumbra/core/num/v1"
	custody "github.com/strangelove-ventures/interchaintest/v8/chain/penumbra/custody/v1"
	view "github.com/strangelove-ventures/interchaintest/v8/chain/penumbra/view/v1"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// PenumbraClientNode represents an instance of pclientd.
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

	GRPCConn *grpc.ClientConn

	address    []byte
	addrString string

	containerLifecycle *dockerutil.ContainerLifecycle

	// Set during StartContainer.
	hostGRPCPort string
}

// NewClientNode attempts to initialize a new instance of pclientd.
// It then attempts to create the Docker container lifecycle and the Docker volume before setting the volume owner.
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

var pclientdPorts = nat.PortMap{
	nat.Port(pclientdPort): {},
}

// Name of the test node container.
func (p *PenumbraClientNode) Name() string {
	return fmt.Sprintf("pclientd-%d-%s-%s-%s", p.Index, p.KeyName, p.Chain.Config().ChainID, p.TestName)
}

// HostName returns the hostname of the test node container.
func (p *PenumbraClientNode) HostName() string {
	return dockerutil.CondenseHostName(p.Name())
}

// Bind returns the home folder bind point for running the node.
func (p *PenumbraClientNode) Bind() []string {
	return []string{fmt.Sprintf("%s:%s", p.VolumeName, p.HomeDir())}
}

// HomeDir returns the home directory for this instance of pclientd in the Docker filesystem.
func (p *PenumbraClientNode) HomeDir() string {
	return "/home/pclientd"
}

// GetAddress returns the Bech32m encoded string of the inner bytes as a slice of bytes.
func (p *PenumbraClientNode) GetAddress(ctx context.Context) ([]byte, error) {
	addrReq := &view.AddressByIndexRequest{
		AddressIndex: &keys.AddressIndex{
			Account: 0,
		},
	}

	viewClient := view.NewViewServiceClient(p.GRPCConn)

	resp, err := viewClient.AddressByIndex(ctx, addrReq)
	if err != nil {
		return nil, err
	}

	return resp.Address.Inner, nil
}

// SendFunds sends funds from the PenumbraClientNode to a specified address.
// It generates a transaction plan, gets authorization data for the transaction,
// builds and signs the transaction, and broadcasts it. Returns an error if any step of the process fails.
func (p *PenumbraClientNode) SendFunds(ctx context.Context, amount ibc.WalletAmount) error {
	hi, lo := translateBigInt(amount.Amount)

	// Generate a transaction plan sending funds to an address.
	tpr := &view.TransactionPlannerRequest{
		Outputs: []*view.TransactionPlannerRequest_Output{{
			Value: &asset.Value{
				Amount: &num.Amount{
					Lo: lo,
					Hi: hi,
				},
				AssetId: &asset.AssetId{AltBaseDenom: amount.Denom},
			},
			Address: &keys.Address{AltBech32M: amount.Address},
		}},
	}

	viewClient := view.NewViewServiceClient(p.GRPCConn)

	resp, err := viewClient.TransactionPlanner(ctx, tpr)
	if err != nil {
		return err
	}

	// Get authorization data for the transaction from pclientd (signing).
	custodyClient := custody.NewCustodyServiceClient(p.GRPCConn)
	authorizeReq := &custody.AuthorizeRequest{
		Plan:              resp.Plan,
		PreAuthorizations: []*custody.PreAuthorization{},
	}

	authData, err := custodyClient.Authorize(ctx, authorizeReq)
	if err != nil {
		return err
	}

	// Have pclientd build and sign the planned transaction.
	wbr := &view.WitnessAndBuildRequest{
		TransactionPlan:   resp.Plan,
		AuthorizationData: authData.Data,
	}

	buildClient, err := viewClient.WitnessAndBuild(ctx, wbr)
	if err != nil {
		return err
	}

	var tx *transactionv1.Transaction
	for {
		buildResp, err := buildClient.Recv()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return err
			}
		}

		status := buildResp.GetBuildProgress()

		if status != nil {
			// Progress is a float between 0 and 1 that is an approximation of the build progress.
			// If the status is not complete we need to loop and wait for completion.
			if status.Progress < 1 {
				continue
			}
		}

		tx = buildResp.GetComplete().Transaction
		if tx == nil {
			continue
		}
	}

	// Have pclientd broadcast and await confirmation of the built transaction.
	btr := &view.BroadcastTransactionRequest{
		Transaction:    tx,
		AwaitDetection: true,
	}

	_, err = viewClient.BroadcastTransaction(ctx, btr)
	if err != nil {
		return err
	}

	return nil
}

// SendIBCTransfer sends an IBC transfer from the current PenumbraClientNode to a specified destination address on a specified channel.
// The function validates the address string on the current PenumbraClientNode instance. If the address string is empty, it returns an error.
// It translates the amount to a big integer and creates an `ibcv1.Ics20Withdrawal` with the amount, denom, destination address, return address, timeout height, timeout timestamp
func (p *PenumbraClientNode) SendIBCTransfer(
	ctx context.Context,
	channelID string,
	amount ibc.WalletAmount,
	options ibc.TransferOptions,
) (ibc.Tx, error) {
	if p.addrString == "" {
		return ibc.Tx{}, fmt.Errorf("address string was not cached on pclientd instance for key with name %s", p.KeyName)
	}

	timeoutHeight, timeoutTimestamp := ibcTransferTimeouts(options)

	hi, lo := translateBigInt(amount.Amount)

	withdrawal := &ibcv1.Ics20Withdrawal{
		Amount: &num.Amount{
			Lo: lo,
			Hi: hi,
		},
		Denom: &asset.Denom{
			Denom: amount.Denom,
		},
		DestinationChainAddress: amount.Address,
		ReturnAddress: &keys.Address{
			AltBech32M: p.addrString,
		},
		TimeoutHeight: &timeoutHeight,
		TimeoutTime:   timeoutTimestamp,
		SourceChannel: channelID,
	}

	// TODO: remove debug output
	fmt.Printf("Timeout timestamp: %+v \n", timeoutTimestamp)
	fmt.Printf("Timeout: %+v \n", timeoutHeight)
	fmt.Printf("Withdrawal: %+v \n", withdrawal)

	// Generate a transaction plan sending ics_20 transfer
	tpr := &view.TransactionPlannerRequest{
		Ics20Withdrawals: []*ibcv1.Ics20Withdrawal{withdrawal},
	}

	viewClient := view.NewViewServiceClient(p.GRPCConn)

	resp, err := viewClient.TransactionPlanner(ctx, tpr)
	if err != nil {
		return ibc.Tx{}, err
	}

	// Get authorization data for the transaction from pclientd (signing).
	custodyClient := custody.NewCustodyServiceClient(p.GRPCConn)
	authorizeReq := &custody.AuthorizeRequest{
		Plan:              resp.Plan,
		PreAuthorizations: []*custody.PreAuthorization{},
	}

	authData, err := custodyClient.Authorize(ctx, authorizeReq)
	if err != nil {
		return ibc.Tx{}, err
	}

	// Have pclientd build and sign the planned transaction.
	wbr := &view.WitnessAndBuildRequest{
		TransactionPlan:   resp.Plan,
		AuthorizationData: authData.Data,
	}

	buildClient, err := viewClient.WitnessAndBuild(ctx, wbr)
	if err != nil {
		return ibc.Tx{}, err
	}

	var tx *transactionv1.Transaction
	for {
		buildResp, err := buildClient.Recv()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return ibc.Tx{}, err
			}
		}

		status := buildResp.GetBuildProgress()

		if status != nil {
			// Progress is a float between 0 and 1 that is an approximation of the build progress.
			// If the status is not complete we need to loop and wait for completion.
			if status.Progress < 1 {
				continue
			}
		}

		tx = buildResp.GetComplete().Transaction
		if tx == nil {
			continue
		}
	}

	// Have pclientd broadcast and await confirmation of the built transaction.
	btr := &view.BroadcastTransactionRequest{
		Transaction:    tx,
		AwaitDetection: true,
	}

	txClient, err := viewClient.BroadcastTransaction(ctx, btr)
	if err != nil {
		return ibc.Tx{}, err
	}

	var confirmed *view.BroadcastTransactionResponse_Confirmed
	for {
		txResp, err := txClient.Recv()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return ibc.Tx{}, err
			}
		}

		// Wait until the tx is confirmed on-chain by the view server.
		confirmed = txResp.GetConfirmed()
		if confirmed == nil {
			continue
		}
	}

	// TODO: fill in rest of tx details
	return ibc.Tx{
		Height:   int64(confirmed.DetectionHeight),
		TxHash:   string(confirmed.Id.Inner),
		GasSpent: 0,
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

// GetBalance retrieves the balance of a specific denom for the PenumbraClientNode.
//
// It creates a client for the ViewProtocolService and constructs a BalancesRequest with an AccountFilter and AssetIdFilter.
// A Balances stream response is obtained from the server.
// The balances are collected in a slice until the stream is done, or an error occurs.
// Otherwise, the first balance in the slice is used to construct a math.Int value and returned.
// Returns:
// - math.Int: The balance of the specified denom.
// - error: An error if any occurred during the balance retrieval.
func (p *PenumbraClientNode) GetBalance(ctx context.Context, denom string) (math.Int, error) {
	viewClient := view.NewViewServiceClient(p.GRPCConn)

	balanceRequest := &view.BalancesRequest{
		AccountFilter: &keys.AddressIndex{
			Account: 0,
		},
		AssetIdFilter: &asset.AssetId{
			AltBaseDenom: denom,
		},
	}

	// The BalanceByAddress method returns a stream response, containing
	// zero-or-more balances, including denom and amount info per balance.
	balanceStream, err := viewClient.Balances(ctx, balanceRequest)
	if err != nil {
		return math.Int{}, err
	}

	var balances []*view.BalancesResponse
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

	if len(balances) <= 0 {
		return math.ZeroInt(), nil
	}

	balance := balances[0]
	hi := balance.GetBalanceView().GetKnownAssetId().GetAmount().GetHi()
	lo := balance.GetBalanceView().GetKnownAssetId().GetAmount().GetLo()
	return translateHiAndLo(hi, lo), nil
}

// translateHiAndLo takes the high and low order bytes and decodes the two uint64 values into the single int128 value
// they represent.
//
// Since Go does not support native uint128 we make use of the big.Int type.
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

// translateBigInt converts a Cosmos SDK Int, which is a wrapper around Go's big.Int, into two uint64 values.
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
func (p *PenumbraClientNode) GetDenomMetadata(ctx context.Context, assetId *asset.AssetId) (*asset.Metadata, error) {
	queryClient := pool.NewQueryServiceClient(p.GRPCConn)
	req := &pool.AssetMetadataByIdRequest{
		AssetId: assetId,
	}

	resp, err := queryClient.AssetMetadataById(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.DenomMetadata, nil
}

// WriteFile accepts file contents in a byte slice and writes the contents to
// the Docker filesystem. relPath describes the location of the file in the
// Docker volume relative to the home directory.
func (p *PenumbraClientNode) WriteFile(ctx context.Context, content []byte, relPath string) error {
	fw := dockerutil.NewFileWriter(p.log, p.DockerClient, p.TestName)
	return fw.WriteFile(ctx, p.VolumeName, relPath, content)
}

// Initialize loads the view and spend keys into the pclientd config.
func (p *PenumbraClientNode) Initialize(ctx context.Context, pdAddress, spendKey, fullViewingKey string) error {
	c := make(testutil.Toml)

	kmsConfig := make(testutil.Toml)
	kmsConfig["spend_key"] = spendKey
	c["kms_config"] = kmsConfig
	c["full_viewing_key"] = fullViewingKey
	c["grpc_url"] = pdAddress
	c["bind_addr"] = "0.0.0.0:" + strings.Split(pclientdPort, "/")[0]

	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(c); err != nil {
		return err
	}

	return p.WriteFile(ctx, buf.Bytes(), "config.toml")
}

// CreateNodeContainer creates a container for the Penumbra client node.
func (p *PenumbraClientNode) CreateNodeContainer(ctx context.Context) error {
	cmd := []string{
		"pclientd",
		"--home", p.HomeDir(),
		"start",
	}

	return p.containerLifecycle.CreateContainer(ctx, p.TestName, p.NetworkID, p.Image, pclientdPorts, p.Bind(), nil, p.HostName(), cmd, p.Chain.Config().Env)
}

// StopContainer stops the container associated with the PenumbraClientNode.
func (p *PenumbraClientNode) StopContainer(ctx context.Context) error {
	return p.containerLifecycle.StopContainer(ctx)
}

// StartContainer starts the test node container.
func (p *PenumbraClientNode) StartContainer(ctx context.Context) error {
	if err := p.containerLifecycle.StartContainer(ctx); err != nil {
		return err
	}

	hostPorts, err := p.containerLifecycle.GetHostPorts(ctx, pclientdPort)
	if err != nil {
		return err
	}

	p.hostGRPCPort = hostPorts[0]

	p.GRPCConn, err = grpc.Dial(p.hostGRPCPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	return nil
}

// Exec runs a container for a specific job and blocks until the container exits.
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

// shouldUseDefaults checks if the provided timeout is nil or has both the NanoSeconds and Height fields set to zero.
// If the timeout is nil or both fields are zeros, it returns true, indicating that the defaults should be used.
// Otherwise, it returns false, indicating that the provided timeout should be used.
func shouldUseDefaults(timeout *ibc.IBCTimeout) bool {
	return timeout == nil || (timeout.NanoSeconds == 0 && timeout.Height == 0)
}

// ibcTransferTimeouts calculates the timeout height and timeout timestamp for an IBC transfer based on the provided options.
//
// If the options.Timeout is nil or both NanoSeconds and Height are equal to zero, it uses the defaultTransferTimeouts function to get the default timeout values.
// Otherwise, it sets the timeoutTimestamp to options.Timeout.NanoSeconds and timeoutHeight to clienttypes.NewHeight(0, options.Timeout.Height).
//
// The function then returns the timeoutHeight and timeoutTimestamp.
func ibcTransferTimeouts(options ibc.TransferOptions) (clienttypes.Height, uint64) {
	var (
		timeoutHeight    clienttypes.Height
		timeoutTimestamp uint64
	)

	if shouldUseDefaults(options.Timeout) {
		timeoutHeight, timeoutTimestamp = defaultTransferTimeouts()
	} else {
		timeoutTimestamp = options.Timeout.NanoSeconds
		timeoutHeight = clienttypes.NewHeight(0, uint64(options.Timeout.Height))
	}

	return timeoutHeight, timeoutTimestamp
}

// defaultTransferTimeouts returns the default relative timeout values from ics-20 for both block height and timestamp
// based timeouts.
// see: https://github.com/cosmos/ibc-go/blob/0364aae96f0326651c411ed0f3486be570280e5c/modules/apps/transfer/types/packet.go#L22-L33
func defaultTransferTimeouts() (clienttypes.Height, uint64) {
	t, err := clienttypes.ParseHeight(transfertypes.DefaultRelativePacketTimeoutHeight)
	if err != nil {
		panic(fmt.Errorf("cannot parse packet timeout height string when retrieving default value: %w", err))
	}
	return t, transfertypes.DefaultRelativePacketTimeoutTimestamp
}
