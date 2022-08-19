# Up and Running: Architecting Tests

[learn_ibc_test.go](./examples/learn_ibc_test.go) is a basic test that:
1) Spins up two chains (Gaia and Osmosis) 
2) Creates an IBC Path between them (client, connection, channel)
3) Sends an IBC transaction between them.

It confirms that each step was successful. 

We'll use this test to break down code snippets and go into more detail about extra options below.


## Chain Factory

```go
cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
    {Name: "gaia", ChainName: "gaia", Version: "v7.0.3"},
    {Name: "osmosis", ChainName: "osmo", Version: "v11.0.1"},
})
```

The chain factory is where you configure your chain binaries. 

`ibctest` needs a docker image with the chain binary(s) of choice installed to spin up nodes. 


Its integration with [Heighliner](https://github.com/strangelove-ventures/heighliner) (repository of docker images of many IBC enabled chains) makes this easy by simply passing in a `Name` and `Version` into the `ChainFactory`. 

You can also pass in remote images AND local docker images. 

See an example of each below:

```go
cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
    // HEIGHLINER EXAMPLE -- gaia chain
    {Name: "gaia", ChainName: "gaia", Version: "v7.0.2"},

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

By default, `ibctest` will spin up a 3 docker images for each binary; 2 validator nodes, 1 full node. These settings can all be configured inside the `ChainSpec`.
EXAMPLE: specifying validators and full nodes:

```go
gaiaValidators := int(4)
gaiaFullnodes := int(2)
cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
    {Name: "gaia", ChainName: "gaia", Version: "v7.0.2", NumValidators: &gaiaValidators, NumFullNodes: &gaiaFullnodes},
})
```


## Relayer Factory

The relayer factory is where relayer docker images are configured. 
Here we prep an image with the [Golang Relayer](https://github.com/cosmos/relayer)(CosmosRly)

```go
client, network := ibctest.DockerSetup(t)
r := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
    t, client, network)
```

## Interchain

This is where we configure our test-net/interchain. 

We prep the "interchain" here by adding chains, a relayer, and specifying which chains to create IBC paths for:

```go
const ibcPath = "gaia-osmosis-demo"
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
```

The `Build` function below spins everything up. Note that this function takes a `testReporter`. This will instruct `ibctest` to create logs and reports. This functionality is discussed ##HERE. The `RelayerExecReporter` satisfies the reporter requirement. 

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

Upon calling build, several things happen (specifically for cosmos based chains):
- genesis for each chain takes place
- each validator gets 2 trillion units of "stake"
    - 1 trillion "stake" are staked
    - 100 billion "stake" are self delegated
- each chain gets a faucet address (key named "faucet") with 10 billion units of denom
- the realyer wallet gets 1 billion units of each chains denom #HELP! Does the faucet fund relayer wallets? Is the faucet wallet reachable via the API?
- IBC paths are created: `client`, `connection`, `channel` for each link
    
    
Unless specified, default options are used for `client`, `connection`, and `channel` creation. 

Default `channel options` are:
```yaml
    SourcePortName: "transfer",
    DestPortName:   "transfer",
    Order:          Unordered,
    Version:        "ics20-1",
```

EXAMPLE: passing in channel options to support the `ics27-1` interchain accounts standard:
```go
require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
		TestName:  t.Name(),
		Client:    client,
		NetworkID: network,

		CreateChannelOpts: ibc.CreateChannelOptions{
			SourcePortName: "transfer",
			DestPortName:   "transfer",
			Order:          ibc.Ordered,
			Version:        "ics27-1",
		},

		SkipPathCreation: false},
	),
	)
```

Note the `SkipPathCreation` boolean. You can set this to `true` if you would like to manually call the relayer to create the `client`, `connection` and `channel`.
EXAMPLE: creating client manually with relayer: 

```go
r.CreateClients(ctx, eRep, ibcPath)
```

Now that the interchain is built, you interact with each binary. 

EXAMPLE: Getting the RPC address:
```go
chains, err := cf.Chains(t.Name())
require.NoError(t, err)
gaia, osmosis := chains[0], chains[1]

gaiaRPC := gaia.GetGRPCAddress()
osmosisRPC := osmosis.GetGRPCAddress()
```


Here we create new funded wallets(users) for both chains. These wallets are funded from the faucet key.
Note that there is also the option to restore a wallet (`ibctest.GetAndFundTestUserWithMnemonic`)
```go
fundAmount := 1_000
users := ibctest.GetAndFundTestUsers(t, ctx, "default", int64(fundAmount), gaia, osmosis)
gaiaUser := users[0]
osmosisUser := users[1]
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