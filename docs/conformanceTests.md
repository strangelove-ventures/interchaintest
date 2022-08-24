# Conformance Tests

`ibctest` comes with a suite of conformance tests. These tests ensure IBC and relayer compatibility on a high level. It tests things such as `client`, `channel`, and `connection` creation. It ensure that messages are properly relayed and acknowledged and that packets are being properly timed out.

You can view all the specific conformance test by reviewing them in the [conformance](../conformance/) folder.


To run them from the binary, simply run the binary without any extra arguments:
```shell
ibctest
```

To run straight from source code:
```shell
go test -v ./cmd/ibctest/
```

**The benefit of running from a binary is that you can easily pass in custom chain pairs and custom settings about the testing environment.**

This is accomplished via the `-matrix` argument. 
```shell
ibctest -matrix <path/to/matrix.json>
```

By passing in a matrix file you can easily customize these aspects of the environment:
- chain pairs
- number of validators
- number of full nodes
- relayer tech (currently only integrated with [Go Relayer](https://github.com/cosmos/relayer))


`ibctest` comes with several [pre-configured chains](./preconfiguredChains.txt). 

Note that the docker images for these pre-configured chains are being pulled from [Heighliner](https://github.com/strangelove-ventures/heighliner) (repository of docker images of many IBC enabled chains). Heighliner needs to have the `Version` you are requesting.

**Example Matrix Files:**
- [example_matrix.json](../cmd/ibctest/example_matrix.json) - Basic example using pre-configured chains
- [example_matrix_custom.json](../cmd/ibctest/example_matrix_custom.json) - More customized example pointing to specific chain binary docker images


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