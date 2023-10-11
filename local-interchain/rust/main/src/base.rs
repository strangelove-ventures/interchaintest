use std::path;

// Use clap to parse args in the future
pub const API_URL: &str = "http://127.0.0.1:8080";

/// local-interchain/rust directory
/// # Panics
///
/// Will panic if the current directory path is not found.
#[must_use]
pub fn get_current_dir() -> path::PathBuf {
    match std::env::current_dir() {
        Ok(p) => p,
        Err(e) => panic!("Could not get current dir: {e}"),
    }
}

/// local-interchain directory
/// # Panics
///
/// Will panic if the `local_interchain` directory is not found in the parent path.
#[must_use]
pub fn get_local_interchain_dir() -> path::PathBuf {
    let current_dir = get_current_dir();
    let Some(parent_dir) = current_dir.parent() else { panic!("Could not get parent dir") };
    parent_dir.to_path_buf()
}

/// local-interchain/contracts directory
#[must_use]
pub fn get_contract_path() -> path::PathBuf {
    get_local_interchain_dir().join("contracts")
}

/// local-interchain/configs/contract.json file
#[must_use]
pub fn get_contract_cache_path() -> path::PathBuf {
    get_local_interchain_dir()
        .join("configs")
        .join("contract.json")
}
