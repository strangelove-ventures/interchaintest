package utxo

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Depending on the wallet version, getwalletinfo may require a created wallet name
func (c *UtxoChain) GetWalletVersion(ctx context.Context, keyName string) (int, error) {
	var walletInfo WalletInfo
	var stdout []byte
	var err error
	
	if keyName == "" {
		cmd := append(c.BaseCli, "getwalletinfo")
		stdout, _, err = c.Exec(ctx, cmd, nil)
		if err != nil {
			return 0, err
		}
	} else {
		if err := c.LoadWallet(ctx, keyName); err != nil {
			return 0, err
		}

		cmd := append(c.BaseCli, fmt.Sprintf("-rpcwallet=%s", keyName), "getwalletinfo")
		stdout, _, err = c.Exec(ctx, cmd, nil)
		if err != nil {
			return 0, err
		}
		if err := c.UnloadWallet(ctx, keyName); err != nil {
			return 0, err
		}
	}

	if err := json.Unmarshal(stdout, &walletInfo); err != nil {
		return 0, err
	}
	
	return walletInfo.WalletVersion, nil
}

// UnloadWalletAfterUse() sets whether non-default wallets stay loaded
// Default value is false, wallets will stay loaded
// Setting this to true will load/unload a wallet for each action on a specific wallet.
// Currently, the only know case where this is required true is when using bifrost.
func (c *UtxoChain) UnloadWalletAfterUse(on bool) {
	c.unloadWalletAfterUse = on
}

func (c *UtxoChain) LoadWallet(ctx context.Context, keyName string) error {
	if !c.unloadWalletAfterUse {
		return nil
	}

	if c.WalletVersion == 0 || c.WalletVersion >= noDefaultKeyWalletVersion {
		wallet, err := c.getWallet(keyName)
		if err != nil {
			return err
		}
		wallet.mu.Lock()
		defer wallet.mu.Unlock()
		if wallet.loadCount == 0 {
			cmd := append(c.BaseCli, "loadwallet", keyName)
			_, _, err = c.Exec(ctx, cmd, nil)
			if err != nil {
				return err
			}
		}
		wallet.loadCount++
	} 
	return nil
}

func (c *UtxoChain) UnloadWallet(ctx context.Context, keyName string) error {
	if !c.unloadWalletAfterUse {
		return nil
	}

	if c.WalletVersion == 0 || c.WalletVersion >= noDefaultKeyWalletVersion {
		wallet, err := c.getWallet(keyName)
		if err != nil {
			return err
		}
		wallet.mu.Lock()
		defer wallet.mu.Unlock()
		if wallet.loadCount == 1 {
			cmd := append(c.BaseCli, "unloadwallet", keyName)
			_, _, err = c.Exec(ctx, cmd, nil)
			if err != nil {
				return err
			}
		}
		if wallet.loadCount > 0 {
			wallet.loadCount--
		}
	} 
	return nil
}

func (c *UtxoChain) CreateWallet(ctx context.Context, keyName string) error {
	if c.WalletVersion == 0 || c.WalletVersion >= noDefaultKeyWalletVersion {
		cmd := append(c.BaseCli, "createwallet", keyName)
		_, _, err := c.Exec(ctx, cmd, nil)
		if err != nil {
			return err
		}
	
		c.KeyNameToWalletMap[keyName] = &NodeWallet{
			keyName: keyName,
			loadCount: 1,
		}
	}
	
	return c.UnloadWallet(ctx, keyName)
}

func (c *UtxoChain) GetNewAddress(ctx context.Context, keyName string, mweb bool) (string, error){
	wallet, err := c.getWalletForNewAddress(keyName)
	if err != nil {
		return "", err
	}

	if err := c.LoadWallet(ctx, keyName); err != nil {
		return "", err
	}

	var cmd []string
	if c.WalletVersion >= noDefaultKeyWalletVersion {
		cmd = append(c.BaseCli, fmt.Sprintf("-rpcwallet=%s", keyName), "getnewaddress")
	} else {
		cmd = append(c.BaseCli, "getnewaddress")
	}

	if mweb {
		cmd = append(cmd, "mweb", "mweb")
	}
	
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return "", err
	}
	addr := strings.TrimSpace(string(stdout))
	
	// Remove "bchreg:" from addresses like: bchreg:qz2lxh4vzg2tqw294p7d6taxntu2snwnjuxd2k9auq
	splitAddr := strings.Split(addr, ":")
	if len(splitAddr) == 2 {
		addr = splitAddr[1]
	}
	
	wallet.address = addr
	c.AddrToKeyNameMap[addr] = keyName

	if c.WalletVersion >= noDefaultKeyWalletVersion {
		wallet.ready = true
	}

	if err := c.UnloadWallet(ctx, keyName); err != nil {
		return "", err
	}

	return addr, nil
}

func (c *UtxoChain) SetAccount(ctx context.Context, addr string, keyName string) error {
	if c.WalletVersion < noDefaultKeyWalletVersion {
		wallet, err := c.getWalletForSetAccount(keyName, addr)
		if err != nil {
			return err
		}
		cmd := append(c.BaseCli, "setaccount", addr, keyName)
		_, _, err = c.Exec(ctx, cmd, nil)
		if err != nil {
			return err
		}
		wallet.ready = true
	}

	return nil
}

// sendToMwebAddress is used for creating the mweb transaction needed at block 431
// no other use case is currently supported
func (c *UtxoChain) sendToMwebAddress(ctx context.Context, keyName string, addr string, amount float64) error {
	_, err := c.getWalletForUse(keyName)
	if err != nil {
		return err
	}

	if err := c.LoadWallet(ctx, keyName); err != nil {
		return err
	}

	cmd := append(c.BaseCli,
		fmt.Sprintf("-rpcwallet=%s", keyName), "-named", "sendtoaddress", 
		fmt.Sprintf("address=%s", addr), 
		fmt.Sprintf("amount=%.8f", amount),
	)
	
	_, _, err = c.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}
	
	return c.UnloadWallet(ctx, keyName)
}

func (c *UtxoChain) ListUnspent(ctx context.Context, keyName string) (ListUtxo, error) {
	wallet, err := c.getWalletForUse(keyName)
	if err != nil {
		return nil, err
	}
	
	if err := c.LoadWallet(ctx, keyName); err != nil {
		return nil, err
	}

	var cmd []string
	if c.WalletVersion >= noDefaultKeyWalletVersion {
		cmd = append(c.BaseCli, fmt.Sprintf("-rpcwallet=%s", keyName), "listunspent")
	} else {
		cmd = append(c.BaseCli, "listunspent", "0", "99999999", fmt.Sprintf("[\"%s\"]", wallet.address))
	}
	
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return nil, err
	}
	
	if err := c.UnloadWallet(ctx, keyName); err != nil {
		return nil, err
	}

	var listUtxo ListUtxo
	if err := json.Unmarshal(stdout, &listUtxo); err != nil {
		return nil, err
	}	

	return listUtxo, nil
}

func (c *UtxoChain) CreateRawTransaction(ctx context.Context, keyName string, listUtxo ListUtxo, addr string, sendAmount float64, script []byte) (string, error) {
	wallet, err := c.getWalletForUse(keyName)
	if err != nil {
		return "", err
	}

	var sendInputs SendInputs
	utxoTotal := float64(0.0)
	fees, err := strconv.ParseFloat(c.cfg.GasPrices, 64)
	if err != nil {
		return "", err
	}
	fees = fees * c.cfg.GasAdjustment
	for _, utxo := range listUtxo {
		if wallet.address == utxo.Address || strings.Contains(utxo.Address, wallet.address) {
			sendInputs = append(sendInputs, SendInput{
				TxId: utxo.TxId,
				Vout: utxo.Vout,
			})
			utxoTotal += utxo.Amount
			if utxoTotal > sendAmount + fees {
				break
			}
		}
	}
	sendInputsBz, err := json.Marshal(sendInputs)
	if err != nil {
		return "", err
	}

	sanitizedSendAmount, err := strconv.ParseFloat(fmt.Sprintf("%.8f", sendAmount), 64)
	if err != nil {
		return "", err
	}

	sanitizedChange, err := strconv.ParseFloat(fmt.Sprintf("%.8f", utxoTotal - sendAmount - fees), 64)
	if err != nil {
		return "", err
	}

	var sendOutputsBz []byte
	if c.WalletVersion >= noDefaultKeyWalletVersion {
		sendOutputs := SendOutputs{
			SendOutput{
				Amount: sanitizedSendAmount,
			},
			SendOutput{
				Change: sanitizedChange,
			},
		}

		if len(script) > 0 {
			sendOutputs = append(sendOutputs, SendOutput{
				Data: hex.EncodeToString(script),
			})
		}

		sendOutputsBz, err = json.Marshal(sendOutputs)
		if err != nil {
			return "", err
		}
	} else {
		sendOutputs := SendOutput{
			Amount: sanitizedSendAmount,
			Change: sanitizedChange,
			Data: hex.EncodeToString(script),
		}

		sendOutputsBz, err = json.Marshal(sendOutputs)
		if err != nil {
			return "", err
		}
	}

	// create raw transaction

	sendInputsStr := string(sendInputsBz)
	sendOutputsStr := strings.Replace(string(sendOutputsBz), "replaceWithAddress", addr, 1)
	sendOutputsStr = strings.Replace(sendOutputsStr, "replaceWithChangeAddr", wallet.address, 1)

	// createrawtransaction 
	cmd := append(c.BaseCli, 
		"createrawtransaction", fmt.Sprintf("%s", sendInputsStr), fmt.Sprintf("%s", sendOutputsStr))
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(stdout)), nil
}

func (c *UtxoChain) SignRawTransaction(ctx context.Context, keyName string, rawTxHex string) (string, error) {
	wallet, err := c.getWalletForUse(keyName)
	if err != nil {
		return "", err
	}

	var cmd []string
	if c.WalletVersion >= noDefaultKeyWalletVersion {
		cmd = append(c.BaseCli, 
			fmt.Sprintf("-rpcwallet=%s", keyName), "signrawtransactionwithwallet", rawTxHex)
	} else {
		// export priv key of sending address
		cmd = append(c.BaseCli, "dumpprivkey", wallet.address)
		stdout, _, err := c.Exec(ctx, cmd, nil)
		if err != nil {
			return "", err
		}

		// sign raw tx with priv key
		cmd = append(c.BaseCli, 
			"signrawtransaction", rawTxHex, "null", fmt.Sprintf("[\"%s\"]",
			strings.TrimSpace(string(stdout))))
	}
	
	if err := c.LoadWallet(ctx, keyName); err != nil {
		return "", err
	}

	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return "", err
	}
	
	if err := c.UnloadWallet(ctx, keyName); err != nil {
		return "", err
	}

	var signRawTxOutput SignRawTxOutput
	if err := json.Unmarshal(stdout, &signRawTxOutput); err != nil {
		return "", err
	}

	if !signRawTxOutput.Complete {
		c.logger().Error(fmt.Sprintf("Signing transaction did not complete, (%d) errors", len(signRawTxOutput.Errors)))
		for i, sErr := range signRawTxOutput.Errors {
			c.logger().Error(fmt.Sprintf("Signing error %d: %s", i, sErr.Error))
		}
		return "", fmt.Errorf("Sign transaction error on %s", c.cfg.Name)
	}
	
	return signRawTxOutput.Hex, nil
}

func (c *UtxoChain) SendRawTransaction(ctx context.Context, signedRawTxHex string) (string, error) {
	cmd := []string{}
	if c.WalletVersion >= namedFixWalletVersion {
		cmd = append(c.BaseCli, "sendrawtransaction", signedRawTxHex)
	} else if c.WalletVersion > noDefaultKeyWalletVersion {
		cmd = append(c.BaseCli, "sendrawtransaction", signedRawTxHex, "0")
	} else {
		cmd = append(c.BaseCli, "sendrawtransaction", signedRawTxHex)
	}
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(stdout)), nil
}