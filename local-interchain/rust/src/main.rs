use reqwest::blocking::Client;

pub mod polling;
use polling::poll_for_start;

pub mod base;
use base::{API_URL};

pub mod transactions;
use transactions::{send_request, RequestBase, RequestType};

// #[tokio::main] - moving away from async
fn main() {
    let client = Client::new();        
    poll_for_start(client, API_URL, 150);

    let b = RequestBase::new(API_URL.to_string(), "localjuno-1".to_string(), RequestType::Query);

    let s = send_request(b, "q auth accounts".to_string(), Some(false), Some(true));
    println!("s: {:?}", s);    

    println!("Hello, world!");
}
