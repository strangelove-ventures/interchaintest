package main

import (
	"fmt"
	"os"
)

func main() {
	rootCmd.AddCommand(chainsCmd)
	rootCmd.AddCommand(newChainCmd)
	rootCmd.AddCommand(startCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error while executing your CLI. Err: %v\n", err)
		os.Exit(1)
	}
}
