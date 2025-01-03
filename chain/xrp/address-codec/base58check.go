package addresscodec

import (
	"crypto/sha256"
	"errors"
)

// ErrChecksum indicates that the checksum of a check-encoded string does not verify against
// the checksum.
// ErrInvalidFormat indicates that the check-encoded string has an invalid format.
var (
	ErrChecksum      = errors.New("checksum error")
	ErrInvalidFormat = errors.New("invalid format: version and/or checksum bytes missing")
)

// checksum: first four bytes of sha256^2
func checksum(input []byte) (cksum [4]byte) {
	h := sha256.Sum256(input)
	h2 := sha256.Sum256(h[:])
	copy(cksum[:], h2[:4])
	return cksum
}

// CheckEncode prepends a version byte, appends a four byte checksum and returns
// a base58 encoding of the byte slice.
func Base58CheckEncode(input []byte, prefix ...byte) string {
	b := make([]byte, 0, 1+len(input)+4)
	b = append(b, prefix...)
	b = append(b, input...)

	cksum := checksum(b)
	b = append(b, cksum[:]...)
	return EncodeBase58(b)
}

// CheckDecode decodes a string that was encoded with CheckEncode and verifies the checksum.
func Base58CheckDecode(input string) (result []byte, err error) {
	decoded := DecodeBase58(input)
	if len(decoded) < 5 {
		return nil, ErrInvalidFormat
	}

	var cksum [4]byte
	copy(cksum[:], decoded[len(decoded)-4:])
	if checksum(decoded[:len(decoded)-4]) != cksum {
		return nil, ErrChecksum
	}

	result = decoded[:len(decoded)-4]
	return
}
