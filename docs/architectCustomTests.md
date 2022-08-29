# Up and Running: Architecting Tests

This doc breaks down code snippets found in [learn_ibc_test.go](./examples/learn_ibc_test.go).

This is a basic test that:

1) Spins up two chains (Gaia and Osmosis) 
2) Creates an IBC Path between them (client, connection, channel)
3) Sends an IBC transaction between them.

It validates each step and confirms that the balances of each wallet are correct. 


## Chain Factory

```go
cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
    {Name: "gaia", Version: "v7.0.3", ChainConfig: ibc.ChainConfig{
        GasPrices: "0.0uatom",
    }},
    {Name: "osmosis", Version: "v11.0.1"},
})
```

The chain factory is where you configure your chain binaries. 

`ibctest` needs a docker image with the chain binary(s) installed to spin up nodes. 

To spin up tests quickly, IBCTest has several [pre-configured chains](./docs/preconfiguredChains.txt). These docker images are pulled from [Heighliner](https://github.com/strangelove-ventures/heighliner) (repository of docker images of many IBC enabled chains). Note that heighliner needs the `Version` you are requesting.

If the `Name` matches the name of a pre-configured chain, the pre-configured settings are used. You can override these settings by passing them intot the `ibc.ChainConfig` when initalizing your ChainFactory. We do this above with the `GasPrices` for gaia.

You can also pass in **remote images** AND/OR **local docker images**. 

See an example of each below:

```go
cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
    // PRE CONFIGURED CHAIN EXAMPLE -- gaia chain
    {Name: "gaia", Version: "v7.0.2"},

    // REMOTE IMAGE EXAMPLE -- ibc-go simd chain
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

By default, `ibctest` will spin up a 3 docker images for each chain; 2 validator nodes, 1 full node. These settings can all be configured inside the `ChainSpec`.
EXAMPLE: Overrideing defaults for number of validators and full nodes:

```go
gaiaValidators := int(4)
gaiaFullnodes := int(2)
cf := ibctest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*ibctest.ChainSpec{
    {Name: "gaia", ChainName: "gaia", Version: "v7.0.2", NumValidators: &gaiaValidators, NumFullNodes: &gaiaFullnodes},
})
```

Here we break out each chain in preparation to pass into `Interchain`:
```go
chains, err := cf.Chains(t.Name())
require.NoError(t, err)
gaia, osmosis := chains[0], chains[1]
```

## Relayer Factory

The relayer factory is where relayer docker images are configured. 

Currently only the [Cosmos/Relayer](https://github.com/cosmos/relayer)(CosmosRly) is integrated into IBCtest. 

Here we prep an image with the Cosmos/Relayer:
```go
client, network := ibctest.DockerSetup(t)
r := ibctest.NewBuiltinRelayerFactory(ibc.CosmosRly, zaptest.NewLogger(t)).Build(
    t, client, network)
```

## Interchain

This is where we configure our test-net/interchain. 

We prep the "interchain" by adding chains, a relayer, and specifying which chains to create IBC paths for:

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

The `Build` function below spins everything up.

```go
rep := testreporter.NewReporter(f) // f is the location to write the logs
eRep := rep.RelayerExecReporter(t)
require.NoError(t, ic.Build(ctx, eRep, ibctest.InterchainBuildOptions{
    TestName:  t.Name(),
    Client:    client,
    NetworkID: network,

    SkipPathCreation: false,
}))
```

Upon calling build, several things happen (specifically for cosmos based chains):

- each validator gets 2 trillion units of "stake" funded in genesis
    - 1 trillion "stake" are staked
    - 100 billion "stake" are self delegated
- each chain gets a faucet address (key named "faucet") with 10 billion units of denom funded in gensis
- the realyer wallet gets 1 billion units of each chains denom funded in genesis 
- genesis for each chain takes place
- IBC paths are created: `client`, `connection`, `channel` for each link


 Note that this function takes a `testReporter`. This will instruct `ibctest` to create logs and reports. The `RelayerExecReporter` satisfies the reporter requirement. 

Note: If log files are not needed, you can use `testreporter.NewNopReporter()` instead.
    
    
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
r.CreateClients(ctx, eRep, "my-path")
```

## Creating Users(wallets)

Here we create new funded wallets(users) for both chains. These wallets are funded from the "faucet" key.
Note that there is also the option to restore a wallet (`ibctest.GetAndFundTestUserWithMnemonic`)

```go
fundAmount := int64(10_000_000)
users := ibctest.GetAndFundTestUsers(t, ctx, "default", int64(fundAmount), gaia, osmosis)
gaiaUser := users[0]
osmosisUser := users[1]
```

## Interacting with the Interchain

Now that the interchain is built, you can interact with each binary. 

EXAMPLE: Getting the RPC address:
```go
gaiaRPC := gaia.GetGRPCAddress()
osmosisRPC := osmosis.GetGRPCAddress()
```

The rest of code in `learn_ibc_test.go` is failry self explanitory. 


## How to run

Running tests leverages Go Tests.

For more details run:
`go help test`

In general your test needs to be in a file ending in "_test.go". The function name must start with "Test"

To run:
`go test -timeout 10m -v -run <NAME_OF_TEST> <PATH/TO/FOLDER/HOUSING/TESTS/FILES `