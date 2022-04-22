# IBC Test Framework

The IBC Test Framework orchestrates Go tests that utilize Docker containers for multiple
[IBC](https://docs.cosmos.network/master/ibc/overview.html)-compatible blockchains.

## Contributing

Running `make ibctest` will produce an `ibctest` binary into `./bin`.
Running that binary without any extra arguments will run a simple IBC test suite involving
the [Go Relayer](https://github.com/cosmos/relayer).
Alternatively, you can run `ibctest -matrix path/to/matrix.json` to define a set of chains to IBC-test.
See [`cmd/ibctest/README.md`](cmd/ibctest/README.md) for more details.

Note that `ibc-test-framework` is under active development
and we are not yet ready to commit to any stable APIs around the testing interfaces.

## Trophies

Significant bugs that were more easily fixed with the `ibc-test-framework`:

- [Juno network halt](https://github.com/strangelove-ventures/ibc-test-framework/pull/7)