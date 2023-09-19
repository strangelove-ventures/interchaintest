use thiserror::Error;

#[derive(Error, Debug)]
pub enum LocalError {
    #[error("{{msg}}")]
    Custom { msg: String },

    #[error("This err returned is NotImplemented")]
    NotImplemented {},

    #[error("the transaction hash was not found.")]
    TxHashNotFound {},

    #[error("CosmWasm upload failed for path: {path}. Reason: {reason}")]
    CWUploadFailed { path: String, reason: String },

    #[error("Transaction was not successful. Status: {code_status}. log: {raw_log}")]
    TxNotSuccessful { code_status: i64, raw_log: String },

    #[error("Contract address not found")]
    ContractAddressNotFound {},
}
