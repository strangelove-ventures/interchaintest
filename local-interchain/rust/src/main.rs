use reqwest::blocking::Client;

pub mod polling;
use polling::poll_for_start;

pub mod base;
use base::API_URL;

pub mod transactions;
use transactions::RequestBuilder;

fn main() {
    let client = Client::new();
    poll_for_start(client.clone(), &API_URL, 150);
    
    let req_builder = RequestBuilder::new(API_URL.to_string(), "localjuno-1".to_string(), true);
    
    // queries
    // get_all_accounts(&req_builder);
    get_bank_total_supply(&req_builder);


    // run a binary command
    // get_keyring_accounts(&req_builder);
    // transaction_decode(&req_builder);    


    // rb.binary("keys list --keyring-backend=test --output=json");
    // rb.binary("config keyring-backend test");
    // rb.binary("config");
}

fn get_all_accounts(req_builder: &RequestBuilder) {
    let res = req_builder.query("q auth accounts --output=json");
    let accounts = res["accounts"].as_array().unwrap();    

    accounts.iter().for_each(|account| {
        let acc_type = account["@type"].as_str().unwrap_or_default();
        
        let addr: &str = match acc_type {
            "/cosmos.auth.v1beta1.ModuleAccount" => {
                account["base_account"]["address"].as_str().unwrap_or_default()
            }
            _ => account["address"].as_str().unwrap_or_default(),
        };
                
        println!("{}: {}", acc_type, addr);
    });
}

fn get_bank_total_supply(req_builder: &RequestBuilder) {
    // Total supply: {"pagination":{"next_key":null,"total":"0"},"supply":[{"amount":"110048643629768","denom":"ujuno"}]}
    let res = req_builder.query("q bank total");
    
    // iter all supplies and print them. Also take amount, convert to Uint128, divide by 6, and remove the u from the front.
    let supplies = res["supply"].as_array().unwrap();
    supplies.iter().for_each(|supply| {
        let amount = supply["amount"].as_str().unwrap_or_default();
        let denom = supply["denom"].as_str().unwrap_or_default();
        let amount = amount.parse::<u128>().unwrap_or_default();
        
        let human_denom = denom[1..].to_string().to_uppercase();
        let human_amount = amount / 1000000;
        println!("{}: {} = ({}: {})", denom, amount, human_denom, human_amount);
    });
}

fn get_keyring_accounts(req_builder: &RequestBuilder) {    
    let accounts = req_builder.binary("keys list --keyring-backend=test");
    
    accounts.as_array().unwrap().iter().for_each(|account| {
        let name = account["name"].as_str().unwrap_or_default();
        let addr = account["address"].as_str().unwrap_or_default();
        println!("Key '{}': {}", name, addr);
    });
}

fn transaction_decode(req_builder: &RequestBuilder) {
    let cmd = "tx decode ClMKUQobL2Nvc21vcy5nb3YudjFiZXRhMS5Nc2dWb3RlEjIIpwISK2p1bm8xZGM3a2MyZzVrZ2wycmdmZHllZGZ6MDl1YTlwZWo1eDNsODc3ZzcYARJmClAKRgofL2Nvc21vcy5jcnlwdG8uc2VjcDI1NmsxLlB1YktleRIjCiECxjGMmYp4MlxxfFWi9x4u+jOleJVde3Cru+HnxAVUJmgSBAoCCH8YNBISCgwKBXVqdW5vEgMyMDQQofwEGkDPE4dCQ4zUh6LIB9wqNXDBx+nMKtg0tEGiIYEH8xlw4H8dDQQStgAe6xFO7I/oYVSWwa2d9qUjs9qyB8r+V0Gy";
    let res = req_builder.binary(cmd);
    println!("{}", res);
} 