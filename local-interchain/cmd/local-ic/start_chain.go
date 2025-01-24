package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/interchaintest/local-interchain/interchain"
	"github.com/strangelove-ventures/interchaintest/local-interchain/interchain/types"
	"github.com/strangelove-ventures/interchaintest/v9/ibc"
)

const (
	FlagAPIAddressOverride = "api-address"
	FlagAPIPortOverride    = "api-port"

	FlagRelayerImage        = "relayer-image"
	FlagRelayerVersion      = "relayer-version"
	FlagRelayerUidGid       = "relayer-uidgid"
	FlagRelayerStartupFlags = "relayer-startup-flags"
	FlagAuthKey             = "auth-key"
)

var startCmd = &cobra.Command{
	Use:     "start <config.json>",
	Aliases: []string{"s", "run"},
	Short:   "Starts up the chain of choice with the config name",
	Example: `local-ic start base_ibc
ICTEST_HOME=. local-ic start mychain
local-ic start https://gist.githubusercontent.com/Reecepbcups/70bf59c82c797ead9a5430b8b9d8d852/raw/cecc7be35bcec8b976a5d92e78fd6d56de2e1aa1/cosmoshub_localic_config.json
local-ic start https://pastebin.com/raw/Ummk4DTM
`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configPath := args[0]
		isURL := strings.HasPrefix(configPath, "http")

		var (
			parentDir string
			config    *types.Config
			err       error
		)

		if path.IsAbs(configPath) {
			dir, err := filepath.Abs(configPath)
			if err != nil {
				panic(err)
			}

			parentDir = dir
			configPath = filepath.Base(configPath)
		}

		if isURL {
			config, err = interchain.LoadConfigFromURL(configPath)
			if err != nil {
				panic(err)
			}

			// last part of the URL to be the test name
			configPath = configPath[strings.LastIndex(configPath, "/")+1:]
		} else {
			parentDir = GetDirectory()

			configPath, err = GetConfigWithExtension(parentDir, configPath)
			if err != nil {
				panic(err)
			}

			config, err = interchain.LoadConfig(parentDir, configPath)
			if err != nil {
				panic(err)
			}
		}

		apiAddr, _ := cmd.Flags().GetString(FlagAPIAddressOverride)
		apiPort, _ := cmd.Flags().GetUint16(FlagAPIPortOverride)

		relayerImg := cmd.Flag(FlagRelayerImage).Value.String()
		relayerVer := cmd.Flag(FlagRelayerVersion).Value.String()
		relayerUidGid := cmd.Flag(FlagRelayerUidGid).Value.String()
		relayerFlags := strings.Split(cmd.Flag(FlagRelayerStartupFlags).Value.String(), " ")

		interchain.StartChain(parentDir, configPath, &types.AppStartConfig{
			Address: apiAddr,
			Port:    apiPort,
			Cfg:     config,

			Relayer: types.Relayer{
				DockerImage: ibc.DockerImage{
					Repository: relayerImg,
					Version:    relayerVer,
					UIDGID:     relayerUidGid,
				},
				StartupFlags: relayerFlags,
			},

			AuthKey: cmd.Flag(FlagAuthKey).Value.String(),
		})
	},
}

// GetConfigWithExtension returns the config with the file extension attached if one was not provided.
// If "hub" is passed it, it will search for hub.yaml, hub.yml, or hub.json.
// If an extension is already applied, it will use that.
func GetConfigWithExtension(parentDir, config string) (string, error) {
	if path.Ext(config) != "" {
		return config, nil
	}

	extensions := []string{".yaml", ".yml", ".json"}
	for _, ext := range extensions {
		fp := path.Join(parentDir, interchain.ChainDir, config+ext)
		if _, err := os.Stat(fp); err != nil {
			continue
		}

		return config + ext, nil
	}

	return "", fmt.Errorf("could not find a file with an accepted extension: %s. (%+v)", config, extensions)
}

func init() {
	startCmd.Flags().String(FlagAPIAddressOverride, "127.0.0.1", "override the default API address")
	startCmd.Flags().Uint16(FlagAPIPortOverride, 8080, "override the default API port")

	startCmd.Flags().String(FlagRelayerImage, "ghcr.io/cosmos/relayer", "override the docker relayer image")
	startCmd.Flags().String(FlagRelayerVersion, "latest", "override the default relayer version")
	startCmd.Flags().String(FlagRelayerUidGid, "100:1000", "override the default image UID:GID")
	startCmd.Flags().String(FlagRelayerStartupFlags, "--block-history=100", "override the default relayer startup flags")

	startCmd.Flags().String(FlagAuthKey, "", "require an auth key to use the internal API")
}
