<div align="center">
<h1>ibctest</h1>

[![Go Reference](https://pkg.go.dev/badge/github.com/strangelove-ventures/ibctest@main.svg)](https://pkg.go.dev/github.com/strangelove-ventures/ibctest@main)
[![License: Apache-2.0](https://img.shields.io/github/license/strangelove-ventures/ibctest.svg?style=flat-square)](https://github.com/strangelove-ventures/ibctest/blob/main/create-test-readme/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/strangelove-ventures/ibctest)](https://goreportcard.com/report/github.com/strangelove-ventures/ibctest)



`ibctest` orchestrates Go tests that utilize Docker containers for multiple
[IBC](https://docs.cosmos.network/master/ibc/overview.html)-compatible blockchains.

It allows users to quickly spin up custom testnets and dev environments to test IBC and chain infrastructures.
</div>


**Features:**
- Built-in suite of conformance tests to test high-level IBC compatibility between chain sets.
- Easily construct customized tests in highly configurable environments
- Deployable as CI tests in production workflows


## Table Of Contents
- [Building Binary](#building-binary)
- **Usage:**
    - [Running Conformance Tests](./docs/conformanceTests.md) - suite of built-in tests that test high-level IBC compatibility
    - [Architect Custom Tests](./docs/architectCustomTests.md) - How to create custom tests
- [Retaining Data on Failed Tests](./docs/retainingDataOnFailedTests.md)
- [Deploy as GitHub CI tests] (./docs/ciTests.md) - Docs WIP


## Building Binary

While it is not necessary to build the binary, sometimes it can be more convenient, *specifically* when running conformance test with custom chain sets. 

Building binary:
```shell
git clone https://github.com/strangelove-ventures/ibctest.git
cd ibctest
make ibctest
```

This places the binary in `ibctest/.bin/ibctest`

Note that this is not in your Go path.


## Contributing

Contributing is encouraged.

Please read the [logging style guide](./docs/logging.md).

## Trophies

Significant bugs that were more easily fixed with `ibctest`:

- [Juno network halt reproduction](https://github.com/strangelove-ventures/ibctest/pull/7)
- [Juno network halt fix confirmation](https://github.com/strangelove-ventures/ibctest/pull/8)