package avs_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/strangelove-ventures/interchaintest/v8"
	"github.com/strangelove-ventures/interchaintest/v8/chain/ethereum"

	"github.com/strangelove-ventures/interchaintest/v8/testreporter"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/Layr-Labs/eigensdk-go/chainio/clients/eth"
	//clientconstructor "github.com/Layr-Labs/eigensdk-go/chainio/constructor"
	sdklogging "github.com/Layr-Labs/eigensdk-go/logging"
	"github.com/Layr-Labs/eigensdk-go/signer"
	sdkutils "github.com/Layr-Labs/eigensdk-go/utils"
	"github.com/Layr-Labs/incredible-squaring-avs/aggregator"
	"github.com/Layr-Labs/incredible-squaring-avs/core/chainio"
	"github.com/Layr-Labs/incredible-squaring-avs/core/config"
	"github.com/Layr-Labs/incredible-squaring-avs/operator"
	"github.com/Layr-Labs/incredible-squaring-avs/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestIntegration(t *testing.T) {

	if testing.Short() {
		t.Skip()
	}

	t.Parallel()

	client, network := interchaintest.DockerSetup(t)

	// Log location
	f, err := interchaintest.CreateLogFile(fmt.Sprintf("%d.json", time.Now().Unix()))
	require.NoError(t, err)
	// Reporter/logs
	rep := testreporter.NewReporter(f)
	eRep := rep.RelayerExecReporter(t)

	ctx := context.Background()

	// Get default ethereum chain config for anvil
	anvilConfig := ethereum.DefaultEthereumAnvilChainConfig("ethereum")

	// add --load-state config
	configFileOverrides := make(map[string]any)
	configFileOverrides["--load-state"] = "avs-and-eigenlayer-deployed-anvil-state.json" // Relative path of state.json
	anvilConfig.ConfigFileOverrides = configFileOverrides

	cf := interchaintest.NewBuiltinChainFactory(zaptest.NewLogger(t), []*interchaintest.ChainSpec{
		{
			ChainName: "ethereum", 
			Name: "ethereum",
			Version: "latest",
			ChainConfig: anvilConfig,
		},
	})

	chains, err := cf.Chains(t.Name())
	require.NoError(t, err)

	ethereumChain := chains[0].(*ethereum.EthereumChain)

	ic := interchaintest.NewInterchain().
		AddChain(ethereumChain)

	require.NoError(t, ic.Build(ctx, eRep, interchaintest.InterchainBuildOptions{
		TestName:          t.Name(),
		Client:            client,
		NetworkID:         network,
		// BlockDatabaseFile: interchaintest.DefaultBlockDatabaseFilepath(),
		SkipPathCreation:  true, // Skip path creation, so we can have granular control over the process
	}))
	fmt.Println("Interchain built")

	// Start integration test from incredible-mesh-avs
	/* Prepare the config file for aggregator */
	var aggConfigRaw config.ConfigRaw
	aggConfigFilePath := "config-files/aggregator.yaml"
	sdkutils.ReadYamlConfig(aggConfigFilePath, &aggConfigRaw)
	aggConfigRaw.EthRpcUrl = ethereumChain.GetHostRPCAddress()
	aggConfigRaw.EthWsUrl = ethereumChain.GetHostWSAddress()

	var credibleSquaringDeploymentRaw config.CredibleSquaringDeploymentRaw
	credibleSquaringDeploymentFilePath := "contracts/script/output/31337/credible_squaring_avs_deployment_output.json"
	sdkutils.ReadJsonConfig(credibleSquaringDeploymentFilePath, &credibleSquaringDeploymentRaw)

	var sharedAvsContractsDeploymentRaw config.SharedAvsContractsRaw
	sharedAvsContractsDeploymentFilePath := "contracts/script/output/31337/shared_avs_contracts_deployment_output.json"
	sdkutils.ReadJsonConfig(sharedAvsContractsDeploymentFilePath, &sharedAvsContractsDeploymentRaw)

	logger, err := sdklogging.NewZapLogger(aggConfigRaw.Environment)
	if err != nil {
		t.Fatalf("Failed to create logger: %s", err.Error())
	}
	ethRpcClient, err := eth.NewClient(aggConfigRaw.EthRpcUrl)
	if err != nil {
		t.Fatalf("Failed to create eth client: %s", err.Error())
	}
	ethWsClient, err := eth.NewClient(aggConfigRaw.EthWsUrl)
	if err != nil {
		t.Fatalf("Failed to create eth client: %s", err.Error())
	}

	ecdsaPrivateKeyString := "0x2a871d0798f97d79848a013d4936a73bf4cc922c825d33c1cf7073dff6d409c6"
	if ecdsaPrivateKeyString[:2] == "0x" {
		ecdsaPrivateKeyString = ecdsaPrivateKeyString[2:]
	}
	ecdsaPrivateKey, err := crypto.HexToECDSA(ecdsaPrivateKeyString)
	if err != nil {
		t.Fatalf("Cannot parse ecdsa private key: %s", err.Error())
	}

	operatorAddr, err := sdkutils.EcdsaPrivateKeyToAddress(ecdsaPrivateKey)
	if err != nil {
		t.Fatalf("Cannot get operator address: %s", err.Error())
	}

	chainId, err := ethRpcClient.ChainID(context.Background())
	if err != nil {
		t.Fatalf("Cannot get chainId: %s", err.Error())
	}

	privateKeySigner, err := signer.NewPrivateKeySigner(ecdsaPrivateKey, chainId)
	if err != nil {
		t.Fatalf("Cannot create signer: %s", err.Error())
	}

	config := &config.Config{
		EcdsaPrivateKey:                      ecdsaPrivateKey,
		Logger:                               logger,
		EthRpcUrl:                            aggConfigRaw.EthRpcUrl,
		EthHttpClient:                        ethRpcClient,
		EthWsClient:                          ethWsClient,
		BlsOperatorStateRetrieverAddr:        common.HexToAddress(sharedAvsContractsDeploymentRaw.BlsOperatorStateRetrieverAddr),
		IncredibleSquaringServiceManagerAddr: common.HexToAddress(credibleSquaringDeploymentRaw.Addresses.IncredibleSquaringServiceManagerAddr),
		SlasherAddr:                          common.HexToAddress(""),
		AggregatorServerIpPortAddr:           aggConfigRaw.AggregatorServerIpPortAddr,
		RegisterOperatorOnStartup:            aggConfigRaw.RegisterOperatorOnStartup,
		Signer:                               privateKeySigner,
		OperatorAddress:                      operatorAddr,
		BlsPublicKeyCompendiumAddress:        common.HexToAddress(aggConfigRaw.BLSPubkeyCompendiumAddr),
	}

	/* Prepare the config file for operator */
	nodeConfig := types.NodeConfig{}
	nodeConfigFilePath := "config-files/operator.anvil.yaml"
	err = sdkutils.ReadYamlConfig(nodeConfigFilePath, &nodeConfig)
	if err != nil {
		t.Fatalf("Failed to read yaml config: %s", err.Error())
	}
	/* Register operator*/
	// log.Println("registering operator for integration tests")
	// we need to do this dynamically and can't just hardcode a registered operator into the anvil
	// state because the anvil state dump doesn't also dump the receipts tree so we lose events,
	// and the aggregator thus can't get the operator's pubkey
	// operatorRegistrationCmd := exec.Command("bash", "./operator-registration.sh")
	// err = operatorRegistrationCmd.Run()
	// if err != nil {
	// 	t.Fatalf("Failed to register operator: %s", err.Error())
	// }

	ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
	defer cancel()
	/* start operator */
	// the passwords are set to empty strings
	log.Println("starting operator for integration tests")
	os.Setenv("OPERATOR_BLS_KEY_PASSWORD", "")
	os.Setenv("OPERATOR_ECDSA_KEY_PASSWORD", "")
	nodeConfig.BlsPrivateKeyStorePath = "keys/test.bls.key.json"
	nodeConfig.EcdsaPrivateKeyStorePath = "keys/test.ecdsa.key.json"
	nodeConfig.RegisterOperatorOnStartup = true
	nodeConfig.EthRpcUrl = ethereumChain.GetHostRPCAddress()
	nodeConfig.EthWsUrl = ethereumChain.GetHostWSAddress()
	operator, err := operator.NewOperatorFromConfig(nodeConfig)
	if err != nil {
		t.Fatalf("Failed to create operator: %s", err.Error())
	}
	go operator.Start(ctx)
	log.Println("Started operator. Sleeping 15 seconds to give it time to register...")
	time.Sleep(15 * time.Second)

	/* start aggregator */
	log.Println("starting aggregator for integration tests")
	agg, err := aggregator.NewAggregator(config)
	if err != nil {
		t.Fatalf("Failed to create aggregator: %s", err.Error())
	}
	go agg.Start(ctx)
	log.Println("Started aggregator. Sleeping 20 seconds to give operator time to answer task 1...")
	time.Sleep(20 * time.Second)

	// get avsRegistry client to interact with the chain
	avsReader, err := chainio.NewAvsReaderFromConfig(config)
	if err != nil {
		t.Fatalf("Cannot create AVS Reader: %s", err.Error())
	}

	// check if the task is recorded in the contract for task index 1
	taskHash, err := avsReader.AvsServiceBindings.TaskManager.AllTaskHashes(&bind.CallOpts{}, 1)
	if err != nil {
		t.Fatalf("Cannot get task hash: %s", err.Error())
	}
	if taskHash == [32]byte{} {
		t.Fatalf("Task hash is empty")
	}

	// check if the task response is recorded in the contract for task index 1
	taskResponseHash, err := avsReader.AvsServiceBindings.TaskManager.AllTaskResponses(&bind.CallOpts{}, 1)
	log.Printf("taskResponseHash: %v", taskResponseHash)
	if err != nil {
		t.Fatalf("Cannot get task response hash: %s", err.Error())
	}
	if taskResponseHash == [32]byte{} {
		t.Fatalf("Task response hash is empty")
	}

}