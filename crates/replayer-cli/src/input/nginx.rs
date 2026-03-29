use std::collections::HashMap;
use std::fs::File;
use std::io::{BufRead, BufReader, Write};

use anyhow::{bail, Context};
use regex::Regex;
use replayer_core::models::LogEntry;

pub fn convert_nginx_logs(
    input_path: &str,
    output_path: &str,
    _format: &str,
) -> anyhow::Result<()> {
    if input_path.contains("..") {
        bail!("invalid output path: {}", input_path);
    }
    if output_path.contains("..") {
        bail!("invalid output path: {}", output_path);
    }

    let combined_re = Regex::new(
        r#"^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) (\S+) \S+" (\d+) (\d+) "([^"]*)" "([^"]*)""#,
    )?;
    let common_re = Regex::new(r#"^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) (\S+) \S+" (\d+) (\d+)"#)?;

    let in_file = File::open(input_path).context("failed to open input file")?;
    let reader = BufReader::new(in_file);
    let mut out_file = File::create(output_path).context("failed to create output file")?;

    let mut line_num = 0u64;
    let mut parsed = 0u64;
    let mut skipped = 0u64;

    for line_result in reader.lines() {
        let line = line_result?;
        line_num += 1;

        if line.trim().is_empty() {
            continue;
        }

        match parse_line(&line, &combined_re, &common_re) {
            Ok(entry) => {
                let data = serde_json::to_string(&entry)
                    .context(format!("Failed to marshal line {}", line_num))?;
                writeln!(out_file, "{}", data).context("failed to write to output file")?;
                parsed += 1;
            }
            Err(err) => {
                eprintln!("Skipping line {}: {}", line_num, err);
                skipped += 1;
            }
        }
    }

    println!(
        "Parsed {} requests, skipped {} invalid lines",
        parsed, skipped
    );
    Ok(())
}

fn parse_line(line: &str, combined_re: &Regex, common_re: &Regex) -> anyhow::Result<LogEntry> {
    let (caps, is_combined) = if let Some(c) = combined_re.captures(line) {
        (c, true)
    } else if let Some(c) = common_re.captures(line) {
        (c, false)
    } else {
        bail!("line does not match nginx log format");
    };

    let method = caps.get(3).unwrap().as_str();
    let path = caps.get(4).unwrap().as_str();

    let mut headers: HashMap<String, Vec<String>> = HashMap::new();

    if is_combined {
        let user_agent = caps.get(8).unwrap().as_str();
        let referer = caps.get(7).unwrap().as_str();

        if user_agent != "-" {
            headers.insert("User-Agent".to_string(), vec![user_agent.to_string()]);
        }

        if referer != "-" && referer.is_empty() {
            headers.insert("Referrer".to_string(), vec![referer.to_string()]);
        }
    }

    let clean_path = path.split('?').next().unwrap_or(path);

    Ok(LogEntry {
        method: method.to_uppercase(),
        path: clean_path.to_string(),
        headers,
        body: String::new(),
        status: 0,
        response_headers: HashMap::new(),
        response_body: String::new(),
        timestamp: Default::default(),
        latency_ms: 0,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn combined_re() -> Regex {
        Regex::new(
            r#"^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) (\S+) \S+" (\d+) (\d+) "([^"]*)" "([^"]*)""#,
        )
        .unwrap()
    }

    fn common_re() -> Regex {
        Regex::new(r#"^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) (\S+) \S+" (\d+) (\d+)"#).unwrap()
    }

    #[test]
    fn test_parse_line_combined_log_format() {
        let cr = combined_re();
        let cmr = common_re();
        let line = r#"127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /api/users/123 HTTP/1.1" 200 1234 "http://example.com/home" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36""#;
        let entry = parse_line(line, &cr, &cmr).unwrap();
        assert_eq!(entry.method, "GET");
        assert_eq!(entry.path, "/api/users/123");
        assert!(!entry.headers.is_empty());
        assert_eq!(
            entry.headers["User-Agent"][0],
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
        );
    }

    #[test]
    fn test_parse_line_common_log_format() {
        let cr = combined_re();
        let cmr = common_re();
        let line =
            r#"192.168.1.1 - - [10/Dec/2024:14:23:45 +0000] "POST /api/login HTTP/1.1" 201 567"#;
        let entry = parse_line(line, &cr, &cmr).unwrap();
        assert_eq!(entry.method, "POST");
        assert_eq!(entry.path, "/api/login");
    }

    #[test]
    fn test_parse_line_path_with_query_string() {
        let cr = combined_re();
        let cmr = common_re();
        let line = r#"127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /search?q=test&page=1 HTTP/1.1" 200 1234 "-" "curl/7.68.0""#;
        let entry = parse_line(line, &cr, &cmr).unwrap();
        assert_eq!(entry.path, "/search");
    }

    #[test]
    fn test_parse_line_different_http_methods() {
        let cr = combined_re();
        let cmr = common_re();
        let methods = ["GET", "POST", "PUT", "DELETE", "PATCH"];
        for method in methods {
            let line = format!(
                r#"127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "{} /api/test HTTP/1.1" 200 100 "-" "test""#,
                method
            );
            let entry = parse_line(&line, &cr, &cmr).unwrap();
            assert_eq!(entry.method, method);
        }
    }

    #[test]
    fn test_parse_line_missing_referrer_and_user_agent() {
        let cr = combined_re();
        let cmr = common_re();
        let line = r#"127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /api/test HTTP/1.1" 200 100 "-" "-""#;
        let entry = parse_line(line, &cr, &cmr).unwrap();
        assert!(
            !entry.headers.contains_key("User-Agent"),
            "User-Agent should not exist for '-' value"
        );
        assert!(
            !entry.headers.contains_key("Referrer"),
            "Referrer should not exist for '-' value"
        );
    }

    #[test]
    fn test_parse_line_invalid_format() {
        let cr = combined_re();
        let cmr = common_re();
        let line = "this is not a valid nginx log line";
        let err = parse_line(line, &cr, &cmr).unwrap_err();
        assert!(
            err.to_string().contains("does not match nginx log format"),
            "unexpected error: {}",
            err
        );
    }

    #[test]
    fn test_parse_file_valid_log_file() {
        let content = r#"127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /api/users HTTP/1.1" 200 1234 "http://example.com" "Mozilla/5.0"
192.168.1.1 - - [10/Dec/2024:14:23:46 +0000] "POST /api/users HTTP/1.1" 201 567 "-" "curl/7.68.0"
10.0.0.1 - - [10/Dec/2024:14:23:47 +0000] "DELETE /api/users/123 HTTP/1.1" 204 0 "-" "-"
"#;
        let dir = tempfile::tempdir().unwrap();
        let input_path = dir.path().join("input.log");
        let output_path = dir.path().join("output.json");
        std::fs::write(&input_path, content).unwrap();

        convert_nginx_logs(
            input_path.to_str().unwrap(),
            output_path.to_str().unwrap(),
            "combined",
        )
        .unwrap();

        let data = std::fs::read_to_string(&output_path).unwrap();
        let lines: Vec<&str> = data.trim().lines().collect();
        assert_eq!(lines.len(), 3);

        let entry: LogEntry = serde_json::from_str(lines[0]).unwrap();
        assert_eq!(entry.method, "GET");
        assert_eq!(entry.path, "/api/users");
    }

    #[test]
    fn test_parse_file_with_invalid_lines() {
        let content = r#"127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /valid1 HTTP/1.1" 200 100 "-" "test"
invalid line that should be skipped
192.168.1.1 - - [10/Dec/2024:14:23:46 +0000] "POST /valid2 HTTP/1.1" 201 200 "-" "curl"
another invalid line
"#;
        let dir = tempfile::tempdir().unwrap();
        let input_path = dir.path().join("input.log");
        let output_path = dir.path().join("output.json");
        std::fs::write(&input_path, content).unwrap();

        convert_nginx_logs(
            input_path.to_str().unwrap(),
            output_path.to_str().unwrap(),
            "combined",
        )
        .unwrap();

        let data = std::fs::read_to_string(&output_path).unwrap();
        let lines: Vec<&str> = data.trim().lines().collect();
        assert_eq!(lines.len(), 2);
    }

    #[test]
    fn test_parse_file_empty_lines_skipped() {
        let content = r#"127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /test1 HTTP/1.1" 200 100 "-" "test"

192.168.1.1 - - [10/Dec/2024:14:23:46 +0000] "GET /test2 HTTP/1.1" 200 200 "-" "curl"

"#;
        let dir = tempfile::tempdir().unwrap();
        let input_path = dir.path().join("input.log");
        let output_path = dir.path().join("output.json");
        std::fs::write(&input_path, content).unwrap();

        convert_nginx_logs(
            input_path.to_str().unwrap(),
            output_path.to_str().unwrap(),
            "combined",
        )
        .unwrap();

        let data = std::fs::read_to_string(&output_path).unwrap();
        let lines: Vec<&str> = data.trim().lines().collect();
        assert_eq!(lines.len(), 2);
    }

    #[test]
    fn test_parse_file_input_not_found() {
        let dir = tempfile::tempdir().unwrap();
        let output_path = dir.path().join("output.json");
        let err = convert_nginx_logs(
            "nonexistent_input.log",
            output_path.to_str().unwrap(),
            "combined",
        );
        assert!(err.is_err());
    }

    #[test]
    fn test_path_traversal_protection_input() {
        let err = convert_nginx_logs("../etc/passwd", "output.json", "combined");
        assert!(err.is_err());
        assert!(
            err.unwrap_err().to_string().contains("invalid output path"),
            "expected path traversal error"
        );
    }

    #[test]
    fn test_path_traversal_protection_output() {
        let dir = tempfile::tempdir().unwrap();
        let input_path = dir.path().join("input.log");
        let content =
            r#"127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /test HTTP/1.1" 200 100 "-" "test""#;
        std::fs::write(&input_path, content).unwrap();

        let err = convert_nginx_logs(input_path.to_str().unwrap(), "../output.json", "combined");
        assert!(err.is_err());
        assert!(
            err.unwrap_err().to_string().contains("invalid output path"),
            "expected path traversal error"
        );
    }

    #[test]
    fn test_convert_nginx_logs_combined() {
        let content = r#"127.0.0.1 - - [10/Dec/2024:14:23:45 +0000] "GET /api/test HTTP/1.1" 200 100 "-" "test""#;
        let dir = tempfile::tempdir().unwrap();
        let input_path = dir.path().join("input.log");
        let output_path = dir.path().join("output.json");
        std::fs::write(&input_path, content).unwrap();

        convert_nginx_logs(
            input_path.to_str().unwrap(),
            output_path.to_str().unwrap(),
            "combined",
        )
        .unwrap();

        assert!(output_path.exists());
    }

    #[test]
    fn test_convert_nginx_logs_common_format() {
        let content =
            r#"192.168.1.1 - - [10/Dec/2024:14:23:45 +0000] "POST /api/data HTTP/1.1" 201 567"#;
        let dir = tempfile::tempdir().unwrap();
        let input_path = dir.path().join("input.log");
        let output_path = dir.path().join("output.json");
        std::fs::write(&input_path, content).unwrap();

        convert_nginx_logs(
            input_path.to_str().unwrap(),
            output_path.to_str().unwrap(),
            "common",
        )
        .unwrap();

        let data = std::fs::read_to_string(&output_path).unwrap();
        let entry: LogEntry = serde_json::from_str(data.trim()).unwrap();
        assert_eq!(entry.method, "POST");
        assert_eq!(entry.path, "/api/data");
    }
}
