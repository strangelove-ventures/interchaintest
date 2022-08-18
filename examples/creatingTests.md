# Up and Running: Creating a Test

This document is meant to provide a high level overview on how to architect tests.


Here is a basic example to get up and running. 

We'll then break code snippets and go into more detail about extra options below.

```go
func TestTemplate(t *testing.T) {
	ctx := context.Background()

	cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
        {Name: "gaia", ChainName: "gaia", Version: "v7.0.3"},
        {Name: "osmosis", ChainName: "osmo", Version: "v11.0.1"},
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
    {Name: "gaia", ChainName: "gaia", Version: "v7.0.3"},
    {Name: "osmosis", ChainName: "osmo", Version: "v11.0.1"},
})
```

The chain factory is where you configure your chain binaries. 

`ibctest` needs a docker package with the binary installed in order spin up nodes. 


Its integration with [Heighliner](https://github.com/strangelove-ventures/heighliner) (repository of docker images of many IBC enabled chains) makes this easy by simply passing in a `Name` and `Version` into the `ChainFactory`. 


However, you can also pass in remote images AND local images. 

See an example of each below:

```go
cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
    // HEIGHLINER EXAMPLE -- gaia chain -- Note that we override the default number of validators (default: 2) and full nodes (default: 1) for the gaia chain.
    {Name: "gaia", ChainName: "gaia", Version: "v7.0.2", NumValidators: &gaiaValidators, NumFullNodes: &gaiaFullnodes},

    // REMOTE REPO EXAMPLE -- ibc-go simd chain
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
        Denom: "gos",
        GasPrices: "0.00gos",
        GasAdjustment: 1.3,
        TrustingPeriod: "508h",
        NoHostMount: false},
    },

    // LOCAL DOCKER IMAGE EXAMPLE
    {ChainConfig: ibc.ChainConfig{
        Type: "cosmos",
        Name: "stringChain",
        ChainID: "string-1",
        Images: []ibc.DockerImage{
            {
                Repository: "string", // local docker image name
                Version: "v1.0.0",	// docker tag 
            },
        },
        Bin: "stringd",
        Bech32Prefix: "cosmos",
        Denom: "cheese",
        GasPrices: "0.01cheese",
        GasAdjustment: 1.3,
        TrustingPeriod: "508h",
        NoHostMount: false},
    },
    })
```

By default, `ibctest` will spin up a 3 docker images for each binary; 2 validators, 1 full node. These settings can all be configured inside the `ChainSpec`.
Example:

```go
gaiaValidators := int(4)
gaiaFullnodes := int(2)
cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
    {Name: "gaia", ChainName: "gaia", Version: "v7.0.2", NumValidators: &gaiaValidators, NumFullNodes: &gaiaFullnodes},
})
```


## Relayer Factory

The relayer factory is where relayer docker images are configured. 
Here we prep an image with the [Golang Relayer](https://github.com/cosmos/relayer)

```go
client, network := ibctest.DockerSetup(t)
r := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
    t, client, network)
```

## Interchain

This is where docker containers are built and all initial IBC transactions and links are made.

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

And initiate the build below. Note that the `Build` function takes a `testReporter`. This will instruct `ibctest` to create logs and reports. This functionality is discussed ##HERE. The `RelayerExecReporter` statisfies the reporter requirment. 

Note: If log files are not needed, you can use `testreporter.NewNopReporter()` instead.

```go
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









Once you configure your `ChainFactory`, You interact with each binary. 

For example, getting the RPC address:

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






## WIP RESEARCH: 
Would you use `cosmos.NewCosmosChain` instead oc `chainFactory` if you only need to initialize and start one chain?
https://github.com/strangelove-ventures/ibctest/blob/andrew/upgrade_test/examples/upgrade_test.go
```go
	ctx := context.Background()

	home := ibctest.TempDir(t)
	client, network := ibctest.DockerSetup(t)

	cfg, err := ibctest.BuiltinChainConfig(chainName)
	require.NoError(t, err)

	cfg.Images[0].Version = initialVersion
	cfg.ChainID = "chain-1"

	chain := cosmos.NewCosmosChain(t.Name(), cfg, 4, 1, zaptest.NewLogger(t))

	err = chain.Initialize(t.Name(), home, client, network, ibc.HaltHeight(haltHeight))
	require.NoError(t, err, "error initializing chain")

	err = chain.Start(t.Name(), ctx)
	require.NoError(t, err, "error starting chain")
```


## Transaction Example

## Reports


## How to run