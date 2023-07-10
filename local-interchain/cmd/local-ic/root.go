package main

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/spf13/cobra"
)

var (
	MakeFileInstallDirectory string
)

var rootCmd = &cobra.Command{
	Use:   "local-ic",
	Short: "Your local IBC interchain of nodes program",
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.Help(); err != nil {
			log.Fatal(err)
		}
	},
}

func GetDirectory() string {
	// Config variable override for the ICTEST_HOME
	if res := os.Getenv("ICTEST_HOME"); res != "" {
		MakeFileInstallDirectory = res
		return res
	}

	if MakeFileInstallDirectory == "" {
		dirname, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(dirname)

		MakeFileInstallDirectory = path.Join(dirname, "local-interchain")
	}

	return MakeFileInstallDirectory
}
