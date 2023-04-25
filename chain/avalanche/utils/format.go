package utils

import (
	"errors"
	"fmt"

	"github.com/cosmos/btcutil/bech32"
)

var (
	errBits8To5 = errors.New("unable to convert address from 8-bit to 5-bit formatting")
)

// FormatBech32 takes an address's bytes as input and returns a bech32 address
func FormatBech32(hrp string, payload []byte) (string, error) {
	fiveBits, err := bech32.ConvertBits(payload, 8, 5, true)
	if err != nil {
		return "", errBits8To5
	}
	return bech32.Encode(hrp, fiveBits)
}

// Format takes in a chain prefix, HRP, and byte slice to produce a string for
// an address.
func Format(chainIDAlias string, hrp string, addr []byte) (string, error) {
	addrStr, err := FormatBech32(hrp, addr)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s", chainIDAlias, addrStr), nil
}
