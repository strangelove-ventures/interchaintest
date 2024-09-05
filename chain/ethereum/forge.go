package ethereum

import (
	"context"
	"os"
	"path"
	"path/filepath"

	"github.com/docker/docker/api/types/mount"
	"github.com/strangelove-ventures/interchaintest/v8/dockerutil"
)

// cli options for the `forge script` command
// see: https://book.getfoundry.sh/reference/forge/forge-script
type ForgeScriptOpts struct {
	ContractRootDir  string   // required, root directory of the contract with all local dependencies
	SolidityContract string   // required, contract script to run
	SignatureFn      string   // optional, signature function to run, empty string uses default run()
	ConfigFile       string   // optional, json config file used for sol contract
	RawOptions       []string // optional, appends additional options to command
}

// Add private-key or keystore to cmd
func (c *EthereumChain) AddKey(cmd []string, keyName string) []string {
	// choose whether to use private-key or keystore
	if keyName == "faucet" {
		cmd = append(cmd,
			"--private-key", "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		)
	} else {
		cmd = append(cmd,
			"--keystores", c.keystoreMap[keyName],
			"--password", "",
		)
	}
	return cmd
}

// Add signature function to cmd, if present
func AddSignature(cmd []string, signature string) []string {
	if signature != "" {
		cmd = append(cmd, "--sig", signature)
	}
	return cmd
}

func GetConfigFilePath(configFile, localContractRootDir, solidityContractDir string) string {
	return filepath.Join(localContractRootDir, solidityContractDir, configFile)
}

// ReadAndAppendConfigFile, returns the cmd, configFileBz
func ReadAndAppendConfigFile(cmd []string, configFile, localContractRootDir, solidityContractDir string) ([]string, []byte, error) {
	// if config file is present, read the file and add it to cmd, after running, overwrite the results
	if configFile != "" {
		configFilePath := GetConfigFilePath(configFile, localContractRootDir, solidityContractDir)
		configFileBz, err := os.ReadFile(configFilePath)
		if err != nil {
			return nil, nil, err
		}
		cmd = append(cmd, "--", configFile)
		return cmd, configFileBz, err
	}
	return cmd, nil, nil
}

// WriteConfigFile - if config file is present, we need to overwrite what forge changed
func WriteConfigFile(configFile string, localContractRootDir string, solidityContractDir string, configFileBz []byte) error {
	if configFile != "" {
		configFilePath := GetConfigFilePath(configFile, localContractRootDir, solidityContractDir)
		err := os.WriteFile(configFilePath, configFileBz, 0o644)
		if err != nil {
			return err
		}
	}
	return nil
}

// Run "forge script"
// see: https://book.getfoundry.sh/reference/forge/forge-script
func (c *EthereumChain) ForgeScript(ctx context.Context, keyName string, opts ForgeScriptOpts) (stdout, stderr []byte, err error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, nil, err
	}
	localContractRootDir := filepath.Join(pwd, opts.ContractRootDir)
	dockerContractRootDir := c.HomeDir() + path.Base(opts.ContractRootDir)

	// Assemble cmd
	cmd := []string{"forge", "script", opts.SolidityContract, "--rpc-url", c.GetRPCAddress(), "--broadcast"}
	cmd = c.AddKey(cmd, keyName)
	cmd = AddSignature(cmd, opts.SignatureFn)
	cmd = append(cmd, opts.RawOptions...)
	cmd, configFileBz, err := ReadAndAppendConfigFile(cmd, opts.ConfigFile, localContractRootDir, path.Dir(opts.SolidityContract))
	if err != nil {
		return nil, nil, err
	}

	job := dockerutil.NewImage(c.logger(), c.DockerClient, c.NetworkID, c.testName, c.cfg.Images[0].Repository, c.cfg.Images[0].Version)
	containerOpts := dockerutil.ContainerOptions{
		Binds: c.Bind(),
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: localContractRootDir,
				Target: dockerContractRootDir,
			},
		},
		WorkingDir: dockerContractRootDir,
	}
	res := job.Run(ctx, cmd, containerOpts)

	err = WriteConfigFile(opts.ConfigFile, localContractRootDir, path.Dir(opts.SolidityContract), configFileBz)
	if err != nil {
		return nil, nil, err
	}

	return res.Stdout, res.Stderr, res.Err
}
