package relayer

//go:generate go run golang.org/x/tools/cmd/stringer -type=Capability

// While the relayer capability type may have made a little more sense inside the interchaintest package,
// we would expect individual relayer implementations to specify their own capabilities.
// The interchaintest package depends on the relayer implementations,
// therefore the relayer capability type exists here to avoid a circular dependency.

// Capability indicates a relayer's support of a given feature.
type Capability int

// The list of relayer capabilities that interchaintest understands.
const (
	TimestampTimeout Capability = iota
	HeightTimeout

	// Whether the relayer supports a one-off flush command.
	Flush
)

// FullCapabilities returns a mapping of all known relayer features to true,
// indicating that all features are supported.
// FullCapabilities returns a new map every time it is called,
// so callers are free to set one value to false if they support everything but one or two features.
func FullCapabilities() map[Capability]bool {
	return map[Capability]bool{
		TimestampTimeout: true,
		HeightTimeout:    true,

		Flush: true,
	}
}
