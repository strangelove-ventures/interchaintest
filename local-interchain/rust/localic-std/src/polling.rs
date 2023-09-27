use reqwest::blocking::Client as BClient;

use crate::errors::LocalError;

/// Polls the API URL for a response. If the response is successful, then the server is running.
/// # Errors
///
/// If the server does not start within the `wait_seconds`, then an error is returned.
pub fn poll_for_start(c: &BClient, api_url: &str, wait_seconds: u32) -> Result<(), LocalError> {
    for i in 0..wait_seconds {
        if c.get(api_url).send().is_ok() {
            return Ok(());
        }
        println!("waiting for server to start (iter:{i}) ({api_url})");
        std::thread::sleep(std::time::Duration::from_secs(1));
    }

    Err(LocalError::ServerDidNotStart {})
}

// TODO: polling for a future block (wait_until) delta
