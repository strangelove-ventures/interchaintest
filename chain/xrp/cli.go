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
	
//mkdir -p ~/xrpl-private-network/validator_1/config; cd into this
	configDir := "config"
	cmd := []string{"mkdir", "-p", fmt.Sprintf("%s/%s", c.HomeDir(), configDir)}
	_, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}

//add validators.txt
	// validatorConfig := ValidatorConfig{
	// 	Validators: []string{c.ValidatorKeyInfo.PublicKey},
	// }
	// // Marshal the config to TOML
	// validatorConfigToml, err := toml.Marshal(validatorConfig)
	// if err != nil {
	// 	return fmt.Errorf("failed to marshal validator config: %v", err)
	// }
	validatorConfig := NewValidatorConfig(c.ValidatorKeyInfo.PublicKey)
	fmt.Println("validator.txt:", string(validatorConfig))
	if err := c.WriteFile(ctx, validatorConfig, "config/validators.txt"); err != nil {
		return fmt.Errorf("error writing validator.txt: %v", err)
	}

// add rippled.cfg
	// rippledConfig := NewDefaultRippledConfig()
	// rippledConfig.ValidatorToken = c.ValidatorToken
	// rippledConfigToml, err := toml.Marshal(rippledConfig)
	// if err != nil {
	// 	return fmt.Errorf("failed to marshal rippled config: %v", err)
	// }
	rippledConfig := NewRippledConfig(c.ValidatorToken)
	fmt.Println("rippled.cfg:", string(rippledConfig))
	if err := c.WriteFile(ctx, rippledConfig, "config/rippled.cfg"); err != nil {
		return fmt.Errorf("error writing rippled.cfg: %v", err)
	}

	return nil
}

// func (c *XrpChain) GetServerInfo(ctx context.Context) (*xrpclient.ServerInfoResponse, error) {
// 	cmd := []string{c.RippledCli, "--conf", fmt.Sprintf("%s/config/rippled.cfg", c.HomeDir()), "server_info"}
// 	stdout, _, err := c.Exec(ctx, cmd, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	fmt.Println("server info:", string(stdout))

// 	var serverInfo xrpclient.ServerInfoResponse
// 	if err := json.Unmarshal(stdout, &serverInfo); err != nil {
// 		return nil, fmt.Errorf("error unmarshal server info, %v", err)
// 	}

// 	return &serverInfo, nil
// }

// func (c *XrpChain) GetAccountInfo(ctx context.Context, address string) (*AccountInfoResponse, error) {
// 	cmd := []string{c.RippledCli, "--conf", fmt.Sprintf("%s/config/rippled.cfg", c.HomeDir()), "account_info", address}
// 	stdout, _, err := c.Exec(ctx, cmd, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	fmt.Println("account info:", string(stdout))

// 	var accountInfo AccountInfoResponse
// 	if err := json.Unmarshal(stdout, &accountInfo); err != nil {
// 		return nil, fmt.Errorf("error unmarshal account info, %v", err)
// 	}

// 	return &accountInfo, nil
// }

// func (c *XrpChain) CreateNewAccount(ctx context.Context) (*WalletResponse, error) {
// 	cmd := []string{c.RippledCli, "--conf", fmt.Sprintf("%s/config/rippled.cfg", c.HomeDir()), "wallet_propose"}
// 	stdout, _, err := c.Exec(ctx, cmd, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	fmt.Println("wallet info:", string(stdout))

// 	var walletInfo WalletResponse
// 	if err := json.Unmarshal(stdout, &walletInfo); err != nil {
// 		return nil, fmt.Errorf("error unmarshal wallet info, %v", err)
// 	}

// 	return &walletInfo, nil
// }

// func (c *XrpChain) GetRootAccount(ctx context.Context, keyName string) (*XrpWallet, error) {
// 	// cmd := []string{c.RippledCli, "wallet_propose", "masterpassphrase"}
// 	// stdout, _, err := c.Exec(ctx, cmd, nil)
// 	// if err != nil {
// 	// 	return nil, err
// 	// }
// 	// fmt.Println("wallet info:", string(stdout))

// 	// stdout := []byte("{
// 	// 	"result" : {
// 	// 	   "account_id" : "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh",
// 	// 	   "key_type" : "secp256k1",
// 	// 	   "master_key" : "I IRE BOND BOW TRIO LAID SEAT GOAL HEN IBIS IBIS DARE",
// 	// 	   "master_seed" : "snoPBrXtMeMyMHUVTgbuqAfg1SUTb",
// 	// 	   "master_seed_hex" : "DEDCE9CE67B451D852FD4E846FCDE31C",
// 	// 	   "public_key" : "aBQG8RQAzjs1eTKFEAQXr2gS4utcDiEC9wmi7pfUPTi27VCahwgw",
// 	// 	   "public_key_hex" : "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020",
// 	// 	   "status" : "success",
// 	// 	   "warning" : "This wallet was generated using a user-supplied passphrase that has low entropy and is vulnerable to brute-force attacks."
// 	// 	}
// 	//  }")
// 	//  stdout := "{
// 	// 	\"result\" : {
// 	// 	   \"account_id\" : \"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh\",
// 	// 	   \"key_type\" : \"secp256k1\",
// 	// 	   \"master_key\" : \"I IRE BOND BOW TRIO LAID SEAT GOAL HEN IBIS IBIS DARE\",
// 	// 	   \"master_seed\" : \"snoPBrXtMeMyMHUVTgbuqAfg1SUTb\",
// 	// 	   \"master_seed_hex\" : \"DEDCE9CE67B451D852FD4E846FCDE31C\",
// 	// 	   \"public_key\" : \"aBQG8RQAzjs1eTKFEAQXr2gS4utcDiEC9wmi7pfUPTi27VCahwgw\",
// 	// 	   \"public_key_hex\" : \"0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020\",
// 	// 	   \"status\" : \"success\",
// 	// 	   \"warning\" : \"This wallet was generated using a user-supplied passphrase that has low entropy and is vulnerable to brute-force attacks.\"
// 	// 	}
// 	//  }"
// 	// stdout := "{
// 	// 	\"result\" : {
// 	// 	   \"account_id\" : \"rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh\",
// 	// 	   \"key_type\" : \"secp256k1\",
// 	// 	   \"master_key\" : \"I IRE BOND BOW TRIO LAID SEAT GOAL HEN IBIS IBIS DARE\",
// 	// 	   \"master_seed\" : \"snoPBrXtMeMyMHUVTgbuqAfg1SUTb\",
// 	// 	   \"master_seed_hex\" : \"DEDCE9CE67B451D852FD4E846FCDE31C\",
// 	// 	   \"public_key\" : \"aBQG8RQAzjs1eTKFEAQXr2gS4utcDiEC9wmi7pfUPTi27VCahwgw\",
// 	// 	   \"public_key_hex\" : \"0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020\",
// 	// 	   \"status\" : \"success\",
// 	// 	   \"warning\" : \"This wallet was generated using a user-supplied passphrase that has low entropy and is vulnerable to brute-force attacks.\"
// 	// 	}
// 	//  }"
// 	// walletInfo := WalletResponse{}
// 	// walletInfo.Result.AccountID = "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"
// 	// walletInfo.Result.KeyType = "secp256k1"
// 	// walletInfo.Result.MasterKey = "I IRE BOND BOW TRIO LAID SEAT GOAL HEN IBIS IBIS DARE"
// 	// walletInfo.Result.MasterSeed = "snoPBrXtMeMyMHUVTgbuqAfg1SUTb"
// 	// walletInfo.Result.MasterSeedHex = "DEDCE9CE67B451D852FD4E846FCDE31C"
// 	// walletInfo.Result.PublicKey = "aBQG8RQAzjs1eTKFEAQXr2gS4utcDiEC9wmi7pfUPTi27VCahwgw"
// 	// walletInfo.Result.PublicKeyHex = "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020"
// 	// walletInfo.Result.Status = "success"

// 	rootAccount := NewWallet(keyName)
// 	rootAccount.AccountID = "rHb9CJAWyB4rj91VRWn96DkukG4bwdtyTh"
// 	rootAccount.KeyType = "secp256k1"
// 	rootAccount.MasterKey = "I IRE BOND BOW TRIO LAID SEAT GOAL HEN IBIS IBIS DARE"
// 	rootAccount.MasterSeed = "snoPBrXtMeMyMHUVTgbuqAfg1SUTb"
// 	rootAccount.MasterSeedHex = "DEDCE9CE67B451D852FD4E846FCDE31C"
// 	rootAccount.PublicKey = "aBQG8RQAzjs1eTKFEAQXr2gS4utcDiEC9wmi7pfUPTi27VCahwgw"
// 	rootAccount.PublicKeyHex = "0330E7FC9D56BB25D6893BA3F317AE5BCF33B3291BD63DB32654A313222F7FD020"
// 	rootAccount.Status = "success"	

// 	return rootAccount, nil
// }

// func (c *XrpChain) GetNewAddress(ctx context.Context, keyName string) (string, error) {
	
// }
