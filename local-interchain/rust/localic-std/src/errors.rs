use thiserror::Error;

#[derive(Error, Debug, Clone)]
pub enum LocalError {
    #[error("{{msg}}")]
    Custom { msg: String },

    #[error("This err returned is NotImplemented")]
    NotImplemented {},

    #[error("the transaction hash was not found.")]
    TxHashNotFound {},

    #[error("file upload failed for path: {path}. reason: {reason}")]
    UploadFailed { path: String, reason: String },

    #[error("transaction was not successful. status: {code_status}. log: {raw_log}")]
    TxNotSuccessful { code_status: i64, raw_log: String },

    #[error("contract address not found. {events}")]
    ContractAddressNotFound { events: String },

    #[error("key_bech32 failed. reason: {reason}")]
    KeyBech32Failed { reason: String },
}
