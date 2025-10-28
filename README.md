<div align="center">
<h1><code>interchaintest</code></h1>

Formerly known as `ibctest`, `interchaintest` was built and initially maintained by [Strangelove Ventures](https://strange.love/).

This fork is maintained by the Interchain Operations team at [Hypha Worker Co-operative](https://hypha.coop/).

[![License: Apache-2.0](https://img.shields.io/github/license/strangelove-ventures/interchaintest.svg?style=flat-square)](https://github.com/cosmos/interchaintest/blob/main/LICENSE)
</div>

🌌 Why use `interchaintest`?
=============================

In order to ship production-grade software for the Interchain, we needed sophisticated developer tooling...but IBC and Web3 have a *lot* of moving parts, which can lead to a steep learning curve and all sorts of pain. Recognize any of these?

- repeatedly building repo-specific, Docker- and shell-based testing solutions,
- duplication of effort, and
- difficulty in repurposing existing testing harnesses for new problem domains.

We built `interchaintest` to extract patterns and create a generic test harness: a use-case-agnostic framework for generating repeatable, diagnostic tests for every aspect of IBC.

Read more at the [Announcing `interchaintest` blog post](https://strange.love/blog/announcing-interchaintest).

🌌 Who benefits from `interchaintest`?
=============================

`interchaintest` is for developers who expect top-shelf testing tools when working on blockchain protocols such as Cosmos or Ethereum.


🌌 What does `interchaintest` do?
=============================

`interchaintest` is a framework for testing blockchain functionality and interoperability between chains, primarily with the Inter-Blockchain Communication (IBC) protocol.

Want to quickly spin up custom testnets and dev environments to test IBC, [Relayer](https://github.com/cosmos/relayer) setup, chain infrastructure, smart contracts, etc.? `interchaintest` orchestrates Go tests that utilize Docker containers for multiple [IBC](https://www.ibcprotocol.dev/)-compatible blockchains.

🌌 How do I use it?
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

🌌 Extras
=============================

## Version Tagging

Tags are assigned to commits that target a specific chain version. For instance, the `v25.1.1-gaia` tag targets Gaia `v25.1.1`.
This allows interchaintest to support specific combinations of the Cosmos stack that are used in production.

|                                    Tag                                     | Cosmos SDK | CometBFT | IBC-Go  | wasmd | Hermes  |
| :------------------------------------------------------------------------: | :--------: | :------: | :-----: | :---: | :-----: |
| [v25.1.1-gaia](https://github.com/cosmos/interchaintest/tree/v25.0.0-gaia) |   v0.53    |  v38.17  | v10.3.0 | v0.60 | v1.13.2 |
|      [v10.0.0](https://github.com/cosmos/interchaintest/tree/v10.0.0)      |   v0.53    |  v38.17  | v10.3.0 | v0.60 | v1.8.2  |


## Contributing

Contributing is encouraged.

Please read the [logging style guide](./docs/logging.md).

## Trophies

Significant bugs that were more easily fixed with `interchaintest`:

- [Juno network halt reproduction](https://github.com/strangelove-ventures/interchaintest/pull/7)
- [Juno network halt fix confirmation](https://github.com/strangelove-ventures/interchaintest/pull/8)
