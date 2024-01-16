#!/usr/bin/env bash

set -eox pipefail

buf generate --template proto/buf.gen.penumbra.yaml buf.build/penumbra-zone/penumbra

# move proto files to the right places
# Note: Proto files are suffixed with the current binary version.
# rm -r github.com/strangelove-ventures/interchaintest/v*/chain/penumbra/client
#rm -r github.com/strangelove-ventures/interchaintest/v*/chain/penumbra/narsil
cp -r github.com/strangelove-ventures/interchaintest/v*/* ./
rm -rf github.com
