package interchaintest_test

import (
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	interchaintest "github.com/strangelove-ventures/interchaintest/v7"
	"github.com/strangelove-ventures/interchaintest/v7/ibc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestChainSpec_Config(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		s := interchaintest.ChainSpec{
			Name: "gaia",

			Version: "v7.0.1",
		}

		_, err := s.Config(zaptest.NewLogger(t))
		require.NoError(t, err)
	})

	t.Run("omit name when all other fields provided", func(t *testing.T) {
		s := interchaintest.ChainSpec{
			ChainName: "mychain",

			ChainConfig: ibc.ChainConfig{
				Type: "cosmos",
				// Skip Name, as that is intended to be inherited from ChainName.
				ChainID: "mychain-123",
				Images: []ibc.DockerImage{
					{Repository: "docker.example.com", Version: "latest"},
				},
				Bin:            "/bin/true",
				Bech32Prefix:   "foo",
				Denom:          "bar",
				GasPrices:      "1bar",
				GasAdjustment:  2,
				TrustingPeriod: "24h",
			},
		}

		_, err := s.Config(zaptest.NewLogger(t))
		require.NoError(t, err)
	})

	t.Run("consistently generated config", func(t *testing.T) {
		s := interchaintest.ChainSpec{
			Name: "gaia",

			Version: "v7.0.1",
		}

		cfg1, err := s.Config(zaptest.NewLogger(t))
		require.NoError(t, err)

		cfg2, err := s.Config(zaptest.NewLogger(t))
		require.NoError(t, err)

		diff := cmp.Diff(cfg1, cfg2)
		require.Empty(t, diff, "diff when generating config multiple times")
	})

	t.Run("name and chain ID generation", func(t *testing.T) {
		t.Run("same name and chain ID generated when ChainName and ChainID omitted", func(t *testing.T) {
			s := interchaintest.ChainSpec{
				Name: "gaia",

				Version: "v7.0.1",
			}

			cfg, err := s.Config(zaptest.NewLogger(t))
			require.NoError(t, err)

			require.Regexp(t, regexp.MustCompile(`^gaia-\d+$`), cfg.Name)
			require.Equal(t, cfg.Name, cfg.ChainID)
		})

		t.Run("chain ID generated from ChainName, when ChainName provided", func(t *testing.T) {
			s := interchaintest.ChainSpec{
				Name:      "gaia",
				ChainName: "mychain",

				Version: "v7.0.1",
			}

			cfg, err := s.Config(zaptest.NewLogger(t))
			require.NoError(t, err)

			require.Equal(t, "mychain", cfg.Name)
			require.Regexp(t, regexp.MustCompile(`^mychain-\d+$`), cfg.ChainID)
		})
	})

	t.Run("overrides", func(t *testing.T) {
		baseSpec := &interchaintest.ChainSpec{
			Name:    "gaia",
			Version: "v7.0.1",

			ChainName: "g",
			ChainConfig: ibc.ChainConfig{
				ChainID: "g-0000",
			},
		}
		baseCfg, err := baseSpec.Config(zaptest.NewLogger(t))
		require.NoError(t, err)

		t.Run("NoHostMount", func(t *testing.T) {
			m := true
			require.NotEqual(t, baseCfg.NoHostMount, m)

			s := baseSpec
			s.NoHostMount = &m

			cfg, err := s.Config(zaptest.NewLogger(t))
			require.NoError(t, err)

			require.Equal(t, m, cfg.NoHostMount)
		})
	})

	t.Run("error cases", func(t *testing.T) {
		t.Run("version required", func(t *testing.T) {
			s := interchaintest.ChainSpec{
				Name: "gaia",
			}

			_, err := s.Config(zaptest.NewLogger(t))
			require.EqualError(t, err, "ChainSpec.Version must not be empty")
		})

		t.Run("name required", func(t *testing.T) {
			s := interchaintest.ChainSpec{
				Version: "v1.2.3",
			}

			_, err := s.Config(zaptest.NewLogger(t))
			require.EqualError(t, err, "ChainSpec.Name required when not all config fields are set")
		})

		t.Run("name invalid", func(t *testing.T) {
			s := interchaintest.ChainSpec{
				Name:    "invalid_chain",
				Version: "v1.2.3",
			}

			_, err := s.Config(zaptest.NewLogger(t))
			require.ErrorContains(t, err, "no chain configuration for invalid_chain (available chains are:")
		})
	})
}
