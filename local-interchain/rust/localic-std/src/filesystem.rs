use crate::{errors::LocalError, transactions::ChainRequestBuilder};

/// # Errors
///
/// Returns `Err` if the files can not be found.
pub fn get_files(rb: &ChainRequestBuilder, absolute_path: &str) -> Result<Vec<String>, LocalError> {
    let cmd: String = format!("ls {absolute_path}");
    let res = rb.exec(cmd.as_str(), true);

    if let Some(err) = res["error"].as_str() {
        return Err(LocalError::GetFilesError {
            error: err.to_string(),
        });
    };

    let text = res["text"].as_str();

    let Some(text) = text else { return Ok(vec![]) };

    Ok(text
        .split('\n')
        .filter(|s| !s.is_empty())
        .map(std::string::ToString::to_string)
        .collect())
}
