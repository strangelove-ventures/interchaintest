<div align="center">
<h1><code>interchaintest</code></h1>

Formerly known as `ibctest`.

[![Go Reference](https://pkg.go.dev/badge/github.com/strangelove-ventures/interchaintest@main.svg)](https://pkg.go.dev/github.com/strangelove-ventures/interchaintest@main)
[![License: Apache-2.0](https://img.shields.io/github/license/strangelove-ventures/interchaintest.svg?style=flat-square)](https://github.com/strangelove-ventures/interchaintest/blob/main/create-test-readme/LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/strangelove-ventures/interchaintest)](https://goreportcard.com/report/github.com/strangelove-ventures/interchaintest)
[![Conforms to README.lint](https://img.shields.io/badge/README.lint-conforming-brightgreen)](https://github.com/strangelove-ventures/readme-dot-lint)
</div>



🌌 Why use `interchaintest`?
=============================

In order to ship production-grade software for the Interchain, we needed sophisticated developer tooling...but IBC and Web3 have a *lot* of moving parts, which can lead to a steep learning curve and all sorts of pain. Recognize any of these?

- repeatedly building repo-specific, Docker- and shell-based testing solutions,
- duplication of effort, and
- difficulty in repurposing existing testing harnesses for new problem domains.

We built `interchaintest` to extract patterns and create a generic test harness: a use-case-agnostic framework for generating repeatable, diagnostic tests for every aspect of IBC.

Read more at the [Announcing `interchaintest` blog post](https://strange.love/blog/announcing-interchaintest).

🌌🌌 Who benefits from `interchaintest`?
=============================

`interchaintest` is for developers who expect top-shelf testing tools when working on blockchain protocols such as Cosmos or Ethereum.


🌌🌌🌌 What does `interchaintest` do?
=============================

`interchaintest` is a framework for testing blockchain functionality and interoperability between chains, primarily with the Inter-Blockchain Communication (IBC) protocol.

Want to quickly spin up custom testnets and dev environments to test IBC, [Relayer](https://github.com/cosmos/relayer) setup, chain infrastructure, smart contracts, etc.? `interchaintest` orchestrates Go tests that utilize Docker containers for multiple [IBC](https://www.ibcprotocol.dev/)-compatible blockchains.



🌌🌌🌌🌌 How do I use it?
=============================

## As a Module

Most people choose to import `interchaintest` as a module.
- Often, teams will [integrate `interchaintest` with a github CI/CD pipeline](./docs/ciTests.md).
- Most teams will write their own suite. Here's a tutorial on [Writing Custom Tests](./docs/writeCustomTests.md).
- You can also [utilize our suite of built-in Conformance Tests that exercise high-level IBC compatibility](./docs/conformance-tests-lib.md).

## As a Binary

There's also an option to [build and run `interchaintest` as a binary](./docs/buildBinary.md) (which might be preferable, e.g., with custom chain sets). You can still [run Conformance Tests](./docs/conformance-tests-bin.md).


## References
- [Environment Variable Options](./docs/envOptions.md)
- [Retaining Data on Failed Tests](./docs/retainingDataOnFailedTests.md)


🌌🌌🌌🌌🌌 Extras
=============================



### Maintained Branches

|                                **Branch Name**                               | **IBC-Go** | **Cosmos-sdk** |
|:----------------------------------------------------------------------------:|:----------:|:--------------:|
|     [main](https://github.com/strangelove-ventures/interchaintest)           |     v8     |      v0.50     |
|     [v7](https://github.com/strangelove-ventures/interchaintest/tree/v7)     |     v7     |      v0.47     |

### Deprecated Branches

These are branches that we no longer actively update or maintain but may be of use if a chain is running older versions of the `Cosmos SDK ` or `IBC Go`. Please see the [Backport Policy](#backport-policy) below.


|                               **Branch Name**                                | **IBC-Go** | **Cosmos-sdk** | **Deprecated Date** |
|:----------------------------------------------------------------------------:|:----------:|:--------------:|:--------------------:|
|     [v6](https://github.com/strangelove-ventures/interchaintest/tree/v6)     |     v6     |     v0.46      |     Sept 5 2023      |
|     [v5](https://github.com/strangelove-ventures/interchaintest/tree/v5)     |     v5     |     v0.46      |     Aug 11 2023      |
|     [v4](https://github.com/strangelove-ventures/interchaintest/tree/v4)     |     v4     |     v0.45      |     Aug 11 2023      |
| [v4-ics](https://github.com/strangelove-ventures/interchaintest/tree/v4-ics) |     v4     |  v0.45.x-ics   |     Aug 11 2023      |
|     [v3](https://github.com/strangelove-ventures/interchaintest/tree/v3)     |     v3     |     v0.45      |     June 25 2023     |
| [v3-ics](https://github.com/strangelove-ventures/interchaintest/tree/v3-ics) |     v3     |  v0.45.11-ics  |    April 24 2023     |


#### Backport Policy:
Strangelove maintains `n` and `n - 1` branches of `interchaintest`, `n` being current `main`.

We strive to keep `interchaintest` inline with the latest from the ibc-go and cosmos sdk teams. Once an alpha versions of the next major ibc-go version is released, we will discontinue `n - 1` and branch off a new `n`.

**Recommendation:** Even if your chain uses an older version of ibc-go, try importing from `main`. This should work unless you are decoding transactions that require a specific ibc-go version.

If there is a feature you would like backported to an older branch, make an issue! We are happy to work with you.


## Contributing

Contributing is encouraged.

Please read the [logging style guide](./docs/logging.md).

## Trophies

Significant bugs that were more easily fixed with `interchaintest`:

- [Juno network halt reproduction](https://github.com/strangelove-ventures/interchaintest/pull/7)
- [Juno network halt fix confirmation](https://github.com/strangelove-ventures/interchaintest/pull/8)
