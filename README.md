# ibctest

[![Go Reference](https://pkg.go.dev/badge/github.com/strangelove-ventures/ibctest@main.svg)](https://pkg.go.dev/github.com/strangelove-ventures/ibctest@main)

`ibctest` orchestrates Go tests that utilize Docker containers for multiple
[IBC](https://docs.cosmos.network/master/ibc/overview.html)-compatible blockchains.

It allows users to quickly spin up custom testnets and dev environments to test IBC and chain infrastructures.

## Building Binary

While it is not necessary to build the binary, sometimes it can be more convenient, specifically when running conformance test with custom chain sets (details below). 

To build binary:

```shell
git clone https://github.com/strangelove-ventures/ibctest.git
cd ibctest
make ibctest
```

This places the binary in `ibctest/.bin/ibctest`


Note that this is not in your Go path.

## Conformance Tests

`ibctest` comes with a suite of conformance tests. These tests ensure IBC and relayer compatibility on a high level. It test things such as `client`, `channel`, and `connection` creation. It ensure that messages are properly relayed and acknowledged across chain pairs. 

You can view all the specific conformance test by reviewing them in the [conformance](./conformance/) folder.


To run them from the binary:
```shell
ibctest
```


To run straight from source code:
```shell
go test -v ./cmd/ibctest/
```

**The benefit of running from a binary is that you can easily pass in custom chain pairs and custom settings about the testing environment.**

This is accomplished via the `-matrix <path/to/matrix.json>` argument. 

Passing in a matrix file you can easily customize these aspects of the environment:
- chain pairs
- number of validators
- number of full nodes
- relayer tech (currently only integrated with [Go Relayer](https://github.com/cosmos/relayer))


`ibctest` comes with several [pre-configured chains](./docs/preconfiguredChains.txt). 

Note that the docker images for these pre-configured chains are being pulled from [Heighliner](https://github.com/strangelove-ventures/heighliner) (repository of docker images of many IBC enabled chains). Heighliner needs to have the `Version` you are requesting.


Here is an example of a matrix file using pre-configured chains: [example_matrix.json](./cmd/ibctest/example_matrix.json)


Here is an example of a more customized matrix file [example_matrix_custom.json](./cmd/ibctest/example_matrix_custom.json)


**Local Docker Images**
You can supply local docker images in the matrix file by:

```yaml
...
        Images: []ibc.DockerImage{
            {
                Repository: "simd", // local docker image name
                Version: "v1.0.0",	// docker tag 
            },
...
```

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