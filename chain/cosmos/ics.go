package cosmos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/icza/dyno"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"

	sdkmath "cosmossdk.io/math"

	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types" // nolint:staticcheck
	ccvclient "github.com/cosmos/interchain-security/v7/x/ccv/provider/client"

	"github.com/cosmos/cosmos-sdk/types"
	govv1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	stakingttypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
)

const (
	icsVer330 = "v3.3.0"
	icsVer400 = "v4.0.0"
	icsVer450 = "v4.5.0"
	icsVer640 = "v6.4.0"
)

// FinishICSProviderSetup sets up the base of an ICS connection with respect to the relayer, provider actions, and flushing of packets.
// 1. Stop the relayer, then start it back up. This completes the ICS20-1 transfer channel setup.
//   - You must set look-back block history >100 blocks in [interchaintest.NewBuiltinRelayerFactory].
//
// 2. Get the first provider validator, and delegate 1,000,000denom to it. This triggers a CometBFT power increase of 1.
// 3. Flush the pending ICS packets to the consumer chain.
func (c *CosmosChain) FinishICSProviderSetup(ctx context.Context, r ibc.Relayer, eRep *testreporter.RelayerExecReporter, ibcPath string) error {
	// Restart the relayer to finish IBC transfer connection w/ ics20-1 link
	if err := r.StopRelayer(ctx, eRep); err != nil {
		return fmt.Errorf("failed to stop relayer: %w", err)
	}
	if err := r.StartRelayer(ctx, eRep); err != nil {
		return fmt.Errorf("failed to start relayer: %w", err)
	}

	// perform provider delegation to complete provider<>consumer channel connection
	stakingVals, err := c.StakingQueryValidators(ctx, stakingttypes.BondStatusBonded)
	if err != nil {
		return fmt.Errorf("failed to query validators: %w", err)
	}

	providerVal := stakingVals[0]

	err = c.GetNode().StakingDelegate(ctx, "validator", providerVal.OperatorAddress, fmt.Sprintf("1000000%s", c.Config().Denom))
	if err != nil {
		return fmt.Errorf("failed to delegate to validator: %w", err)
	}

	stakingVals, err = c.StakingQueryValidators(ctx, stakingttypes.BondStatusBonded)
	if err != nil {
		return fmt.Errorf("failed to query validators: %w", err)
	}
	var providerAfter *stakingttypes.Validator
	for _, val := range stakingVals {
		if val.OperatorAddress == providerVal.OperatorAddress {
			providerAfter = &val
			break
		}
	}
	if providerAfter == nil {
		return fmt.Errorf("failed to find provider validator after delegation")
	}
	if providerAfter.Tokens.LT(providerVal.Tokens) {
		return fmt.Errorf("delegation failed; before: %v, after: %v", providerVal.Tokens, providerAfter.Tokens)
	}

	return c.FlushPendingICSPackets(ctx, r, eRep, ibcPath)
}

// FlushPendingICSPackets flushes the pending ICS packets to the consumer chain from the "provider" port.
func (c *CosmosChain) FlushPendingICSPackets(ctx context.Context, r ibc.Relayer, eRep *testreporter.RelayerExecReporter, ibcPath string) error {
	channels, err := r.GetChannels(ctx, eRep, c.cfg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to get channels: %w", err)
	}

	ICSChannel := ""
	for _, channel := range channels {
		if channel.PortID == "provider" {
			ICSChannel = channel.ChannelID
		}
	}

	return r.Flush(ctx, eRep, ibcPath, ICSChannel)
}

// Bootstraps the provider chain and starts it from genesis.
func (c *CosmosChain) StartProvider(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	if c.cfg.InterchainSecurityConfig.ConsumerCopyProviderKey != nil {
		return fmt.Errorf("don't set ConsumerCopyProviderKey in the provider chain")
	}
	existingFunc := c.cfg.ModifyGenesis
	c.cfg.ModifyGenesis = func(cc ibc.ChainConfig, b []byte) ([]byte, error) {
		var err error
		b, err = ModifyGenesis([]GenesisKV{
			NewGenesisKV("app_state.gov.params.voting_period", "10s"),
			NewGenesisKV("app_state.gov.params.max_deposit_period", "10s"),
			NewGenesisKV("app_state.gov.params.min_deposit.0.denom", c.cfg.Denom),
		})(cc, b)
		if err != nil {
			return nil, err
		}
		if existingFunc != nil {
			return existingFunc(cc, b)
		}
		return b, nil
	}

	const proposerKeyName = "proposer"
	if err := c.CreateKey(ctx, proposerKeyName); err != nil {
		return fmt.Errorf("failed to add proposer key: %s", err)
	}

	proposerAddr, err := c.GetFullNode().AccountKeyBech32(ctx, proposerKeyName)
	if err != nil {
		return fmt.Errorf("failed to get proposer key: %s", err)
	}

	proposer := ibc.WalletAmount{
		Address: proposerAddr,
		Denom:   c.cfg.Denom,
		Amount:  sdkmath.NewInt(10_000_000_000_000),
	}

	additionalGenesisWallets = append(additionalGenesisWallets, proposer)

	if err := c.Start(testName, ctx, additionalGenesisWallets...); err != nil {
		return err
	}

	trustingPeriod, err := time.ParseDuration(c.cfg.TrustingPeriod)
	if err != nil {
		return fmt.Errorf("failed to parse trusting period in 'StartProvider': %w", err)
	}

	spawnTimeWait := os.Getenv("ICS_SPAWN_TIME_WAIT")
	if spawnTimeWait == "" {
		spawnTimeWait = "45s"
	}
	spawnTimeWaitDuration, err := time.ParseDuration(spawnTimeWait)
	if err != nil {
		return fmt.Errorf("invalid ICS_SPAWN_TIME_WAIT %s: %w", spawnTimeWait, err)
	}
	for _, consumer := range c.Consumers {
		prop := ccvclient.ConsumerAdditionProposalJSON{
			Title:         fmt.Sprintf("Addition of %s consumer chain", consumer.cfg.Name),
			Summary:       "Proposal to add new consumer chain",
			ChainId:       consumer.cfg.ChainID,
			InitialHeight: clienttypes.Height{RevisionNumber: clienttypes.ParseChainID(consumer.cfg.ChainID), RevisionHeight: 1},
			GenesisHash:   []byte("gen_hash"),
			BinaryHash:    []byte("bin_hash"),
			SpawnTime:     time.Now().Add(spawnTimeWaitDuration),

			// TODO fetch or default variables
			BlocksPerDistributionTransmission: 1000,
			CcvTimeoutPeriod:                  trustingPeriod * 2,
			TransferTimeoutPeriod:             trustingPeriod,
			ConsumerRedistributionFraction:    "0.75",
			HistoricalEntries:                 10000,
			UnbondingPeriod:                   trustingPeriod,
			Deposit:                           "100000000" + c.cfg.Denom,
		}

		height, err := c.Height(ctx)
		if err != nil {
			return fmt.Errorf("failed to query provider height before consumer addition proposal: %w", err)
		}

		propTx, err := c.ConsumerAdditionProposal(ctx, proposerKeyName, prop)
		if err != nil {
			return err
		}

		propID, err := strconv.ParseUint(propTx.ProposalID, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse proposal id: %w", err)
		}

		if err := c.VoteOnProposalAllValidators(ctx, propID, ProposalVoteYes); err != nil {
			return err
		}

		_, err = PollForProposalStatus(ctx, c, height, height+10, propID, govv1beta1.StatusPassed)
		if err != nil {
			return fmt.Errorf("proposal status did not change to passed in expected number of blocks: %w", err)
		}
	}

	return nil
}

// Bootstraps the consumer chain and starts it from genesis.
func (c *CosmosChain) StartConsumer(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	chainCfg := c.Config()

	configFileOverrides := chainCfg.ConfigFileOverrides

	eg := new(errgroup.Group)
	// Initialize validators and fullnodes.
	for _, v := range c.Nodes() {
		eg.Go(func() error {
			if err := v.InitFullNodeFiles(ctx); err != nil {
				return err
			}
			for configFile, modifiedConfig := range configFileOverrides {
				modifiedToml, ok := modifiedConfig.(testutil.Toml)
				if !ok {
					return fmt.Errorf("provided toml override for file %s is of type (%T). Expected (DecodedToml)", configFile, modifiedConfig)
				}
				if err := testutil.ModifyTomlConfigFile(
					ctx,
					v.logger(),
					v.DockerClient,
					v.TestName,
					v.VolumeName,
					configFile,
					modifiedToml,
				); err != nil {
					return err
				}
			}
			return nil
		})
	}

	// wait for this to finish
	if err := eg.Wait(); err != nil {
		return err
	}
	consumerID, err := c.Provider.GetNode().GetConsumerChainByChainID(ctx, c.cfg.ChainID)
	if err != nil {
		return err
	}

	// Copy provider priv val keys to these nodes
	for i, val := range c.Provider.Validators {
		eg.Go(func() error {
			copy := c.cfg.InterchainSecurityConfig.ConsumerCopyProviderKey != nil && c.cfg.InterchainSecurityConfig.ConsumerCopyProviderKey(i)
			if copy {
				privVal, err := val.PrivValFileContent(ctx)
				if err != nil {
					return err
				}
				if err := c.Validators[i].OverwritePrivValFile(ctx, privVal); err != nil {
					return err
				}
			} else {
				key, _, err := c.Validators[i].ExecBin(ctx, "tendermint", "show-validator")
				if err != nil {
					return fmt.Errorf("failed to get consumer validator pubkey: %w", err)
				}
				keyStr := strings.TrimSpace(string(key))
				_, err = c.Provider.Validators[i].ExecTx(ctx, valKey, "provider", "assign-consensus-key", consumerID, keyStr)
				if err != nil {
					return fmt.Errorf("failed to assign consumer validator pubkey: %w", err)
				}
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	if c.cfg.PreGenesis != nil {
		err := c.cfg.PreGenesis(c)
		if err != nil {
			return err
		}
	}

	// Wait for spawn time
	proposals, _, err := c.Provider.GetNode().ExecQuery(ctx, "gov", "proposals")
	if err != nil {
		return fmt.Errorf("failed to query proposed chains: %w", err)
	}
	spawnTime := gjson.GetBytes(proposals, fmt.Sprintf("proposals.#(messages.0.content.chain_id==%q).messages.0.content.spawn_time", c.cfg.ChainID)).Time()
	c.log.Info("Waiting for chain to spawn", zap.Time("spawn_time", spawnTime), zap.String("chain_id", c.cfg.ChainID))
	time.Sleep(time.Until(spawnTime))
	if err := testutil.WaitForBlocks(ctx, 2, c.Provider); err != nil {
		return err
	}

	validator0 := c.Validators[0]

	for _, wallet := range additionalGenesisWallets {
		if err := validator0.AddGenesisAccount(ctx, wallet.Address, []types.Coin{{Denom: wallet.Denom, Amount: sdkmath.NewInt(wallet.Amount.Int64())}}); err != nil {
			return err
		}
	}

	genbz, err := validator0.GenesisFileContent(ctx)
	if err != nil {
		return err
	}

	ccvStateMarshaled, _, err := c.Provider.GetNode().ExecQuery(ctx, "provider", "consumer-genesis", consumerID)
	if err != nil {
		return fmt.Errorf("failed to query provider for ccv state: %w", err)
	}

	consumerICS := c.GetNode().ICSVersion(ctx)
	providerICS := c.Provider.GetNode().ICSVersion(ctx)
	ccvStateMarshaled, err = c.transformCCVState(ctx, ccvStateMarshaled, consumerICS, providerICS, chainCfg.InterchainSecurityConfig)
	if err != nil {
		return fmt.Errorf("failed to transform ccv state: %w", err)
	}

	c.log.Info("HERE STATE!", zap.String("GEN", string(ccvStateMarshaled)))

	var ccvStateUnmarshaled interface{}
	if err := json.Unmarshal(ccvStateMarshaled, &ccvStateUnmarshaled); err != nil {
		return fmt.Errorf("failed to unmarshal ccv state json: %w", err)
	}

	var genesisJSON interface{}
	if err := json.Unmarshal(genbz, &genesisJSON); err != nil {
		return fmt.Errorf("failed to unmarshal genesis json: %w", err)
	}

	if err := dyno.Set(genesisJSON, ccvStateUnmarshaled, "app_state", "ccvconsumer"); err != nil {
		return fmt.Errorf("failed to populate ccvconsumer genesis state: %w", err)
	}

	if genbz, err = json.Marshal(genesisJSON); err != nil {
		return fmt.Errorf("failed to marshal genesis bytes to json: %w", err)
	}

	genbz = bytes.ReplaceAll(genbz, []byte(`"stake"`), []byte(fmt.Sprintf(`"%s"`, chainCfg.Denom)))

	if c.cfg.ModifyGenesis != nil {
		genbz, err = c.cfg.ModifyGenesis(chainCfg, genbz)
		if err != nil {
			return err
		}
	}

	// Provide EXPORT_GENESIS_FILE_PATH and EXPORT_GENESIS_CHAIN to help debug genesis file
	exportGenesis := os.Getenv("EXPORT_GENESIS_FILE_PATH")
	exportGenesisChain := os.Getenv("EXPORT_GENESIS_CHAIN")
	if exportGenesis != "" && exportGenesisChain == c.cfg.Name {
		c.log.Debug("Exporting genesis file",
			zap.String("chain", exportGenesisChain),
			zap.String("path", exportGenesis),
		)
		_ = os.WriteFile(exportGenesis, genbz, 0o600)
	}

	chainNodes := c.Nodes()

	if c.preStartNodes != nil {
		c.preStartNodes(c)
	}

	for _, cn := range chainNodes {
		if err := cn.OverwriteGenesisFile(ctx, genbz); err != nil {
			return err
		}
	}

	if err := chainNodes.LogGenesisHashes(ctx); err != nil {
		return err
	}

	eg, egCtx := errgroup.WithContext(ctx)
	for _, n := range chainNodes {
		eg.Go(func() error {
			return n.CreateNodeContainer(egCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	peers := chainNodes.PeerString(ctx)

	eg, egCtx = errgroup.WithContext(ctx)
	for _, n := range chainNodes {
		c.log.Info("Starting container", zap.String("container", n.Name()))
		eg.Go(func() error {
			if err := n.SetPeers(egCtx, peers); err != nil {
				return err
			}
			return n.StartContainer(egCtx)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// Wait for 5 blocks before considering the chains "started"
	return testutil.WaitForBlocks(ctx, 5, c.GetFullNode())
}

func (c *CosmosChain) transformCCVState(ctx context.Context, ccvState []byte, consumerVersion, providerVersion string, icsCfg ibc.ICSConfig) ([]byte, error) {
	if icsCfg.ProviderVerOverride != "" {
		providerVersion = icsCfg.ProviderVerOverride
	}

	if icsCfg.ConsumerVerOverride != "" {
		consumerVersion = icsCfg.ConsumerVerOverride
	}
	// If they're both under 3.3.0, or if they're the same version, we don't need to transform the state.
	if semver.MajorMinor(providerVersion) == semver.MajorMinor(consumerVersion) ||
		(semver.Compare(providerVersion, icsVer330) < 0 && semver.Compare(consumerVersion, icsVer330) < 0) {
		return ccvState, nil
	}

	var imageVersion, toVersion string

	// The trick here is that when we convert the state to a consumer < 3.3.0, we need a converter that knows about that version; those are >= 4.0.0, and need a --to flag.
	// Other than that, this is a question of using whichever version is newer. If it's the provider's, we need a --to flag to tell it the consumer version.
	// If it's the consumer's, we don't need a --to flag cause it'll assume the consumer version.
	if semver.Compare(providerVersion, icsVer330) >= 0 && semver.Compare(providerVersion, consumerVersion) > 0 {
		imageVersion = icsVer400
		if semver.Compare(providerVersion, icsVer400) > 0 {
			imageVersion = providerVersion
		}
		if (semver.Major(providerVersion) == "v4" && semver.Compare(semver.MajorMinor(providerVersion), icsVer450) >= 0) ||
			(semver.Compare(semver.MajorMinor(providerVersion), icsVer640) >= 0) {
			switch semver.Major(consumerVersion) {
			case "v4":
				if semver.Compare("v4.5.0", consumerVersion) >= 0 {
					toVersion = "v4.5"
				} else {
					toVersion = "<v4.5"
				}
			case "v5":
				toVersion = "v5"
			case "v6":
				if semver.Compare("v6.4.0", consumerVersion) < 0 {
					toVersion = "<v6.4"
				}
			}
		} else {
			switch semver.Major(consumerVersion) {
			case "v5":
				toVersion = "v5"
			case "v4":
				toVersion = "v4"
			case "v3":
				toVersion = semver.MajorMinor(consumerVersion)
			case "v2":
				toVersion = "v2"
			}
		}
	} else {
		imageVersion = consumerVersion
	}

	c.log.Info("Transforming CCV state", zap.String("provider", providerVersion), zap.String("consumer", consumerVersion), zap.String("imageVersion", imageVersion), zap.String("toVersion", toVersion))

	err := c.GetNode().WriteFile(ctx, ccvState, "ccvconsumer.json")
	if err != nil {
		return nil, fmt.Errorf("failed to write ccv state to file: %w", err)
	}
	repo := icsCfg.ICSImageRepo
	if repo == "" {
		repo = "ghcr.io/strangelove-ventures/heighliner/ics"
	}
	job := dockerutil.NewImage(c.log, c.GetNode().DockerClient, c.GetNode().NetworkID,
		c.GetNode().TestName, repo, imageVersion,
	)
	cmd := []string{"interchain-security-cd", "genesis", "transform"}
	if toVersion != "" {
		cmd = append(cmd, "--to", toVersion+".x")
	}
	cmd = append(cmd, path.Join(c.GetNode().HomeDir(), "ccvconsumer.json"))
	res := job.Run(ctx, cmd, dockerutil.ContainerOptions{Binds: c.GetNode().Bind()})
	if res.Err != nil {
		return nil, fmt.Errorf("failed to transform ccv state: %w", res.Err)
	}
	return res.Stdout, nil
}
