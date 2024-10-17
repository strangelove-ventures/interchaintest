package main

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/spf13/cobra"
)

// This must be global for the Makefile to build properly (ldflags).
var MakeFileInstallDirectory string

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
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	p := path.Join(cwd, "chains")

	// if the current directory has the 'chains' folder, use that. If not, use the default location.
	if f, _ := os.Stat(p); f != nil && f.IsDir() {
		files, err := os.ReadDir(p)
		if err != nil {
			log.Fatal(err)
		}

		if len(files) > 0 {
			return cwd
		}
	}

	// Config variable override for the ICTEST_HOME
	if res := os.Getenv("ICTEST_HOME"); res != "" {
		MakeFileInstallDirectory = res
	}

	if MakeFileInstallDirectory == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}

		MakeFileInstallDirectory = path.Join(homeDir, "local-interchain")
	}

	if err := directoryRequirementChecks(MakeFileInstallDirectory, "chains"); err != nil {
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
