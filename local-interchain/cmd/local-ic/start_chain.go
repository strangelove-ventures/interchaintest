package main

import (
	"path"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/strangelove-ventures/localinterchain/interchain"
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

		interchain.StartChain(parentDir, configPath)
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
