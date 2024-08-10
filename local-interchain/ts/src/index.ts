import { SigningStargateClient, SigningStargateClientOptions, GasPrice } from '@cosmjs/stargate';
import { DirectSecp256k1HdWallet } from "@cosmjs/proto-signing";
import path from 'path';

import { Chain, pollForStart, makeRequest } from './driver';

// local-ic start juno_ibc

// juno1hj5fveer5cjtn4wd6wstzugjfdxzl0xps73ftl
const Mnemonic = "decorate bright ozone fork gallery riot bus exhaust worth way bone indoor calm squirrel merry zero scheme cotton until shop any excess stage laundry"
const RPCAddr = "http://127.0.0.1:26657"
const WalletPrefix = "juno"
const Denom = "ujuno"

 // local-interchain
const APIAddr = "http://127.0.0.1:8080"
const ChainID = "localjuno-1"

// Files
const localInterchainDir = path.dirname(path.dirname(__dirname))
const contract = path.join(localInterchainDir, "contracts", "cw_ibc_example.wasm")
const randomFile = path.join(localInterchainDir, "README.md")

// npm run start
async function main() {
    // Poll for the RPC address to be up
    await pollForStart(APIAddr, 25);

    await localICInteractions(new Chain(APIAddr, ChainID), contract, randomFile);

    await CosmJSInteractions();

    console.log("Test successful");
}

async function localICInteractions(chain: Chain, abcWasmFileLoc: string, absRandomFileLoc: string): Promise<void> {
    await chain.binary("config keyring-backend test")
    await chain.binary("config output json")

    // verify query request 2 different ways
    let total = await chain.makeRequest("query", "bank total")
    console.log("total", total)
    exitIfEmpty(total, "total")

    let total2 = await chain.query("bank total")
    console.log("total2", total2)
    exitIfEmpty(total2, "total2")

    if (total.supply[0].denom !== total2.supply[0].denom) {
        console.log("Query results are not the same. Exiting...");
        process.exit(1);
    }

    if (parseInt(total.supply[0].amount) === 0) {
        console.log("bank total is 0. Exiting...");
        process.exit(1);
    }

    // faucet
    const faucetToAddr = "juno1hj5fveer5cjtn4wd6wstzugjfdxzl0xps73ftl"
    let beforeBal = await chain.query(`bank balances ${faucetToAddr}`)
    let prevBal: number = parseInt(beforeBal.balances[0].amount)

    const faucetAmt = 787
    let faucet = await chain.faucet(faucetToAddr, faucetAmt)
    console.log("faucet", faucet)

    let newBal = await chain.query(`bank balances ${faucetToAddr}`)
    if (parseInt(newBal.balances[0].amount) !== prevBal + faucetAmt) {
        console.log("Faucet failed. Exiting...");
        process.exit(1);
    }

    // CosmWasm
    console.log("Storing wasm file...")
    let cwRes = await chain.wasmStoreFile(abcWasmFileLoc, "faucet")
    console.log("cwRes", cwRes)
    exitIfEmpty(cwRes.code_id, "codeID")

    // Random file
    let miscFile = await chain.storeFile(absRandomFileLoc)
    console.log("miscFile", miscFile)
    let dockerFileLoc = miscFile.location

    // Verify file contents are there
    let readFile = await chain.shell_exec(`cat ${dockerFileLoc}`, false)
    exitIfEmpty(readFile, "readFile")

    // get peer
    let peer = await chain.getPeer()
    console.log("peer", peer)
    exitIfEmpty(peer, "peer")

    // relayer channels
    let channels = await chain.relayer_get_channels()
    console.log("channels", channels)
    if (channels.length === 0) {
        console.log("No channels found. Exiting...");
        process.exit(1);
    }

    // relayer exec
    let rlyBal = await chain.relayer_exec(`rly q balance ${ChainID} --output=json`)
    console.log("rlyBal", rlyBal)
    exitIfEmpty(rlyBal, "rlyBal")

    let recoverKey = await chain.recoverKey("mynewkey",
        "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon art",
    );
    console.log("recoverKey", recoverKey)
    exitIfEmpty(recoverKey, "recoverKey")

    // query key address with bin
    let key = await chain.binary("keys show mynewkey -a", false)
    console.log("key", key)
    exitIfEmpty(key, "key")

    // Add full node
    let addNode = await chain.addFullNode(1)
    console.log("addNode", addNode)
    exitIfEmpty(addNode, "addNode")
}

async function CosmJSInteractions(): Promise<void> {
    // create a wallet out of a mnemonic
    const wallet = await DirectSecp256k1HdWallet.fromMnemonic(Mnemonic, { prefix: WalletPrefix });
    const [account] = await wallet.getAccounts();

    // setup connection to the network with signing for said wallet
    // if you only need queries, you can use SigningStargateClient.connect(...)
    const options: SigningStargateClientOptions = {
        gasPrice: GasPrice.fromString("0.0025"+Denom),
    };

    const client = await SigningStargateClient.connectWithSigner(RPCAddr, wallet, options).then((client) => {
        console.log(`Successfully connected to node ${RPCAddr}`);
        return client;
    }).catch((err) => {
        if (err.toString().includes("fetch failed")) {
            console.log(`Error: ensure the testnet is running and the RPC address (${RPCAddr}) is correct.`);
            process.exit(1);
        } else {
            console.log(err);
        }

        return undefined;
    });

    if (client === undefined) {
        console.log("SigningStargateClient is undefined. Exiting...");
        process.exit(1);
    }

    // Grab the accounts balance
    const balance = await client.getBalance(account.address, Denom)
    console.log(`${account.address} balance: ${balance.amount}${Denom}`);

    // Validate the account has a balance > 0.
    if (parseInt(balance.amount) === 0) {
        console.log("Account balance is 0. Exiting...");
        process.exit(1);
    }

    // send a token to itself
    const sendAmount = {
        denom: Denom,
        amount: "1",
    };

    const recipient = account.address;
    const sender = account.address;

    console.log(`Sending ${sendAmount.amount}${sendAmount.denom} from ${sender} to ${recipient}`);

    const result = await client.sendTokens(sender, recipient, [sendAmount], 1000, "my testing memo");

    console.log(`Sent. Transaction hash: ${result.transactionHash} at height: ${result?.height}`);

    const tx = await client.getTx(result.transactionHash);
    console.log(`Transaction: code:${tx?.code}, rawLog: ${tx?.rawLog}`);

    if (tx?.code !== 0) {
        console.log("Transaction failed. Exiting...");
        process.exit(1);
    }
}

export function exitIfEmpty(data: any, name: string) {
    if (!data || data === "") {
        console.log(`${name} is empty. Exiting...`);
        process.exit(1);
    }
}

main();