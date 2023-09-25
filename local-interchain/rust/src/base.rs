use std::path;

// Use clap to parse args in the future
pub const API_URL: &str = "http://127.0.0.1:8080";

// local-interchain/rust
pub fn get_current_dir() -> path::PathBuf {
    let current_dir = std::env::current_dir().unwrap();
    current_dir
}

// local-interchain
pub fn get_local_interchain_dir() -> path::PathBuf {
    let parent_dir = get_current_dir().parent().unwrap().to_path_buf();
    parent_dir
}

// local-interchain/contracts
pub fn get_contract_path() -> path::PathBuf {
    let contract_path = get_local_interchain_dir().join("contracts");
    contract_path
}

// local-interchain/configs/contract.json
pub fn get_contract_cache_path() -> path::PathBuf {
    let contract_json_path = get_local_interchain_dir()
        .join("configs")
        .join("contract.json");
    contract_json_path
}
