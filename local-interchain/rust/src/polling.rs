use reqwest::blocking::Client as BClient;

pub fn poll_for_start(c: BClient, api_url: &str, wait_seconds: u32) {
    for i in 0..wait_seconds + 1 {        
        match c.get(api_url).send() {
            Ok(_) => return,
            Err(_) => {                
                println!("waiting for server to start (iter:{}) ({})", i, api_url);
                std::thread::sleep(std::time::Duration::from_secs(1));
            }
        }
    }

    panic!("Local-IC REST API Server did not start in time");
}