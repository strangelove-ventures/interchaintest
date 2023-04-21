package polkadot

import (
	"encoding/hex"
	"fmt"

	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
)

const (
	Ss58Format = 49
	ss58Prefix = "SS58PRE"
)

func EncodeAddressSS58(key []byte) (string, error) {
	input := []byte{Ss58Format}
	input = append(input, key...)

	checksum, err := ss58Checksum(input)
	if err != nil {
		return "", err
	}

	final := input
	if len(key) == 32 || len(key) == 33 {
		final = append(final, checksum[0:2]...)
	} else {
		final = append(final, checksum[0:1]...)
	}

	return base58.Encode(final), nil
}

// Decodes an address to public key, refactored from https://github.com/subscan-explorer/subscan-essentials
func DecodeAddressSS58(address string) ([]byte, error) {
	checksumPrefix := []byte(ss58Prefix)
	ss58AddrDecoded, err := base58.Decode(address)
	if len(ss58AddrDecoded) == 0 || ss58AddrDecoded[0] != byte(Ss58Format) || err != nil {
		return nil, err
	}
	var checksumLength int
	if IntInSlice(len(ss58AddrDecoded), []int{3, 4, 6, 10}) {
		checksumLength = 1
	} else if IntInSlice(len(ss58AddrDecoded), []int{5, 7, 11, 35}) {
		checksumLength = 2
	} else if IntInSlice(len(ss58AddrDecoded), []int{8, 12}) {
		checksumLength = 3
	} else if IntInSlice(len(ss58AddrDecoded), []int{9, 13}) {
		checksumLength = 4
	} else if IntInSlice(len(ss58AddrDecoded), []int{14}) {
		checksumLength = 5
	} else if IntInSlice(len(ss58AddrDecoded), []int{15}) {
		checksumLength = 6
	} else if IntInSlice(len(ss58AddrDecoded), []int{16}) {
		checksumLength = 7
	} else if IntInSlice(len(ss58AddrDecoded), []int{17}) {
		checksumLength = 8
	} else {
		return nil, fmt.Errorf("Cannot get checksum length")
	}
	bss := ss58AddrDecoded[0 : len(ss58AddrDecoded)-checksumLength]
	checksum, _ := blake2b.New(64, []byte{})
	w := append(checksumPrefix[:], bss[:]...)
	_, err = checksum.Write(w)
	if err != nil {
		return nil, err
	}

	h := checksum.Sum(nil)
	if BytesToHex(h[0:checksumLength]) != BytesToHex(ss58AddrDecoded[len(ss58AddrDecoded)-checksumLength:]) {
		return nil, fmt.Errorf("Checksum incorrect")
	}
	return ss58AddrDecoded[1 : len(ss58AddrDecoded)-checksumLength], nil
}

func BytesToHex(b []byte) string {
	c := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(c, b)
	return string(c)
}

func IntInSlice(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func ss58Checksum(data []byte) ([]byte, error) {
	hasher, err := blake2b.New512(nil)
	if err != nil {
		return nil, err
	}

	if _, err := hasher.Write([]byte(ss58Prefix)); err != nil {
		return nil, err
	}

	if _, err := hasher.Write(data); err != nil {
		return nil, err
	}

	return hasher.Sum(nil), nil
}
