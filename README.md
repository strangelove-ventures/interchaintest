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

## Table Of Contents
- [Branch Versioning](#maintained-branches)
- **Use Cases:**
    - **Importing as a Module**
        - [Chain Integration and GitHub CI](./docs/ciTests.md)
    -  **Running as a Binary**
        - [Building Binary](./docs/buildBinary.md)
        - [Running Conformance Tests](./docs/conformanceTests.md) - Suite of built-in tests that test high-level IBC compatibility
- [Write Custom Tests](./docs/writeCustomTests.md)
- [Environment Variable Options](./docs/envOptions.md)
- [Retaining Data on Failed Tests](./docs/retainingDataOnFailedTests.md)


### Maintained Branches

|                                **Branch Name**                               | **IBC-Go** | **Cosmos-sdk** |
|:----------------------------------------------------------------------------:|:----------:|:--------------:|
|     [main](https://github.com/strangelove-ventures/interchaintest)           |     v8     |      v0.50     |
|     [v7](https://github.com/strangelove-ventures/interchaintest/tree/v7)     |     v7     |      v0.47     |

### Depreciated Branches

These are branches that we no longer actively update or maintain but may be of use if a chain is running older versions of the `Cosmos SDK ` or `IBC Go`. Please see the [Backport Policy](#backport-policy) below.


|                                **Branch Name**                               | **IBC-Go** | **Cosmos-sdk** | **Depreciated Date** |
|:----------------------------------------------------------------------------:|:----------:|:--------------:|:--------------------:|
|     [v6](https://github.com/strangelove-ventures/interchaintest/tree/v6)     |     v6     |      v0.46     |       Sept 5 2023    |
|     [v5](https://github.com/strangelove-ventures/interchaintest/tree/v5)     |     v5     |      v0.46     |       Aug 11 2023    |
|     [v4](https://github.com/strangelove-ventures/interchaintest/tree/v4)     |     v4     |      v0.45     |       Aug 11 2023    |
| [v4-ics](https://github.com/strangelove-ventures/interchaintest/tree/v4-ics) |     v4     |   v0.45.x-ics  |       Aug 11 2023    |
|     [v3](https://github.com/strangelove-ventures/interchaintest/tree/v3)     |     v3     |      v0.45     |      June 25 2023    |
| [v3-ics](https://github.com/strangelove-ventures/interchaintest/tree/v3-ics) |     v3     |  v0.45.11-ics  |      April 24 2023   |


#### Backport Policy:
Strangelove maintains `n` and `n - 1` branches of interchaintest, `n` being current `main`.

We strive to keep interchaintest inline with the latest from the ibc-go and cosmos sdk teams. Once an alpha versions of the next major ibc-go version is released, we will discontinue `n - 1` and branch off a new `n`.

**Recommendation:** Even if your chain uses an older version of ibc-go, try importing from `main`. This should work unless you are decoding transactions that require a specific ibc-go version.

If there is a feature you would like backported to an older branch, make an issue! We are happy to work with you. 


## Contributing

Contributing is encouraged.

Please read the [logging style guide](./docs/logging.md).

## Trophies

Significant bugs that were more easily fixed with `interchaintest`:

- [Juno network halt reproduction](https://github.com/strangelove-ventures/interchaintest/pull/7)
- [Juno network halt fix confirmation](https://github.com/strangelove-ventures/interchaintest/pull/8)
