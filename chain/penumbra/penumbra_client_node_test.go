package penumbra

import (
	"math/big"
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

func TestBigIntDecoding(t *testing.T) {
	bigInt := math.NewInt(11235813)
	hi, lo := translateBigInt(bigInt)
	converted := translateHiAndLo(hi, lo)
	require.True(t, bigInt.Equal(converted))

	b := big.NewInt(0)
	b.SetString("18446744073709551620", 10) // use a number that is bigger than the maximum value an uint64 can hold
	bInt := math.NewIntFromBigInt(b)
	hi, lo = translateBigInt(bInt)
	converted = translateHiAndLo(hi, lo)
	require.True(t, converted.Equal(bInt))
}
