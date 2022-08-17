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

The chain factory is where you configure your chain binaries. By default, `ibctest` will spin up a 3 docker images for each binary; 2 validators, 1 full node. These settings can all be configured as you'll see below.

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

You can now interact with each binary. For example, getting the RPC address:

```go
chains, err := cf.Chains(t.Name())
require.NoError(t, err)
gaia, osmosis := chains[0], chains[1]

gaiaRPC := gaia.GetGRPCAddress()
osmosisRPC := osmosis.GetGRPCAddress()
```

Here we create new funded wallets(users) for both chains. Note that there is also the option to restore a wallet (`ibctest.GetAndFundTestUserWithMnemonic`)
```go
gaiaUser, osmoUser := ibctest.GetAndFundTestUsers(t, ctx, "key1", 10_000_000, gaia, osmosis)
```

## Relayer Factory

The relayer factory is where docker images with relayers are created. Here we create an image with the [Golang Relayer](https://github.com/cosmos/relayer)

```go
client, network := ibctest.DockerSetup(t)
r := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
    t, client, network)
```

## Interchain

This is where all initial IBC transactions are made and where chains are linked.

We prep the "interchain" here:

```go
const ibcPath = "gaia-osmosis-demo"

ic := ibctest.NewInterchain().
    AddChain(gaia).
    AddChain(osmosis).
    AddRelayer(r1, "relayer").
    AddLink(ibctest.InterchainLink{
        Chain1:  gaia,
        Chain2:  osmosis,
        Relayer: r1,
        Path:    ibcPath,
    })
```

And initiate the build below. Note that the `Build` function takes a `testReporter`. This will instruct `ibctest` to create logs and reports. This functionality is discussed ##HERE.

If log files are not needed, you can use `testreporter.NewNopReporter()` instead.


```go
wd, err := os.Getwd()
require.NoError(t, err)
f, err := os.Create(filepath.Join(wd, "ibcTest_logs", fmt.Sprintf("%d.json", time.Now().Unix())))
require.NoError(t, err)

rep := testreporter.NewReporter(f)
eRep := rep.RelayerExecReporter(t)
require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
    TestName:  t.Name(),
    Client:    client,
    NetworkID: network,

    SkipPathCreation: false,
}))
```

Upon calling build, a faucet account with 10b units of denom is created. This wallet then funds 

Note the `SkipPathCreation` boolean. You can set this to true if you would like to manually call the relayer to create the `client`, `connection` and `channel`.

For example here we can manually create channel using the `ics27-1` standard like so:
```go
r1.CreateChannel(ctx, eRep, ibcPath, ibc.CreateChannelOptions{
    SourcePortName: "transfer",
    DestPortName: "transfer",
    Order: "Ordered",
    
    Version: "ics27-1",
})
```



## Transaction Example

## Reports


## How to run