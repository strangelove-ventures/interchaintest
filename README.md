<div align="center">
<h1>interchaintest</h1>

Formerly known as `ibctest`.

[![Go Reference](https://pkg.go.dev/badge/github.com/strangelove-ventures/interchaintest@main.svg)](https://pkg.go.dev/github.com/strangelove-ventures/interchaintest@main)
[![License: Apache-2.0](https://img.shields.io/github/license/strangelove-ventures/interchaintest.svg?style=flat-square)](https://github.com/strangelove-ventures/interchaintest/blob/main/create-test-readme/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/strangelove-ventures/interchaintest)](https://goreportcard.com/report/github.com/strangelove-ventures/interchaintest)



`interchaintest` orchestrates Go tests that utilize Docker containers for multiple
[IBC](https://docs.cosmos.network/master/ibc/overview.html)-compatible blockchains.

It allows users to quickly spin up custom testnets and dev environments to test IBC, chain infrastructures, smart contracts, etc.
</div>

### -- Features --

- Built-in suite of conformance tests to test high-level IBC compatibility between chain sets
- Easily construct customized tests in highly configurable environments
- Deployable as CI tests in production workflows

<br>

### Maintained Branches

|                                **Branch Name**                               | **IBC-Go** | **Cosmos-sdk** |
|:----------------------------------------------------------------------------:|:----------:|:--------------:|
|                                     main                                     |     v7     |      v0.47     |
|     [v6](https://github.com/strangelove-ventures/interchaintest/tree/v6)     |     v6     |      v0.46     |
|     [v5](https://github.com/strangelove-ventures/interchaintest/tree/v5)     |     v5     |      v0.46     |
|     [v4](https://github.com/strangelove-ventures/interchaintest/tree/v4)     |     v4     |      v0.45     |
|     [v3](https://github.com/strangelove-ventures/interchaintest/tree/v3)     |     v3     |      v0.45     |
| [v3-ics](https://github.com/strangelove-ventures/interchaintest/tree/v3-ics) |     v3     |  v0.45.11-ics  |

## Table Of Contents
- [Building Binary](#building-binary)
- **Usage:**
    - [Running Conformance Tests](./docs/conformanceTests.md) - Suite of built-in tests that test high-level IBC compatibility
    - [Write Custom Tests](./docs/writeCustomTests.md)
- [Retaining Data on Failed Tests](./docs/retainingDataOnFailedTests.md)
- [Deploy as GitHub CI Tests](./docs/ciTests.md)


<br>


## Building Binary

While it is not necessary to build the binary, sometimes it can be more convenient, *specifically* when running conformance test with custom chain sets. 

Building binary:
```shell
git clone https://github.com/strangelove-ventures/interchaintest.git
cd interchaintest
make interchaintest
```

This places the binary in `interchaintest/.bin/interchaintest`

Note that this is not in your Go path.


## Contributing

Contributing is encouraged.

Please read the [logging style guide](./docs/logging.md).

## Trophies

Significant bugs that were more easily fixed with `interchaintest`:

- [Juno network halt reproduction](https://github.com/strangelove-ventures/interchaintest/pull/7)
- [Juno network halt fix confirmation](https://github.com/strangelove-ventures/interchaintest/pull/8)