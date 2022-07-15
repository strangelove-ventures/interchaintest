# ibctest

[![Go Reference](https://pkg.go.dev/badge/github.com/strangelove-ventures/ibctest@main.svg)](https://pkg.go.dev/github.com/strangelove-ventures/ibctest@main)

ibctest orchestrates Go tests that utilize Docker containers for multiple
[IBC](https://docs.cosmos.network/master/ibc/overview.html)-compatible blockchains.

## Focusing on Specific Tests

You may focus on a specific tests using the `-test.run=<regex>` flag.

```shell
ibctest -test.run=/<test category>/<chain combination>/<relayer>/<test subcategory>/<test name>
```

If you want to focus on a specific test:

```shell
ibctest -test.run=/////relay_packet
ibctest -test.run=/////no_timeout
ibctest -test.run=/////height_timeout
ibctest -test.run=/////timestamp_timeout
```

Example of narrowing your focus even more:

```shell
# run all tests for Go relayer
ibctest -test.run=///rly/

# run all tests for Go relayer and gaia chains
ibctest -test.run=//gaia/rly/

# only run no_timeout test for Go relayer and gaia chains
ibctest -test.run=//gaia/rly/conformance/no_timeout
```

## Retaining data on failed tests

By default, failed tests will clean up any temporary directories they created.
Sometimes when debugging a failed test, it can be more helpful to leave the directory behind
for further manual inspection.

Setting the environment variable `IBCTEST_SKIP_FAILURE_CLEANUP` to any non-empty value
will cause the test to skip deletion of the temporary directories.
Any tests that fail and skip cleanup will log a message like
`Not removing temporary directory for test at: /tmp/...`.

Test authors must use
[`ibctest.TempDir`](https://pkg.go.dev/github.com/strangelove-ventures/ibctest#TempDir)
instead of `(*testing.T).Cleanup` to opt in to this behavior.

By default, Docker volumes associated with tests are cleaned up at the end of each test run.
That same `IBCTEST_SKIP_FAILURE_CLEANUP` controls whether the volumes associated with failed tests are pruned.

## Contributing

Running `make ibctest` will produce an `ibctest` binary into `./bin`.
Running that binary without any extra arguments will run a simple IBC test suite involving
the [Go Relayer](https://github.com/cosmos/relayer).
Alternatively, you can run `ibctest -matrix path/to/matrix.json` to define a set of chains to IBC-test.
See [`cmd/ibctest/README.md`](cmd/ibctest/README.md) for more details.

Note that `ibctest` is under active development
and we are not yet ready to commit to any stable APIs around the testing interfaces.

Please read the [logging style guide](./docs/logging.md).

## Trophies

Significant bugs that were more easily fixed with `ibctest`:

- [Juno network halt reproduction](https://github.com/strangelove-ventures/ibctest/pull/7)
- [Juno network halt fix confirmation](https://github.com/strangelove-ventures/ibctest/pull/8)