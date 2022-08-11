# Up and Running: Creating a Test

This document is meant to provide a high level overview on how to architect tests.


Here is a basic example to get up and running. 

We'll then break code snippets and go into more detail about extra options below.

```go
func TestTemplate(t *testing.T) {
	ctx := context.Background()

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
		{Name: "gaia", ChainName: "gaia-1", Version: "v7.0.2"},
		{Name: "osmosis", ChainName: "gaia-2", Version: "v7.0.2"},
	})

	client, network := ibctest.DockerSetup(t)
	r := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
		t, client, network)

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)
	gaia, osmosis := chains[0], chains[1]

    // CREATE USERS

	const ibcPath = "gaia-osmo-demo"

	ic := ibctest.NewInterchain().
		AddChain(gaia).
		AddChain(osmosis).
		AddRelayer(r, "relayer").
		AddLink(ibctest.InterchainLink{
			Chain1:  gaia,
			Chain2:  osmosis,
			Relayer: r,
			Path:    ibcPath,
		})

	rep := testreporter.NewNopReporter()
	eRep := rep.RelayerExecReporter(t)
	require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		HomeDir:   ibctest.TempDir(t),
		Client:    client,
		NetworkID: network,

		SkipPathCreation: false}
        )
    )

    // START RELAYER

    // SEND IBC TX

    // CONFIRM

}
```

## Chain Factory

```go
cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
    {Name: "gaia", ChainName: "gaia-1", Version: "v7.0.2"},
    {Name: "osmosis", ChainName: "gaia-2", Version: "v7.0.2"},
})
```

The chain factory is where you configure your chain binaries. `ibctest` will spin up a docker image for each binary.

Unless `Images` are passed into the `ChainSpec`, `ibctest` will attempt to build images from [Heighliner's](https://github.com/strangelove-ventures/heighliner) packages.

Here is an example of building one gaia chain from Heighliner and another chain from ibc-go's simd images. Note that we override the default number of validators (default: 2) and full nodes (default: 1) for the gaia chain.

```go
gaiaValidators := int(4)
gaiaFullnodes := int(2)
cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
    // gaia chain
    {Name: "gaia", ChainName: "gaia", Version: "v7.0.2", NumValidators: &gaiaValidators, NumFullNodes: &gaiaFullnodes},

    // ibc-go simd chain
    {ChainConfig: ibc.ChainConfig{
        Type: "cosmos",
        Name: "ibc-go-simd",
        ChainID: "simd-1",
        Images: []ibc.DockerImage{
            {
                Repository: "ghcr.io/cosmos/ibc-go-simd-e2e",
                Version: "pr-1973",
            },
        },
        Bin: "simd",
        Bech32Prefix: "cosmos",
        Denom: "steak",
        GasPrices: "0.00steak",
        GasAdjustment: 1.3,
        TrustingPeriod: "508h",
        NoHostMount: false}
    },
    })
```

You can now interact with each docker image. For example, getting the RPC address:

```go
chains, err := cf.Chains(t.Name())
require.NoError(t, err)
gaia, osmosis := chains[0], chains[1]

gaiaRPC := gaia.GetGRPCAddress()
osmosisRPC := osmosis.GetGRPCAddress()
```

Here we create new funded wallets for each chain. Note that also have the option to restore a wallet with --FUNCTION NAME--
```go
# code
```

## Relayer Factory

## Transaction Example


## How to run