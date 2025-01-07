// MIT License

// Copyright (c) 2022 xyield

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package base58

import (
	"math/big"
)

var bigRadix = [...]*big.Int{
	big.NewInt(0),
	big.NewInt(58),
	big.NewInt(58 * 58),
	big.NewInt(58 * 58 * 58),
	big.NewInt(58 * 58 * 58 * 58),
	big.NewInt(58 * 58 * 58 * 58 * 58),
	big.NewInt(58 * 58 * 58 * 58 * 58 * 58),
	big.NewInt(58 * 58 * 58 * 58 * 58 * 58 * 58),
	big.NewInt(58 * 58 * 58 * 58 * 58 * 58 * 58 * 58),
	big.NewInt(58 * 58 * 58 * 58 * 58 * 58 * 58 * 58 * 58),
	bigRadix10,
}

var bigRadix10 = big.NewInt(58 * 58 * 58 * 58 * 58 * 58 * 58 * 58 * 58 * 58)

// Encodes a byte slice to a base58 string encoded with the XRP alphabet.
func Encode(b []byte) string {
	x := new(big.Int)
	x.SetBytes(b)

	// Maximum length of output is log58(2^(8*len(b))) == len(b) * 8 / log(58)
	maxlen := int(float64(len(b))*1.365658237309761) + 1
	answer := make([]byte, 0, maxlen)
	mod := new(big.Int)

	for x.Sign() > 0 {
		// Calculating with big.Int is slow for each iteration.
		//    x, mod = x / 58, x % 58
		//
		// Instead we can try to do as much calculations on int64.
		//    x, mod = x / 58^10, x % 58^10
		//
		// Which will give us mod, which is 10 digit base58 number.
		// We'll loop that 10 times to convert to the answer.

		x.DivMod(x, bigRadix10, mod)

		if x.Sign() == 0 {
			// When x = 0, we need to ensure we don't add any extra zeros.
			m := mod.Int64()
			for m > 0 {
				answer = append(answer, xrpAlphabet[m%58])
				m /= 58
			}
		} else {
			m := mod.Int64()
			for i := 0; i < 10; i++ {
				answer = append(answer, xrpAlphabet[m%58])
				m /= 58
			}
		}
	}
	// leading zero bytes
	for _, i := range b {
		if i != 0 {
			break
		}
		answer = append(answer, alphabetIdx0)
	}

	// reverse
	alen := len(answer)
	for i := 0; i < alen/2; i++ {
		answer[i], answer[alen-1-i] = answer[alen-1-i], answer[i]
	}

	return string(answer)
}

// Decodes a modified base58 string to a byte slice.
func Decode(b string) []byte {
	answer := big.NewInt(0)
	scratch := new(big.Int)

	// Calculating with big.Int is slow for each iteration.
	//    x += b58[b[i]] * j
	//    j *= 58
	//
	// Instead we can try to do as much calculations on int64.
	// We can represent a 10 digit base58 number using an int64.
	//
	// Hence we'll try to convert 10, base58 digits at a time.
	// The rough idea is to calculate `t`, such that:
	//
	//   t := b58[b[i+9]] * 58^9 ... + b58[b[i+1]] * 58^1 + b58[b[i]] * 58^0
	//   x *= 58^10
	//   x += t
	//
	// Of course, in addition, we'll need to handle boundary condition when `b` is not multiple of 58^10.
	// In that case we'll use the bigRadix[n] lookup for the appropriate power.

	for t := b; len(t) > 0; {
		n := len(t)
		if n > 10 {
			n = 10
		}

		total := uint64(0)
		for _, v := range t[:n] {
			tmp := b58[v]
			if tmp == 255 {
				return []byte("")
			}
			total = total*58 + uint64(tmp)
		}

		answer.Mul(answer, bigRadix[n])
		scratch.SetUint64(total)
		answer.Add(answer, scratch)

		t = t[n:]
	}

	tmpval := answer.Bytes()

	var numZeros int
	for numZeros = 0; numZeros < len(b); numZeros++ {
		if b[numZeros] != alphabetIdx0 {
			break
		}
	}
	flen := numZeros + len(tmpval)
	val := make([]byte, flen)
	copy(val[numZeros:], tmpval)

	return val
}
