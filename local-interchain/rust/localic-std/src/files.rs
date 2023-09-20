use crate::transactions::ChainRequestBuilder;


pub fn get_files(rb: &ChainRequestBuilder, absolute_path: &str) -> Vec<String>  {        
    let cmd: String = format!("ls {}", absolute_path);
    let res = rb.exec(cmd.as_str(), true);        

    let err = res["error"].as_str();
    if err.is_some() {
        return vec![];
    }

    let text = res["text"].as_str();
    if text.is_none() {
        return vec![];
    }

    let text = text.unwrap();        

    let files = text.split("\n").filter(|s| !s.is_empty()).map(|s| s.to_string()).collect();
    files
}