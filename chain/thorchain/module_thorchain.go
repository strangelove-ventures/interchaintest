package thorchain

import (
	"context"
	"fmt"

	"cosmossdk.io/math"

	"github.com/strangelove-ventures/interchaintest/v8/ibc"
)

// BankSend sends tokens from one account to another.
func (tn *ChainNode) BankSend(ctx context.Context, keyName string, amount ibc.WalletAmount) error {
	_, err := tn.ExecTx(ctx,
		keyName, "thorchain", "send",
		amount.Address, fmt.Sprintf("%s%s", amount.Amount.String(), amount.Denom),
	)
	return err
}

// BankSendWithNote sends tokens from one account to another with a note/memo.
func (tn *ChainNode) BankSendWithNote(ctx context.Context, keyName string, amount ibc.WalletAmount, note string) (string, error) {
	return tn.ExecTx(ctx,
		keyName, "thorchain", "send",
		amount.Address, fmt.Sprintf("%s%s", amount.Amount.String(), amount.Denom),
		"--note", note,
	)
}

func (c *Thorchain) Deposit(ctx context.Context, keyName string, amount math.Int, denom string, memo string) error {
	_, err := c.getFullNode().ExecTx(ctx,
		keyName, "thorchain", "deposit",
		amount.String(), denom, memo,
	)
	return err
}

func (c *Thorchain) SetMimir(ctx context.Context, keyName string, key string, value string) error {
	_, err := c.getFullNode().ExecTx(ctx,
		keyName, "thorchain", "mimir", key, value,
	)
	return err
}

func (tn *ChainNode) Bond(ctx context.Context, amount math.Int) error {
	_, err := tn.ExecTx(ctx,
		valKey, "thorchain", "deposit",
		amount.String(), tn.Chain.Config().Denom,
		fmt.Sprintf("bond:%s", tn.NodeAccount.NodeAddress),
	)
	return err
}

// Sets validator node keys, must be called by validator.
func (tn *ChainNode) SetNodeKeys(ctx context.Context) error {
	_, err := tn.ExecTx(ctx,
		valKey, "thorchain", "set-node-keys",
		tn.NodeAccount.PubKeySet.Secp256k1, tn.NodeAccount.PubKeySet.Ed25519, tn.NodeAccount.ValidatorConsPubKey,
	)
	return err
}

// Sets validator ip address, must be called by validator.
func (tn *ChainNode) SetIpAddress(ctx context.Context) error {
	_, err := tn.ExecTx(ctx,
		valKey, "thorchain", "set-ip-address", tn.NodeAccount.IPAddress,
	)
	return err
}

// Sets validator's binary version.
func (tn *ChainNode) SetVersion(ctx context.Context) error {
	_, err := tn.ExecTx(ctx,
		valKey, "thorchain", "set-version",
	)
	return err
}
