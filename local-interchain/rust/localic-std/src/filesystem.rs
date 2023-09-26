use crate::transactions::ChainRequestBuilder;

#[must_use]
pub fn get_files(rb: &ChainRequestBuilder, absolute_path: &str) -> Vec<String> {
    let cmd: String = format!("ls {absolute_path}");
    let res = rb.exec(cmd.as_str(), true);

    println!("res: {res:?}");

    if let Some(err) = res["error"].as_str() {
        println!("get_files err: {err:?}");
        return vec![];
    };

    let text = res["text"].as_str();

    let Some(text) = text else { return vec![] };

    text.split('\n')
        .filter(|s| !s.is_empty())
        .map(std::string::ToString::to_string)
        .collect()
}
