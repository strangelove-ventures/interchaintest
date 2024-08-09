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
		cmd := append(c.BaseCli, "loadwallet", keyName)
		_, _, err := c.Exec(ctx, cmd, nil)
		if err != nil {
			return err
		}
	} 
	return nil
}

func (c *UtxoChain) UnloadWallet(ctx context.Context, keyName string) error {
	if !c.unloadWalletAfterUse {
		return nil
	}

	if c.WalletVersion == 0 || c.WalletVersion >= noDefaultKeyWalletVersion {
		cmd := append(c.BaseCli, "unloadwallet", keyName)
		_, _, err := c.Exec(ctx, cmd, nil)
		if err != nil {
			return err
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
	}

	return c.UnloadWallet(ctx, keyName)
}

func (c *UtxoChain) GetNewAddress(ctx context.Context, keyName string, mweb bool) (string, error){
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
	
	c.AddrToWalletMap[addr] = keyName
	c.WalletToAddrMap[keyName] = addr

	if err := c.UnloadWallet(ctx, keyName); err != nil {
		return "", nil
	}

	return addr, nil
}

func (c *UtxoChain) SetAccount(ctx context.Context, addr string, keyName string) error {
	if c.WalletVersion < noDefaultKeyWalletVersion {
		cmd := append(c.BaseCli, "setaccount", addr, keyName)
		_, _, err := c.Exec(ctx, cmd, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *UtxoChain) SendToAddress(ctx context.Context, keyName string, addr string, amount float64) error {
	if err := c.LoadWallet(ctx, keyName); err != nil {
		return err
	}

	cmd := []string{}
	if c.WalletVersion >= namedFixWalletVersion {
		cmd = append(c.BaseCli,
			fmt.Sprintf("-rpcwallet=%s", keyName), "-named", "sendtoaddress", 
			fmt.Sprintf("address=%s", addr), 
			fmt.Sprintf("amount=%.8f", amount),
		)
	} else if c.WalletVersion >= noDefaultKeyWalletVersion {
		cmd = append(c.BaseCli, 
			fmt.Sprintf("-rpcwallet=%s", keyName), "-named", "sendtoaddress", 
			addr,
			fmt.Sprintf("%.8f", amount),
		)
	} else {
		cmd = append(c.BaseCli,
			"sendfrom",
			keyName,
			addr, 
			fmt.Sprintf("%.8f", amount),
		)
	}
	
	_, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return err
	}

	return c.UnloadWallet(ctx, keyName)
}

func (c *UtxoChain) ListUnspent(ctx context.Context, keyName string) (ListUtxo, error) {
	cmd := append(c.BaseCli, fmt.Sprintf("-rpcwallet=%s", keyName), "listunspent")
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return nil, err
	}

	var listUtxo ListUtxo
	if err := json.Unmarshal(stdout, &listUtxo); err != nil {
		return nil, err
	}

	return listUtxo, nil
}

func (c *UtxoChain) CreateRawTransaction(ctx context.Context, keyName string, listUtxo ListUtxo, addr string, sendAmount float64, script []byte) (string, error) {
	var sendInputs SendInputs
	utxoTotal := float64(0.0)
	fees, err := strconv.ParseFloat(c.cfg.GasPrices, 64)
	if err != nil {
		return "", err
	}
	fees = fees * c.cfg.GasAdjustment
	for _, utxo := range listUtxo {
		sendInputs = append(sendInputs, SendInput{
			TxId: utxo.TxId,
			Vout: utxo.Vout,
		})
		utxoTotal += utxo.Amount
		if utxoTotal > sendAmount + fees {
			break
		}
	}

	sanitizedSendAmount, err := strconv.ParseFloat(fmt.Sprintf("%.8f", sendAmount), 64)
	if err != nil {
		return "", err
	}

	sanitizedChange, err := strconv.ParseFloat(fmt.Sprintf("%.8f", utxoTotal - sendAmount - fees), 64)
	if err != nil {
		return "", err
	}

	sendOutputs := SendOutputs{
		SendOutput{
			Amount: sanitizedSendAmount,
		},
		SendOutput{
			Change: sanitizedChange,
		},
		SendOutput{
			Data: hex.EncodeToString(script),
		},
	}

	// create raw transaction
	sendInputsBz, err := json.Marshal(sendInputs)
	if err != nil {
		return "", err
	}

	sendOutputsBz, err := json.Marshal(sendOutputs)
	if err != nil {
		return "", err
	}

	sendInputsStr := string(sendInputsBz)
	sendOutputsStr := strings.Replace(string(sendOutputsBz), "replaceWithAddress", addr, 1)
	sendOutputsStr = strings.Replace(sendOutputsStr, "replaceWithChangeAddr", c.WalletToAddrMap[keyName], 1)

	fmt.Println("SendFundsWithNote inputs", sendInputsStr)
	fmt.Println("SendFundsWithNote outputs", sendOutputsStr)

	// createrawtransaction 
	cmd := append(c.BaseCli, 
		"createrawtransaction", fmt.Sprintf("%s", sendInputsStr), fmt.Sprintf("%s", sendOutputsStr))
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return "", err
	}

	rawTxHex := strings.TrimSpace(string(stdout))

	fmt.Println("SendFundsWithNote rawtxHex", rawTxHex)

	rawTxDecoded, err := hex.DecodeString(rawTxHex)
	if err != nil {
		return "", err
	}

	fmt.Println("SendFundsWithNote rawTx decoded:", string(rawTxDecoded))

	return rawTxHex, nil
}

func (c *UtxoChain) SignRawTransaction(ctx context.Context, keyName string, rawTxHex string) (string, error) {
	cmd := append(c.BaseCli, 
		fmt.Sprintf("-rpcwallet=%s", keyName), "signrawtransactionwithwallet", rawTxHex)
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return "", err
	}

	var signRawTxOutput SignRawTxOutput
	if err := json.Unmarshal(stdout, &signRawTxOutput); err != nil {
		return "", err
	}

	if signRawTxOutput.Complete {
		fmt.Println("Signing of transaction complete!")
	} else {
		fmt.Println("Signing of tx incomplete")
		fmt.Println("Number of errors", len(signRawTxOutput.Errors))
		for i, sErr := range signRawTxOutput.Errors {
			fmt.Println("Signing error", i, ":", sErr.Error)
		}
		return "", fmt.Errorf("Signing error")
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
		cmd = append(c.BaseCli, "sendrawtransaction", signedRawTxHex, "0", "1", "3", "4")
	}
	stdout, _, err := c.Exec(ctx, cmd, nil)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(stdout)), nil
}