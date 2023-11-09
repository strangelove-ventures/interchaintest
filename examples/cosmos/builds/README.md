# Chain Build

## Ethermint
- Repo: <https://github.com/dymensionxyz/dymension>
- Commit: 854ef846a79d26853f32a8e50f597592c24e3d99
- docker build . -t dymension:local
- docker save dymension:local > examples/cosmos/chain_builds/dymension.tar
- [xz](https://linux.die.net/man/1/xz) examples/cosmos/chain_builds/dymension.tar