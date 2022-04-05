package ibc

func (ibc IBCTestCase) CW20IBCTest(testName string, srcChain Chain, dstChain Chain, relayerImplementation RelayerImplementation) error {
	ctx, home, pool, network, cleanup, err := SetupTestRun(testName)
	if err != nil {
		return err
	}
	defer cleanup()

	channels, user, rlyCleanup, err := StartChainsAndRelayer(testName, ctx, pool, network, home, srcChain, dstChain, relayerImplementation, nil)
	if err != nil {
		return err
	}
	defer rlyCleanup()
}

func (ibc IBCTestCase) IBCReflectTest(testName string, srcChain Chain, dstChain Chain, relayerImplementation RelayerImplementation) error {
	ctx, home, pool, network, cleanup, err := SetupTestRun(testName)
	if err != nil {
		return err
	}
	defer cleanup()

	channels, user, rlyCleanup, err := StartChainsAndRelayer(testName, ctx, pool, network, home, srcChain, dstChain, relayerImplementation, nil)
	if err != nil {
		return err
	}
	defer rlyCleanup()

	srcChain.InstantiateContract(ctx, userAccountKeyName, WalletAmount{}, path.join())
}
