package testutil

import (
	"bytes"
	"context"
	"fmt"
	"reflect"

	"github.com/BurntSushi/toml"
	"github.com/docker/docker/client"
	"go.uber.org/zap"

	"github.com/strangelove-ventures/interchaintest/v9/dockerutil"
)

// Toml is used for holding the decoded state of a toml config file.
type Toml map[string]any

// RecursiveModifyToml will apply toml modifications at the current depth,
// then recurse for new depths.
func RecursiveModifyToml(c map[string]any, modifications Toml) error {
	for key, value := range modifications {
		if reflect.ValueOf(value).Kind() == reflect.Map {
			cV, ok := c[key]
			if !ok {
				// Did not find section in existing config, populating fresh.
				cV = make(Toml)
			}
			// Retrieve existing config to apply overrides to.
			cVM, ok := cV.(map[string]any)
			if !ok {
				// if the config does not exist, we should create a blank one to allow creation
				cVM = make(Toml)
			}
			if err := RecursiveModifyToml(cVM, value.(Toml)); err != nil {
				return err
			}
			c[key] = cVM
		} else {
			// Not a map, so we can set override value directly.
			c[key] = value
		}
	}
	return nil
}

// ModifyTomlConfigFile reads, modifies, then overwrites a toml config file, useful for config.toml, app.toml, etc.
func ModifyTomlConfigFile(
	ctx context.Context,
	logger *zap.Logger,
	dockerClient *client.Client,
	testName string,
	volumeName string,
	filePath string,
	modifications Toml,
) error {
	fr := dockerutil.NewFileRetriever(logger, dockerClient, testName)
	config, err := fr.SingleFileContent(ctx, volumeName, filePath)
	if err != nil {
		return fmt.Errorf("failed to retrieve %s: %w", filePath, err)
	}

	var c Toml
	if err := toml.Unmarshal(config, &c); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", filePath, err)
	}

	if err := RecursiveModifyToml(c, modifications); err != nil {
		return fmt.Errorf("failed to modify %s: %w", filePath, err)
	}

	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(c); err != nil {
		return fmt.Errorf("failed to encode %s: %w", filePath, err)
	}

	fw := dockerutil.NewFileWriter(logger, dockerClient, testName)
	if err := fw.WriteFile(ctx, volumeName, filePath, buf.Bytes()); err != nil {
		return fmt.Errorf("overwriting %s: %w", filePath, err)
	}

	return nil
}
