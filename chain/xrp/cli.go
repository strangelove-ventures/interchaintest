package xrp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (c *XrpChain) CreateValidatorKeys(ctx context.Context) error {
	//./validator-keys create_keys --keyfile /root/validator-0-keys.json
	keyfile := fmt.Sprintf("%s/validator-0-keys.json", c.HomeDir())
	cmd := []string{
		c.ValidatorKeysCli,
		"create_keys",
		"--keyfile", keyfile,
	}
	_, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return fmt.Errorf("error creating validator keys, %w", err)
	}

	cmd = []string{
		"cat", keyfile,
	}
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}
	var validatorKeyInfo ValidatorKeyOutput
	if err := json.Unmarshal(stdout, &validatorKeyInfo); err != nil {
		return err
	}
	c.ValidatorKeyInfo = &validatorKeyInfo
	return nil
}

func (c *XrpChain) CreateValidatorToken(ctx context.Context) error {
	if c.ValidatorKeyInfo == nil {
		return fmt.Errorf("validator keys not created yet, must call c.CreateValidatorKeys()")
	}
	//./validator-keys create_keys --keyfile /root/validator-0-keys.json
	keyfile := fmt.Sprintf("%s/validator-0-keys.json", c.HomeDir())
	cmd := []string{
		c.ValidatorKeysCli,
		"create_token",
		"--keyfile", keyfile,
	}
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}

	tokenSplit := strings.Split(string(stdout), "[validator_token]")
	if len(tokenSplit) != 2 {
		return fmt.Errorf("validator_token not returned, %s", string(stdout))
	}

	c.ValidatorToken = tokenSplit[1]
	return nil
}

func (c *XrpChain) CreateRippledConfig(ctx context.Context) error {
	if err := c.CreateValidatorKeys(ctx); err != nil {
		return fmt.Errorf("error creating rippled config, %w", err)
	}
	if err := c.CreateValidatorToken(ctx); err != nil {
		return fmt.Errorf("error creating rippled config, %w", err)
	}

	configDir := "config"
	cmd := []string{"mkdir", "-p", fmt.Sprintf("%s/%s", c.HomeDir(), configDir)}
	_, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}

	validatorConfig := NewValidatorConfig(c.ValidatorKeyInfo.PublicKey)
	if err := c.WriteFile(ctx, validatorConfig, "config/validators.txt"); err != nil {
		return fmt.Errorf("error writing validator.txt: %v", err)
	}

	rippledConfig := NewRippledConfig(c.ValidatorToken)
	if err := c.WriteFile(ctx, rippledConfig, "config/rippled.cfg"); err != nil {
		return fmt.Errorf("error writing rippled.cfg: %v", err)
	}

	return nil
}
