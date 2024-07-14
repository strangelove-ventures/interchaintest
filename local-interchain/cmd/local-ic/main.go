package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// set in the Makefile
var Version = ""

func main() {
	rootCmd.AddCommand(chainsCmd)
	rootCmd.AddCommand(newChainCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(interactCmd)
	rootCmd.AddCommand(&cobra.Command{
		Use:     "version",
		Aliases: []string{"ver"},
		Short:   "Print the version of Local-IC",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print(Version)
		},
	})

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error while executing your CLI. Err: %v\n", err)
		os.Exit(1)
	}
}
