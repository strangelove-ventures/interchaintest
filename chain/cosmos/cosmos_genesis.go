package cosmos

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"sync"

	sdkmath "cosmossdk.io/math"
	types "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/docker/docker/client"
	"github.com/strangelove-ventures/interchaintest/v8/ibc"
	"github.com/strangelove-ventures/interchaintest/v8/testutil"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Bootstraps the chain and starts it from genesis
func (c *CosmosChain) Start(testName string, ctx context.Context, additionalGenesisWallets ...ibc.WalletAmount) error {
	chainCfg := c.Config()

	// genesisAmount := types.Coin{
	// 	Amount: types.NewInt(10_000_000_000_000),
	// 	Denom:  chainCfg.Denom,
	// }

	// genesisSelfDelegation := types.Coin{
	// 	Amount: types.NewInt(5_000_000_000_000),
	// 	Denom:  chainCfg.Denom,
	// }

	// if chainCfg.ModifyGenesisAmounts != nil {
	// 	genesisAmount, genesisSelfDelegation = chainCfg.ModifyGenesisAmounts()
	// }

	// genesisAmounts := []types.Coin{genesisAmount}

	decimalPow := int64(math.Pow10(int(*chainCfg.CoinDecimals)))
	genesisAmounts := make([][]types.Coin, len(c.Validators))
	genesisSelfDelegation := make([]types.Coin, len(c.Validators))

	for i := range c.Validators {
		genesisAmounts[i] = []types.Coin{{Amount: sdkmath.NewInt(10_000_000).MulRaw(decimalPow), Denom: chainCfg.Denom}}
		genesisSelfDelegation[i] = types.Coin{Amount: sdkmath.NewInt(5_000_000).MulRaw(decimalPow), Denom: chainCfg.Denom}
		if chainCfg.ModifyGenesisAmounts != nil {
			amount, selfDelegation := chainCfg.ModifyGenesisAmounts(i)
			genesisAmounts[i] = []types.Coin{amount}
			genesisSelfDelegation[i] = selfDelegation
		}
	}

	if err := c.prepNodes(ctx, c.cfg.SkipGenTx, genesisAmounts, genesisSelfDelegation); err != nil {
		return err
	}

	// TODO: this was changed to chain, we good?
	if c.cfg.PreGenesis != nil {
		// err := c.cfg.PreGenesis(chainCfg)
		err := c.cfg.PreGenesis(c)
		if err != nil {
			return err
		}
	}

	// for the validators we need to collect the gentxs and the accounts
	// to the first node's genesis file
	validator0 := c.Validators[0]
	for i := 1; i < len(c.Validators); i++ {
		validatorN := c.Validators[i]

		bech32, err := validatorN.AccountKeyBech32(ctx, valKey)
		if err != nil {
			return err
		}

		if err := validator0.AddGenesisAccount(ctx, bech32, genesisAmounts[i]); err != nil {
			return err
		}

		if !c.cfg.SkipGenTx {
			if err := validatorN.copyGentx(ctx, validator0); err != nil {
				return err
			}
		}
	}

	for _, wallet := range additionalGenesisWallets {
		if err := validator0.AddGenesisAccount(ctx, wallet.Address, []types.Coin{{Denom: wallet.Denom, Amount: wallet.Amount}}); err != nil {
			return err
		}
	}

	if !c.cfg.SkipGenTx {
		if err := validator0.CollectGentxs(ctx); err != nil {
			return err
		}
	}

	genbz, err := validator0.GenesisFileContent(ctx)
	if err != nil {
		return err
	}

	genbz = bytes.ReplaceAll(genbz, []byte(`"stake"`), []byte(fmt.Sprintf(`"%s"`, chainCfg.Denom)))

	return c.startWithFinalGenesis(ctx, genbz)
}

// Bootstraps the chain and starts it from genesis
func (c *CosmosChain) StartWithGenesisFile(
	ctx context.Context,
	testName string,
	client *client.Client,
	network string,
	genesisFilePath string,
) error {
	genBz, err := os.ReadFile(genesisFilePath)
	if err != nil {
		return fmt.Errorf("failed to read genesis file: %w", err)
	}

	chainCfg := c.Config()

	var genesisFile GenesisFile
	if err := json.Unmarshal(genBz, &genesisFile); err != nil {
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

	var eg errgroup.Group
	var mu sync.Mutex
	genBzReplace := func(find, replace []byte) {
		mu.Lock()
		defer mu.Unlock()
		genBz = bytes.ReplaceAll(genBz, find, replace)
	}

	twoThirdsConsensus := int64(math.Ceil(float64(totalPower) * 2 / 3))
	totalConsensus := int64(0)

	var activeVals []ValidatorWithIntPower
	for _, validator := range validatorsWithPower {
		activeVals = append(activeVals, validator)

		totalConsensus += validator.Power

		if totalConsensus > twoThirdsConsensus {
			break
		}
	}

	c.NumValidators = len(activeVals)

	if err := c.initializeChainNodes(ctx, testName, client, network); err != nil {
		return err
	}

	if err := c.prepNodes(ctx, true, nil, []types.Coin{}); err != nil {
		return err
	}

	// TODO: this is a duplicate, why? do we need here or only in the start? Maybe this is required in both places. idk
	if c.cfg.PreGenesis != nil {
		err := c.cfg.PreGenesis(c)
		if err != nil {
			return err
		}
	}

	for i, validator := range activeVals {
		v := c.Validators[i]
		validator := validator
		eg.Go(func() error {
			testNodePubKeyJsonBytes, err := v.ReadFile(ctx, "config/priv_validator_key.json")
			if err != nil {
				return fmt.Errorf("failed to read priv_validator_key.json: %w", err)
			}

			var testNodePrivValFile PrivValidatorKeyFile
			if err := json.Unmarshal(testNodePubKeyJsonBytes, &testNodePrivValFile); err != nil {
				return fmt.Errorf("failed to unmarshal priv_validator_key.json: %w", err)
			}

			// modify genesis file overwriting validators address with the one generated for this test node
			genBzReplace([]byte(validator.Address), []byte(testNodePrivValFile.Address))

			// modify genesis file overwriting validators base64 pub_key.value with the one generated for this test node
			genBzReplace([]byte(validator.PubKeyBase64), []byte(testNodePrivValFile.PubKey.Value))

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

			genBzReplace([]byte(existingValBech32ValConsAddress), []byte(testNodeBech32ValConsAddress))

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return c.startWithFinalGenesis(ctx, genBz)
}

func (c *CosmosChain) startWithFinalGenesis(ctx context.Context, genbz []byte) error {
	if c.cfg.ModifyGenesis != nil {
		var err error
		genbz, err = c.cfg.ModifyGenesis(c.Config(), genbz)
		if err != nil {
			return err
		}
	}

	c.log.Info("Writing genesis and starting chain", zap.String("name", c.cfg.Name))

	// Provide EXPORT_GENESIS_FILE_PATH and EXPORT_GENESIS_CHAIN to help debug genesis file
	exportGenesis := os.Getenv("EXPORT_GENESIS_FILE_PATH")
	exportGenesisChain := os.Getenv("EXPORT_GENESIS_CHAIN")
	if exportGenesis != "" && exportGenesisChain == c.cfg.Name {
		c.log.Debug("Exporting genesis file",
			zap.String("chain", exportGenesisChain),
			zap.String("path", exportGenesis),
		)
		_ = os.WriteFile(exportGenesis, genbz, 0600)
	}

	chainNodes := c.Nodes()

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
		n := n
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
		n := n
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

	// Wait for blocks before considering the chains "started"
	return testutil.WaitForBlocks(ctx, 2, c.getFullNode())
}

// Bootstraps the chain and starts it from genesis
func (c *CosmosChain) prepNodes(ctx context.Context, skipGenTx bool, genesisAmounts [][]types.Coin, genesisSelfDelegation []types.Coin) error {
	if c.cfg.InterchainSecurityConfig.ConsumerCopyProviderKey != nil && c.Provider == nil {
		return fmt.Errorf("don't set ConsumerCopyProviderKey if it's not a consumer chain")
	}

	chainCfg := c.Config()
	configFileOverrides := chainCfg.ConfigFileOverrides

	eg := new(errgroup.Group)
	// Initialize config and sign gentx for each validator.
	for i, v := range c.Validators {
		v := v
		i := i
		v.Validator = true
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
					return fmt.Errorf("failed to modify toml config file: %w", err)
				}
			}
			if !skipGenTx {
				return v.InitValidatorGenTx(ctx, &chainCfg, genesisAmounts[i], genesisSelfDelegation[i])
			}
			return nil
		})
	}

	// Initialize config for each full node.
	for _, n := range c.FullNodes {
		n := n
		n.Validator = false
		eg.Go(func() error {
			if err := n.InitFullNodeFiles(ctx); err != nil {
				return err
			}
			for configFile, modifiedConfig := range configFileOverrides {
				modifiedToml, ok := modifiedConfig.(testutil.Toml)
				if !ok {
					return fmt.Errorf("provided toml override for file %s is of type (%T). Expected (DecodedToml)", configFile, modifiedConfig)
				}
				if err := testutil.ModifyTomlConfigFile(
					ctx,
					n.logger(),
					n.DockerClient,
					n.TestName,
					n.VolumeName,
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
	return eg.Wait()
}
