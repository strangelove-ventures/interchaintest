import { Coin, SigningStargateClient, SigningStargateClientOptions, GasPrice } from '@cosmjs/stargate';
import { DirectSecp256k1HdWallet } from "@cosmjs/proto-signing";

// cosmos1hj5fveer5cjtn4wd6wstzugjfdxzl0xpxvjjvr
export const Mnemonic = "decorate bright ozone fork gallery riot bus exhaust worth way bone indoor calm squirrel merry zero scheme cotton until shop any excess stage laundry"
export const RPCAddr = "http://127.0.0.1:26657"
export const WalletPrefix = "cosmos"
export const Denom = "uatom"


// npm run start
async function main() {
    // Poll for the RPC address to be up
    const pollingLimit = 10;
    await pollForStart(pollingLimit);

    // create a wallet out of a mnemonic
    const wallet = await DirectSecp256k1HdWallet.fromMnemonic(Mnemonic, { prefix: WalletPrefix });
    const [account] = await wallet.getAccounts();

    // setup connection to the network with signing for said wallet
    // if you only need queries, you can use SigningStargateClient.connect(...)
    const options: SigningStargateClientOptions = {
        gasPrice: GasPrice.fromString("0.025"+Denom),
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

    const result = await client.sendTokens(sender, recipient, [sendAmount], 250_000, "my testing memo");

    console.log(`Sent. Transaction hash: ${result.transactionHash} at height: ${result?.height}`);

    const tx = await client.getTx(result.transactionHash);
    console.log(`Transaction: code:${tx?.code}, rawLog: ${tx?.rawLog}`);

    if (tx?.code !== 0) {
        console.log("Transaction failed. Exiting...");
        process.exit(1);
    }

    console.log("Test successful");
}

async function pollForStart(pollingLimit: number) {
    // every 5 seconds, try and poll if the RPCAddr is up. After pollingLimit, exit.
    let tries = 0;
    while (tries < pollingLimit) {
        tries++;
        try {
            await fetch(RPCAddr);
            console.log(`RPC is up. Starting...`);
            break;
        } catch (err) {
            console.log(`RPC is not up. Trying again in 5 seconds...`);
            await new Promise((r) => setTimeout(r, 5000));
        }
    }
}

main();