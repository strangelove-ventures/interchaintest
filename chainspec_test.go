package ibctest_test

import (
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/strangelove-ventures/ibctest"
	"github.com/strangelove-ventures/ibctest/ibc"
	"github.com/stretchr/testify/require"
)

func TestChainSpec_Config(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		s := ibctest.ChainSpec{
			Name: "gaia",

			Version: "v7.0.1",
		}

		_, err := s.Config()
		require.NoError(t, err)
	})

	t.Run("omit name when all other fields provided", func(t *testing.T) {
		s := ibctest.ChainSpec{
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

		_, err := s.Config()
		require.NoError(t, err)
	})

	t.Run("consistently generated config", func(t *testing.T) {
		s := ibctest.ChainSpec{
			Name: "gaia",

			Version: "v7.0.1",
		}

		cfg1, err := s.Config()
		require.NoError(t, err)

		cfg2, err := s.Config()
		require.NoError(t, err)

		diff := cmp.Diff(cfg1, cfg2)
		require.Empty(t, diff, "diff when generating config multiple times")
	})

	t.Run("name and chain ID generation", func(t *testing.T) {
		t.Run("same name and chain ID generated when ChainName and ChainID omitted", func(t *testing.T) {
			s := ibctest.ChainSpec{
				Name: "gaia",

				Version: "v7.0.1",
			}

			cfg, err := s.Config()
			require.NoError(t, err)

			require.Regexp(t, regexp.MustCompile(`^gaia-\d+$`), cfg.Name)
			require.Equal(t, cfg.Name, cfg.ChainID)
		})

		t.Run("chain ID generated from ChainName, when ChainName provided", func(t *testing.T) {
			s := ibctest.ChainSpec{
				Name:      "gaia",
				ChainName: "mychain",

				Version: "v7.0.1",
			}

			cfg, err := s.Config()
			require.NoError(t, err)

			require.Equal(t, "mychain", cfg.Name)
			require.Regexp(t, regexp.MustCompile(`^mychain-\d+$`), cfg.ChainID)
		})
	})

	t.Run("overrides", func(t *testing.T) {
		baseSpec := &ibctest.ChainSpec{
			Name:    "gaia",
			Version: "v7.0.1",

			ChainName: "g",
			ChainConfig: ibc.ChainConfig{
				ChainID: "g-0000",
			},
		}
		baseCfg, err := baseSpec.Config()
		require.NoError(t, err)

		t.Run("GasAdjustment", func(t *testing.T) {
			g := float64(1234.5)
			require.NotEqual(t, baseCfg.GasAdjustment, g)

			s := baseSpec
			s.GasAdjustment = &g

			cfg, err := s.Config()
			require.NoError(t, err)

			require.Equal(t, g, cfg.GasAdjustment)
		})

		t.Run("NoHostMount", func(t *testing.T) {
			m := true
			require.NotEqual(t, baseCfg.NoHostMount, m)

			s := baseSpec
			s.NoHostMount = &m

			cfg, err := s.Config()
			require.NoError(t, err)

			require.Equal(t, m, cfg.NoHostMount)
		})
	})

	t.Run("error cases", func(t *testing.T) {
		t.Run("version required", func(t *testing.T) {
			s := ibctest.ChainSpec{
				Name: "gaia",
			}

			_, err := s.Config()
			require.EqualError(t, err, "ChainSpec.Version must not be empty")
		})

		t.Run("name required", func(t *testing.T) {
			s := ibctest.ChainSpec{
				Version: "v1.2.3",
			}

			_, err := s.Config()
			require.EqualError(t, err, "ChainSpec.Name required when not all config fields are set")
		})

		t.Run("name invalid", func(t *testing.T) {
			s := ibctest.ChainSpec{
				Name:    "invalid_chain",
				Version: "v1.2.3",
			}

			_, err := s.Config()
			require.ErrorContains(t, err, "no chain configuration for invalid_chain (available chains are:")
		})
	})
}
