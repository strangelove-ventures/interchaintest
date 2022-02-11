# IBC Test Framework

This repo is going to house a new IBC testing framework based on the following work:
- https://github.com/PeggyJV/sommelier/tree/main/integration_tests
- https://github.com/strangelove-ventures/horcrux/tree/main/test
- https://github.com/cosmos/relayer/tree/main/test

The goals are to support:
- [ ] Testing complex IBC interactions between arbitrary chains
- [ ] Testing multiple relayer implemenations
    - [ ] cosmos/relayer
    - [ ] hermes
    - [ ] tsrelayer
- [ ] Testing multiple versions of each chain and compatability of new versions

The tests will be run in `go test` and utilize docker to spin up complete chains and utilize only the chain docker images themseleves.

This repo will rely on images built from https://github.com/strangelove-ventures/heighliner.

## Note

If you do not have the containers from [heighliner](https://github.com/strangelove-ventures/heighliner)
built already, you will need to pull them down (e.g. `docker pull ghcr.io/strangelove-ventures/heighliner/gaia:v6.0.0-rocks`).
Future work will handle this for you.
