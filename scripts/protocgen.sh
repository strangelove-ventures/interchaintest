#!/usr/bin/env bash

set -eox pipefail

buf generate --template proto/buf.gen.penumbra.yaml buf.build/penumbra-zone/penumbra

# move proto files to the right places
# Note: Proto files are suffixed with the current binary version.
cp -r github.com/strangelove-ventures/interchaintest/v*/* ./
rm -rf github.com