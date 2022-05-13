# IBC Test Framework

The IBC Test Framework orchestrates Go tests that utilize Docker containers for multiple
[IBC](https://docs.cosmos.network/master/ibc/overview.html)-compatible blockchains.

## Focusing on Specific Tests

You may focus on a specific tests using the `-test.run=<regex>` flag.

```shell
ibctest -test.run=/<relayer>/<chain combination>/<test name>
```

If you want to focus on a specific test:

```shell
ibctest -test.run=///relay_packet
ibctest -test.run=///no_timeout
ibctest -test.run=///height_timeout
ibctest -test.run=///timestamp_timeout
```

Example of narrowing your focus even more:

```shell
# run all tests for Go relayer
ibctest -test.run=/rly//

# run all tests for Go relayer and gaia chains
ibctest -test.run=/rly/gaia/

# only run no_timeout test for Go relayer and gaia chains
ibctest -test.run=/rly/gaia/no_timeout
```

## Contributing

Running `make ibctest` will produce an `ibctest` binary into `./bin`.
Running that binary without any extra arguments will run a simple IBC test suite involving
the [Go Relayer](https://github.com/cosmos/relayer).
Alternatively, you can run `ibctest -matrix path/to/matrix.json` to define a set of chains to IBC-test.
See [`cmd/ibctest/README.md`](cmd/ibctest/README.md) for more details.

Note that `ibc-test-framework` is under active development
and we are not yet ready to commit to any stable APIs around the testing interfaces.

Please read the [logging style guide](./docs/logging.md).

## Trophies

Significant bugs that were more easily fixed with the `ibc-test-framework`:

- [Juno network halt reproduction](https://github.com/strangelove-ventures/ibc-test-framework/pull/7)
- [Juno network halt fix confirmation](https://github.com/strangelove-ventures/ibc-test-framework/pull/8)