package addresscodec

import "testing"

//nolint
func BenchmarkEncodeBase58(b *testing.B) {
	tt := []struct {
		description string
		input       []byte
	}{
		{
			description: "Benchmark XRP encode",
			input:       []byte("rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN"),
		},
	}

	for _, tc := range tt {
		b.Run(tc.description, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				EncodeBase58(tc.input)
			}
		})
	}
}

//nolint
func BenchmarkDecodeBase58(b *testing.B) {

	tt := []struct {
		description string
		input       string
	}{
		{
			description: "Benchmark XRP decode",
			input:       "s2Fku4vaPpFiqqXdAD3V5rYrSx5a9h9qvUJW3423akZSCeD",
		},
	}

	for _, tc := range tt {
		b.Run(tc.description, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				DecodeBase58(tc.input)
			}
		})
	}
}

//nolint
func BenchmarkEncodeClassicAddressFromPublicKeyHex(b *testing.B) {

	tt := []struct {
		description string
		input       string
		prefix      []byte
	}{
		{
			description: "Benchmark successful encode classic address",
			input:       "ED9434799226374926EDA3B54B1B461B4ABF7237962EAE18528FEA67595397FA32",
			prefix:      []byte{AccountAddressPrefix},
		},
		{
			description: "Benchmark unsuccessful encode classic address - invalid type prefix",
			input:       "ED9434799226374926EDA3B54B1B461B4ABF7237962EAE18528FEA67595397FA32",
			prefix:      []byte{0x00, 0x00},
		},
		{
			description: "Benchmark unsuccessful encode classic address - invalid public key",
			input:       "yurt",
			prefix:      []byte{AccountAddressPrefix},
		},
	}

	for _, tc := range tt {
		b.Run(tc.description, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				EncodeClassicAddressFromPublicKeyHex(tc.input)
			}
		})
	}

}

//nolint
func BenchmarkDecodeClassicAddressToAccountID(b *testing.B) {
	tt := []struct {
		description string
		input       string
	}{
		{
			description: "Benchmark decode classic address",
			input:       "rDTXLQ7ZKZVKz33zJbHjgVShjsBnqMBhmN",
		},
	}

	for _, tc := range tt {
		b.Run(tc.description, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				DecodeClassicAddressToAccountID(tc.input)
			}
		})
	}
}
