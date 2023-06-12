package interchaintest

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"go.uber.org/zap"
)

// ChainSpec is a wrapper around an ibc.ChainConfig
// that allows callers to easily reference one of the built-in chain configs
// and optionally provide overrides for some settings.
type ChainSpec struct {
	// Name is the name of the built-in config to use as a basis for this chain spec.
	// Required unless every other field is set.
	Name string

	// ChainName sets the Name of the embedded ibc.ChainConfig, i.e. the name of the chain.
	ChainName string

	// Version of the docker image to use.
	// Must be set.
	Version string

	// GasAdjustment and NoHostMount are pointers in ChainSpec
	// so zero-overrides can be detected from omitted overrides.
	GasAdjustment *float64
	NoHostMount   *bool

	// Embedded ChainConfig to allow for simple JSON definition of a ChainSpec.
	ibc.ChainConfig

	// How many validators and how many full nodes to use
	// when instantiating the chain.
	// If unspecified, NumValidators defaults to 2 and NumFullNodes defaults to 1.
	NumValidators, NumFullNodes *int

	// Generate the automatic suffix on demand when needed.
	autoSuffixOnce sync.Once
	autoSuffix     string
}

// Config returns the underlying ChainConfig,
// with any overrides applied.
func (s *ChainSpec) Config(log *zap.Logger) (*ibc.ChainConfig, error) {
	if s.Version == "" {
		// Version must be set at top-level if not set in inlined config.
		if len(s.ChainConfig.Images) == 0 || s.ChainConfig.Images[0].Version == "" {
			return nil, errors.New("ChainSpec.Version must not be empty")
		}
	}

	// s.Name and chainConfig.Name are interchangeable
	if s.Name == "" && s.ChainConfig.Name != "" {
		s.Name = s.ChainConfig.Name
	} else if s.Name != "" && s.ChainConfig.Name == "" {
		s.ChainConfig.Name = s.Name
	}

	// Empty name is only valid with a fully defined chain config.
	if s.Name == "" {
		// If ChainName is provided and ChainConfig.Name is not set, set it.
		if s.ChainConfig.Name == "" && s.ChainName != "" {
			s.ChainConfig.Name = s.ChainName
		}
		if !s.ChainConfig.IsFullyConfigured() {
			return nil, errors.New("ChainSpec.Name required when not all config fields are set")
		}

		return s.applyConfigOverrides(s.ChainConfig)
	}

	builtinChainConfigs, err := initBuiltinChainConfig(log)
	if err != nil {
		return nil, fmt.Errorf("failed to get pre-configured chains: %w", err)
	}

	// Get built-in config.
	// If chain doesn't have built in config, but is fully configured, register chain label.
	cfg, ok := builtinChainConfigs[s.Name]
	if !ok {
		if !s.ChainConfig.IsFullyConfigured() {
			availableChains := make([]string, 0, len(builtinChainConfigs))
			for k := range builtinChainConfigs {
				availableChains = append(availableChains, k)
			}
			sort.Strings(availableChains)

			return nil, fmt.Errorf("no chain configuration for %s (available chains are: %s)", s.Name, strings.Join(availableChains, ", "))
		}
		cfg = ibc.ChainConfig{}
	}

	cfg = cfg.Clone()

	// Apply any overrides from this ChainSpec.
	cfg = cfg.MergeChainSpecConfig(s.ChainConfig)

	coinType, err := cfg.VerifyCoinType()
	if err != nil {
		return nil, err
	}
	cfg.CoinType = coinType

	// Apply remaining top-level overrides.
	return s.applyConfigOverrides(cfg)
}

func (s *ChainSpec) applyConfigOverrides(cfg ibc.ChainConfig) (*ibc.ChainConfig, error) {
	// If no ChainName provided, generate one based on the spec name.
	cfg.Name = s.ChainName
	if cfg.Name == "" {
		cfg.Name = s.Name + s.suffix()
	}

	// If no ChainID provided, generate one -- prefer chain name but fall back to spec name.
	if cfg.ChainID == "" {
		prefix := s.ChainName
		if prefix == "" {
			prefix = s.Name
		}
		cfg.ChainID = prefix + s.suffix()
	}

	if s.GasAdjustment != nil {
		cfg.GasAdjustment = *s.GasAdjustment
	}
	if s.NoHostMount != nil {
		cfg.NoHostMount = *s.NoHostMount
	}
	if s.SkipGenTx {
		cfg.SkipGenTx = true
	}
	if s.ModifyGenesis != nil {
		cfg.ModifyGenesis = s.ModifyGenesis
	}
	if s.PreGenesis != nil {
		cfg.PreGenesis = s.PreGenesis
	}
	cfg.UsingNewGenesisCommand = s.UsingNewGenesisCommand

	// Set the version depending on the chain type.
	switch cfg.Type {
	case "cosmos":
		if s.Version != "" && len(cfg.Images) > 0 {
			cfg.Images[0].Version = s.Version
		}
	case "penumbra":
		versionSplit := strings.Split(s.Version, ",")
		if len(versionSplit) != 2 {
			return nil, errors.New("penumbra version should be comma separated penumbra_version,tendermint_version")
		}
		cfg.Images[0].Version = versionSplit[1]
		cfg.Images[1].Version = versionSplit[0]
	case "polkadot":
		// Only set if ChainSpec's Version is set, if not, Version from Images must be set.
		if s.Version != "" {
			versionSplit := strings.Split(s.Version, ",")
			relayChainImageSplit := strings.Split(versionSplit[0], ":")
			var relayChainVersion string
			if len(relayChainImageSplit) > 1 {
				if relayChainImageSplit[0] != "seunlanlege/centauri-polkadot" &&
					relayChainImageSplit[0] != "polkadot" {
					return nil, fmt.Errorf("only polkadot is supported as the relay chain node. got: %s", relayChainImageSplit[0])
				}
				relayChainVersion = relayChainImageSplit[1]
			} else {
				relayChainVersion = relayChainImageSplit[0]
			}
			cfg.Images[0].Version = relayChainVersion
			switch {
			case strings.Contains(s.Name, "composable"):
				if len(versionSplit) != 2 {
					return nil, fmt.Errorf("unexpected composable version: %s. should be comma separated polkadot:version,composable:version", s.Version)
				}
				imageSplit := strings.Split(versionSplit[1], ":")
				if len(imageSplit) != 2 {
					return nil, fmt.Errorf("parachain versions should be in the format parachain_name:parachain_version, got: %s", versionSplit[1])
				}
				if !strings.Contains(cfg.Images[1].Repository, imageSplit[0]) {
					return nil, fmt.Errorf("unexpected parachain: %s", imageSplit[0])
				}
				cfg.Images[1].Version = imageSplit[1]
			default:
				return nil, fmt.Errorf("unexpected parachain: %s", s.Name)
			}
		} else {
			// Ensure there are at least two images and check the 2nd version is populated
			if len(s.ChainConfig.Images) < 2 || s.ChainConfig.Images[1].Version == "" {
				return nil, fmt.Errorf("ChainCongfig.Images must be >1 and ChainConfig.Images[1].Version must not be empty")
			}
		}
	}

	return &cfg, nil
}

// suffix returns the automatically generated, concurrency-safe suffix for
// generating a chain name or chain ID.
func (s *ChainSpec) suffix() string {
	s.autoSuffixOnce.Do(func() {
		s.autoSuffix = fmt.Sprintf("-%d", atomic.AddInt32(&suffixCounter, 1))
	})

	return s.autoSuffix
}

// suffixCounter is a package-level counter for safely generating unique suffixes per execution environment.
var suffixCounter int32
