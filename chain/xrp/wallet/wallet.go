package wallet

func (w *XrpWallet) KeyName() string {
	return w.keyName
}

// Get formatted address, passing in a prefix.
func (w *XrpWallet) FormattedAddress() string {
	return w.AccountID
}

// Get mnemonic, only used for relayer wallets.
func (w *XrpWallet) Mnemonic() string {
	return ""
}

// Get Address with chain's prefix.
func (w *XrpWallet) Address() []byte {
	return []byte(w.AccountID)
}

func GetRootAccountSeed() string {
	return "snoPBrXtMeMyMHUVTgbuqAfg1SUTb"
}
