use thiserror::Error;

#[derive(Error, Debug)]
pub enum LocalError {
    #[error("This err returned is NotImplemented")]
    NotImplemented {},

    #[error("the transaction hash was not found.")]
    TxHashNotFound {},

    #[error("CosmWasm upload failed for path: {path}. Reason: {reason}")]
    CWUploadFailed { path: String, reason: String },
}
