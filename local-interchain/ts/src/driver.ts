


// create a class which holds the API & chain id
export class Chain {
    api: string; // local-interchain
    chainID: string;

    constructor(api: string, chainID: string) {
        this.api = api;
        this.chainID = chainID;

        if(!api || !chainID) {
            throw new Error("API or chainID not provided");
        }

        if (!api.startsWith("http")) {
            throw new Error("API must be a valid http address");
        }
    }

    // generalized makeRequest function to perform actions on the API
    async makeRequest(action: string, command: string, isJSON?: boolean): Promise<any> {
        return makeRequest(this.api, this.chainID, action, command, isJSON);
    }

    // perform a query command on the application daemon
    async query(command: string, isJSON?: boolean): Promise<any> {
        return makeRequest(this.api, this.chainID, "query", command, isJSON)
    }

    // execute a binary command within the container
    // i.e. the content after appd in the command
    async binary(command: string, isJSON?: boolean): Promise<any> {
        return makeRequest(this.api, this.chainID, "bin", command, isJSON)
    }

    // execute a shell command within the container
    async shell_exec(command: string, isJSON?: boolean): Promise<any> {
        return makeRequest(this.api, this.chainID, "exec", command, isJSON)
    }

    // === RELAYER ===
    async relayer_stop(): Promise<any> {
        return makeRequest(this.api, this.chainID, "stop-relayer", "", true)
    }
    async relayer_start(): Promise<any> {
        return makeRequest(this.api, this.chainID, "start-relayer", "", true)
    }
    async relayer_exec(command: string, isJSON?: boolean): Promise<any> {
        return makeRequest(this.api, this.chainID, "relayer-exec", command, isJSON)
    }
    async relayer_get_channels(): Promise<any> {
        return makeRequest(this.api, this.chainID, "get_channels", "", true)
    }

    // === CosmWasm ===
    async wasmStoreFile(absFilePath: string, keyName: string): Promise<any> {
        return storeFile(this.api, this.chainID, absFilePath, keyName);
    }

    // other
    async storeFile(absFilePath: string): Promise<any> {
        return storeFile(this.api, this.chainID, absFilePath);
    }

    async faucet(address: string, amount: number | string): Promise<any> {
        if (typeof amount === "string") {
            amount = parseInt(amount);
        }

        return makeRequest(this.api, this.chainID, "faucet", `amount=${amount};address=${address}`, true)
    }

    async getPeer(): Promise<any> {
        return makeInfoRequest(this.api, this.chainID, "peer", false);
    }

    async recoverKey(keyName: string, mnemonic: string | string[]): Promise<any> {
        if (Array.isArray(mnemonic)) {
            mnemonic = mnemonic.join(" ");
        }

        return makeRequest(this.api, this.chainID, "recover-key", `keyname=${keyName};mnemonic=${mnemonic}`, true)
    }

    async addFullNode(amount: string | number): Promise<any> {
        if (typeof amount === "string") {
            amount = parseInt(amount);
        }
        return makeRequest(this.api, this.chainID, "add-full-nodes", `amount=${amount}`, true)
    }

    async killAll(): Promise<any> {
        return makeRequest(this.api, this.chainID, "kill-all", "", true)
    }
}

// startup
// every 5 seconds, try and poll if the RPCAddr is up. After pollingLimit, exit.
export async function pollForStart(addr: string, maxRetriesLimit: number) {
    let tries = 0;
    while (tries < maxRetriesLimit) {
        tries++;
        try {
            await fetch(addr);
            console.log(`${addr} is up. Starting...`);
            break;
        } catch (err) {
            console.log(`${addr} is not up. Trying again in 5 seconds...`);
            await new Promise((r) => setTimeout(r, 5000));
        }
    }
}

export async function makeRequest(api: string, chainID: string, action: string, command: string, isJSON?: boolean): Promise<any> {
    if (isJSON === undefined) {
        isJSON = true;
    }

    let resp = await fetch(api, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            'chain_id': chainID,
            'action': action,
            'cmd': command
        })
    });

    if (isJSON) {
        return await resp.json();
    }
    return await resp.text();
}

// set keyName if you are storing a cosmwasm contract. Else, leave it blank.
export async function storeFile(api: string, chainID: string, absFilePath: string, keyName?: string): Promise<any> {
    const isCosmWasm = keyName && keyName.length > 0;

    if (!api.endsWith("/")) {
        api = api + "/";
    }
    api = api + "upload";

    let headers: any =  {
        'Content-Type': 'application/json',
    }
    if (isCosmWasm) {
        headers['Upload-Type'] = 'cosmwasm';
    }

    let body: any = {
        'chain_id': chainID,
        'file_path': absFilePath,
    }
    if (isCosmWasm) {
        body['key_name'] = keyName;
    }

    let resp = await fetch(api, {
        method: 'POST',
        headers: headers,
        body: JSON.stringify(body)
    });

    return await resp.json();
}

export async function makeInfoRequest(api: string, chainID: string, request: string, isJSON?: boolean): Promise<any> {
    if (isJSON === undefined) {
        isJSON = true;
    }

    if (!api.endsWith("/info")) {
        api = api + "/info";
    }

    let resp = await fetch(`${api}?chain_id=${chainID}&request=${request}`);

    if (isJSON) {
        return await resp.json();
    }
    return await resp.text();
}