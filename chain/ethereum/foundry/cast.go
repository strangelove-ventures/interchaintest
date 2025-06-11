package foundry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// cast send 
func (c *AnvilChain) CastSend(ctx context.Context, keyName string, params []string) (string, error) {
	cmd := []string{"cast", "send"}
	cmd = append(cmd, params...)

	c.MapAccess.Lock()
	account, ok := c.keystoreMap[keyName]
	c.MapAccess.Unlock()
	if !ok {
		return "", fmt.Errorf("keyname (%s) not found", keyName)
	}
	cmd = append(cmd,
		"--keystore", account.keystore,
		"--password", "",
		"--rpc-url", c.GetRPCAddress(),
	)

	skipTxHash := true
	for _, param := range params {
		if param == "--json" {
			skipTxHash = false
		}
	}

	account.txLock.Lock()
	defer account.txLock.Unlock()
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return "", fmt.Errorf("cast send, exec, %w", err)
	}

	if skipTxHash {
		return "", nil
	}

	var txReceipt TransactionReceipt
	if err = json.Unmarshal([]byte(strings.TrimSpace(string(stdout))), &txReceipt); err != nil {
		return "", fmt.Errorf("tx receipt unmarshal:\n %s\nerror: %w", string(stdout), err)
	}

	return txReceipt.TxHash, nil
}

// cast call
func (c *AnvilChain) CastCall(ctx context.Context, params []string) (string, error) {
	cmd := []string{"cast", "call"}
	cmd = append(cmd, "--rpc-url", c.GetRPCAddress())
	cmd = append(cmd, params...)

	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return "", fmt.Errorf("cast call, exec, %w", err)
	}

	return string(stdout), nil
}

// cast code <CONTRACT_ADDR> --rpc-url http://ethereum-anvil-31337:8545
func (c *AnvilChain) CastCode(ctx context.Context, contract string) (string, error) {
	cmd := []string{"cast", "code", contract, "--rpc-url", c.GetRPCAddress()}
	
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return "", fmt.Errorf("cast code, exec, %w", err)
	}

	return string(stdout), nil
}

//cast block latest --full --rpc-url YOUR_RPC_URL
func (c *AnvilChain) CastBlock(ctx context.Context) ([]byte, error) {
	cmd := []string{"cast", "block", "latest", "--full", "--rpc-url", c.GetRPCAddress(), "--json"}
	
	stdout, _, err := c.Exec(ctx, cmd, []string{"CAST_FULL_BLOCK=true"})
	if err != nil {
		return nil, fmt.Errorf("cast block, exec, %w", err)
	}

	return stdout, nil
}

//cast interface 0xYourAddress --rpc-url YOUR_RPC_URL
func (c *AnvilChain) CastInterface(ctx context.Context, contract string) (string, error) {
	cmd := []string{"cast", "interface", contract, "--rpc-url", c.GetRPCAddress(), "--json"}
	
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return "", fmt.Errorf("cast interface, exec, %w", err)
	}

	return string(stdout), nil
}
