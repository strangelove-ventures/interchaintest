package main

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/localinterchain/interchain"
	"github.com/strangelove-ventures/localinterchain/interchain/types"
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
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configPath := args[0]
		parentDir := GetDirectory()

		if path.IsAbs(configPath) {
			dir, err := filepath.Abs(configPath)
			if err != nil {
				panic(err)
			}

			parentDir = dir
			configPath = filepath.Base(configPath)
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

			Relayer: types.Relayer{
				DockerImage: types.DockerImage{
					Repository: relayerImg,
					Version:    relayerVer,
					UidGid:     relayerUidGid,
				},
				StartupFlags: relayerFlags,
			},

			AuthKey: cmd.Flag(FlagAuthKey).Value.String(),
		})
	},
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
