# ibctest

This directory contains a test that can be parameterized at runtime,
allowing a user to pick and choose what combinations of relayers and chains to test.

You can run the tests during development with `go test`,
but for general distribution, you would generate the executable with `go test -c`.

The test binary supports a `-matrix` flag.
See `example_matrix.json` for an example of what this can look like using the test chains included in this repository.
See `example_matrix_custom.json` for an example of what this can look like using full chain config customization.
You may need to reference the `testMatrix` type in `ibc_test.go`.
