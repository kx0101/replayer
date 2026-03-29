use std::fs::File;
use std::io::{BufRead, BufReader};

use anyhow::{bail, Context};
use replayer_core::models::LogEntry;

pub fn read_entries(input_file: &str, limit: usize) -> anyhow::Result<Vec<LogEntry>> {
    let file = File::open(input_file).context("failed to open file")?;
    let reader = BufReader::new(file);
    let mut entries = Vec::new();
    let mut line_num = 0u64;

    for line_result in reader.lines() {
        if limit > 0 && entries.len() >= limit {
            break;
        }

        let line = line_result?;
        line_num += 1;

        if line.trim().is_empty() {
            continue;
        }

        match serde_json::from_str::<LogEntry>(&line) {
            Ok(entry) => entries.push(entry),
            Err(err) => {
                eprintln!("invalid JSON object {}: {}", line_num, err);
            }
        }
    }

    Ok(entries)
}

pub fn dry_run(input: &str) -> anyhow::Result<()> {
    if input.contains("..") {
        bail!("invalid input path: {}", input);
    }

    let file = File::open(input).context("failed to open file")?;
    let reader = BufReader::new(file);
    let mut line_num = 0u64;

    for line_result in reader.lines() {
        let line = line_result?;
        line_num += 1;

        if line.trim().is_empty() {
            continue;
        }

        match serde_json::from_str::<LogEntry>(&line) {
            Ok(entry) => {
                println!("[DRY RUN] - {}: {:?}", line_num, entry);
            }
            Err(err) => {
                eprintln!("invalid JSON object {}: {}", line_num, err);
            }
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn write_temp(content: &str) -> tempfile::NamedTempFile {
        use std::io::Write;
        let mut f = tempfile::NamedTempFile::new().unwrap();
        f.write_all(content.as_bytes()).unwrap();
        f.flush().unwrap();
        f
    }

    #[test]
    fn test_read_entries_valid_json_lines() {
        let content = r#"{"method":"GET","path":"/users","headers":{},"body":""}
{"method":"POST","path":"/users","headers":{"Content-Type":["application/json"]},"body":"eyJ0ZXN0IjoidmFsdWUifQ=="}
{"method":"DELETE","path":"/users/123","headers":{},"body":""}
"#;
        let f = write_temp(content);
        let entries = read_entries(f.path().to_str().unwrap(), 0).unwrap();
        assert_eq!(entries.len(), 3);
        assert_eq!(entries[0].method, "GET");
        assert_eq!(entries[1].method, "POST");
        assert_eq!(entries[2].method, "DELETE");
    }

    #[test]
    fn test_read_entries_with_limit() {
        let content = r#"{"method":"GET","path":"/1","headers":{},"body":""}
{"method":"GET","path":"/2","headers":{},"body":""}
{"method":"GET","path":"/3","headers":{},"body":""}
{"method":"GET","path":"/4","headers":{},"body":""}
{"method":"GET","path":"/5","headers":{},"body":""}
"#;
        let f = write_temp(content);
        let entries = read_entries(f.path().to_str().unwrap(), 3).unwrap();
        assert_eq!(entries.len(), 3);
    }

    #[test]
    fn test_read_entries_invalid_json_skipped() {
        let content = r#"{"method":"GET","path":"/valid1","headers":{},"body":""}
{invalid json line
{"method":"POST","path":"/valid2","headers":{},"body":""}
"#;
        let f = write_temp(content);
        let entries = read_entries(f.path().to_str().unwrap(), 0).unwrap();
        assert_eq!(entries.len(), 2);
        assert_eq!(entries[0].path, "/valid1");
        assert_eq!(entries[1].path, "/valid2");
    }

    #[test]
    fn test_read_entries_empty_file() {
        let f = write_temp("");
        let entries = read_entries(f.path().to_str().unwrap(), 0).unwrap();
        assert_eq!(entries.len(), 0);
    }

    #[test]
    fn test_read_entries_file_not_found() {
        let result = read_entries("nonexistent_file_12345.json", 0);
        assert!(result.is_err());
    }

    #[test]
    fn test_read_entries_complex_with_headers_and_body() {
        let content = r#"{"method":"POST","path":"/api/users","headers":{"Content-Type":["application/json"],"Authorization":["Bearer token123"]},"body":"eyJuYW1lIjoiTGlha29zIiwiYWdlIjozMH0="}"#;
        let f = write_temp(content);
        let entries = read_entries(f.path().to_str().unwrap(), 0).unwrap();
        assert_eq!(entries.len(), 1);
        let entry = &entries[0];
        assert_eq!(entry.method, "POST");
        assert_eq!(entry.path, "/api/users");
        assert_eq!(entry.headers.len(), 2);
        assert_eq!(entry.headers["Content-Type"][0], "application/json");
        assert!(!entry.body.is_empty());
    }

    #[test]
    fn test_dry_run_valid_file() {
        let content = r#"{"method":"GET","path":"/test1","headers":{},"body":""}
{"method":"POST","path":"/test2","headers":{},"body":""}
"#;
        let f = write_temp(content);
        let result = dry_run(f.path().to_str().unwrap());
        assert!(result.is_ok());
    }

    #[test]
    fn test_dry_run_invalid_path_with_parent_directory() {
        let result = dry_run("../etc/passwd");
        assert!(result.is_err());
        assert!(result
            .unwrap_err()
            .to_string()
            .contains("invalid input path"));
    }

    #[test]
    fn test_dry_run_nonexistent_file() {
        let result = dry_run("nonexistent_file_12345.json");
        assert!(result.is_err());
    }

    #[test]
    fn test_dry_run_invalid_json_handled_gracefully() {
        let content = r#"{"method":"GET","path":"/valid","headers":{},"body":""}
invalid json
{"method":"POST","path":"/valid2","headers":{},"body":""}
"#;
        let f = write_temp(content);
        let result = dry_run(f.path().to_str().unwrap());
        assert!(result.is_ok());
    }
}
