use std::error;

use thiserror::Error;

#[derive(Error, Debug)]
pub enum LocalError {
    #[error("the transaction hash was not found.")]
    TxHashNotFound { },
}