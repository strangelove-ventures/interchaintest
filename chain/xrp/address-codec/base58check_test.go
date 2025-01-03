//go:build unit
// +build unit

package addresscodec

import (
	"bytes"
	"testing"
)

var tt = []struct {
	prefix []byte
	in     []byte
	out    string
}{
	{[]byte{0x00}, []byte{136, 165, 165, 124, 130, 159, 64, 242, 94, 168, 51, 133, 187, 222, 108, 61, 139, 76, 160, 130}, "rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN"},
	{[]byte{SECP256K1}, []byte("testingsomething"), "shKMVJjV52uudwfS7HzzaiwmZqVeP"},
}

func TestCheckEncode(t *testing.T) {

	for x, tc := range tt {
		// test encoding
		if res := Base58CheckEncode([]byte(tc.in), tc.prefix...); res != tc.out {
			t.Errorf("CheckEncode test #%d failed: got %s, want: %s", x, res, tc.out)
		}
	}
}

func TestCheckDecode(t *testing.T) {

	for x, tc := range tt {

		res, err := Base58CheckDecode(tc.out)
		res = res[1:]
		switch {
		case err != nil:
			t.Errorf("CheckDecode test #%d failed with err: %v", x, err)

		case !bytes.Equal(res, tc.in):
			t.Errorf("CheckDecode test #%d failed: got: %s want: %s", x, res, tc.in)
		}

		// test the two decoding failure cases
		// case 1: checksum error
		_, err = Base58CheckDecode("3MNQE1Y")
		if err != ErrChecksum {
			t.Error("Checkdecode test failed, expected ErrChecksum")
		}
		// case 2: invalid formats (string lengths below 5 mean the version byte and/or the checksum
		// bytes are missing).
		testString := ""
		for len := 0; len < 4; len++ {
			testString += "x"
			_, err = Base58CheckDecode(testString)
			if err != ErrInvalidFormat {
				t.Error("Checkdecode test failed, expected ErrInvalidFormat")
			}
		}
	}
}
