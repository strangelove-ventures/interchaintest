package xrp

type ValidatorKeyOutput struct {
	KeyType string  `json:"address"`
	PublicKey string `json:"public_key"`
	Revoked bool `json:"revoked"`
	SecretKey string `json:"secret_key"`
	TokenSequence int `json:"token_sequence"`
}

type WalletResponse struct {
    Result struct {
        AccountID     string `json:"account_id"`
        KeyType       string `json:"key_type"`
        MasterKey     string `json:"master_key"`
        MasterSeed    string `json:"master_seed"`
        MasterSeedHex string `json:"master_seed_hex"`
        PublicKey     string `json:"public_key"`
        PublicKeyHex  string `json:"public_key_hex"`
        Status        string `json:"status"`
    } `json:"result"`
}
