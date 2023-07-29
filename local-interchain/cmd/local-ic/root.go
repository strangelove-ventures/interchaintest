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
	}

	if MakeFileInstallDirectory == "" {
		dirname, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}

		MakeFileInstallDirectory = path.Join(dirname, "local-interchain")
	}

	if err := directoryRequirementChecks(MakeFileInstallDirectory, "configs", "chains"); err != nil {
		log.Fatal(err)
	}

	return MakeFileInstallDirectory
}

func directoryRequirementChecks(parent string, subDirectories ...string) error {
	for _, subDirectory := range subDirectories {
		if _, err := os.Stat(path.Join(parent, subDirectory)); os.IsNotExist(err) {
			return fmt.Errorf("%s/ folder not found in %s", subDirectory, parent)
		}
	}

	return nil
}
