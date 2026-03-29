use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, Instant};

use base64::{engine::general_purpose, Engine as _};
use reqwest::header::{HeaderMap, HeaderName, HeaderValue};
use sha2::{Digest, Sha256};
use tokio::sync::Semaphore;

use replayer_core::models::{LogEntry, MultiEnvResult, ReplayResult, ResponseDiff};

use crate::cli::CliArgs;
use crate::replay::progress::ProgressBar;
use crate::replay::volatile::{config_from_flags, detailed_compare, VolatileConfig};

const LATENCY_BUCKET_MS: i64 = 5;

pub async fn run(entries: &[LogEntry], args: &CliArgs) -> Vec<MultiEnvResult> {
    let client = reqwest::Client::builder()
        .timeout(Duration::from_millis(args.timeout as u64))
        .build()
        .expect("failed to create HTTP client");

    let semaphore = Arc::new(Semaphore::new(args.concurrency));

    let mut results = Vec::with_capacity(entries.len());

    let mut interval = if args.rate_limit > 0 {
        Some(tokio::time::interval(Duration::from_nanos(
            1_000_000_000 / args.rate_limit as u64,
        )))
    } else {
        None
    };

    let volatile_config = if args.ignore_volatile {
        Some(config_from_flags(
            &args.ignore_fields,
            &args.ignore_patterns,
        ))
    } else {
        None
    };

    let mut targets = args.targets.clone();
    targets.sort();

    let pb = if args.progress_bar && !args.output_json {
        Some(ProgressBar::new(entries.len() as u64))
    } else {
        None
    };

    for (i, entry) in entries.iter().enumerate() {
        if let Some(ref mut interval) = interval {
            interval.tick().await;
        }

        let mut handles = Vec::new();
        for target in &targets {
            let permit = semaphore.clone().acquire_owned().await.unwrap();
            let client = client.clone();
            let entry = entry.clone();
            let target = target.clone();
            let tls_cert = args.tls_cert.clone();
            let tls_key = args.tls_key.clone();
            let auth_header = args.auth_header.clone();
            let headers = args.headers.clone();

            handles.push(tokio::spawn(async move {
                let result = replay_single(
                    i as i32,
                    &entry,
                    &client,
                    &target,
                    &tls_cert,
                    &tls_key,
                    &auth_header,
                    &headers,
                )
                .await;
                drop(permit);
                (target, result)
            }));
        }

        let mut responses = HashMap::new();
        for handle in handles {
            if let Ok((target, result)) = handle.await {
                responses.insert(target, result);
            }
        }

        let mut result = MultiEnvResult {
            index: i as i32,
            request: entry.clone(),
            request_id: fingerprint(entry),
            responses,
            diff: None,
        };

        if args.compare && targets.len() > 1 {
            result.diff = compare_responses_deterministic(
                &result.responses,
                &targets,
                volatile_config.as_ref(),
                args.show_volatile_diffs,
            );
        }

        results.push(result);

        if let Some(ref pb) = pb {
            pb.increment();
        }

        if args.delay > 0 {
            tokio::time::sleep(Duration::from_millis(args.delay as u64)).await;
        }
    }

    if let Some(ref pb) = pb {
        pb.finish();
    }

    results
}

#[allow(clippy::too_many_arguments)]
pub async fn replay_single(
    index: i32,
    entry: &LogEntry,
    client: &reqwest::Client,
    target: &str,
    tls_cert: &str,
    tls_key: &str,
    auth_header: &str,
    custom_headers: &[String],
) -> ReplayResult {
    let scheme = if !tls_cert.is_empty() && !tls_key.is_empty() {
        "https"
    } else {
        "http"
    };

    let url = format!("{}://{}{}", scheme, target, entry.path);

    let body_bytes = if !entry.body.is_empty() && entry.body != "null" {
        match general_purpose::STANDARD.decode(&entry.body) {
            Ok(b) => Some(b),
            Err(_) => Some(entry.body.as_bytes().to_vec()),
        }
    } else {
        None
    };

    let method = match entry.method.parse::<reqwest::Method>() {
        Ok(m) => m,
        Err(e) => return wrap_error(index, &format!("invalid method: {}", e), 0),
    };

    let mut headers = HeaderMap::new();

    let mut header_keys: Vec<&String> = entry.headers.keys().collect();
    header_keys.sort();
    for k in header_keys {
        let mut values = entry.headers[k].clone();
        values.sort();
        for v in &values {
            if let (Ok(name), Ok(value)) = (
                HeaderName::from_bytes(k.as_bytes()),
                HeaderValue::from_str(v),
            ) {
                headers.append(name, value);
            }
        }
    }

    if !auth_header.is_empty() {
        if let Ok(value) = HeaderValue::from_str(auth_header) {
            headers.insert(reqwest::header::AUTHORIZATION, value);
        }
    }

    for h in custom_headers {
        if let Some((key, value)) = h.split_once(':') {
            if let (Ok(name), Ok(val)) = (
                HeaderName::from_bytes(key.trim().as_bytes()),
                HeaderValue::from_str(value.trim()),
            ) {
                headers.insert(name, val);
            }
        }
    }

    if body_bytes.is_some() && !headers.contains_key(reqwest::header::CONTENT_TYPE) {
        headers.insert(
            reqwest::header::CONTENT_TYPE,
            HeaderValue::from_static("application/json"),
        );
    }

    if !headers.contains_key(reqwest::header::USER_AGENT) {
        headers.insert(
            reqwest::header::USER_AGENT,
            HeaderValue::from_static("go-http-replayer/1.0"),
        );
    }

    let mut request = client.request(method, &url).headers(headers);
    if let Some(ref body) = body_bytes {
        request = request.body(body.clone());
    }

    let start = Instant::now();
    match request.send().await {
        Ok(resp) => {
            let latency_ms = start.elapsed().as_millis() as i64;
            let latency_ms = normalize_latency(latency_ms);
            let status = resp.status().as_u16() as i32;
            let body = resp.text().await.unwrap_or_default();
            ReplayResult {
                index,
                status: Some(status),
                latency_ms,
                body: Some(body),
                error: None,
            }
        }
        Err(e) => wrap_error(index, &e.to_string(), start.elapsed().as_millis() as i64),
    }
}

pub fn fingerprint(entry: &LogEntry) -> String {
    let mut hasher = Sha256::new();
    hasher.update(entry.method.as_bytes());
    hasher.update(entry.path.as_bytes());
    hasher.update(entry.body.as_bytes());

    let mut keys: Vec<&String> = entry.headers.keys().collect();
    keys.sort();

    for k in keys {
        let mut values = entry.headers[k].clone();
        values.sort();
        for v in &values {
            hasher.update(k.as_bytes());
            hasher.update(v.as_bytes());
        }
    }

    let result = hasher.finalize();
    hex::encode(result)[..16].to_string()
}

pub fn compare_responses_deterministic(
    responses: &HashMap<String, ReplayResult>,
    targets: &[String],
    volatile_config: Option<&VolatileConfig>,
    show_volatile_diffs: bool,
) -> Option<ResponseDiff> {
    if targets.len() < 2 {
        return None;
    }

    let baseline = &targets[0];
    let base = responses.get(baseline)?;

    let mut diff = ResponseDiff {
        status_mismatch: false,
        status_codes: Some(HashMap::new()),
        body_mismatch: false,
        body_diffs: Some(HashMap::new()),
        latency_diff: Some(HashMap::new()),
        volatile_only: false,
        ignored_fields: None,
    };

    let base_body = base.body.as_deref().unwrap_or("");

    for target in targets {
        let r = match responses.get(target) {
            Some(r) => r,
            None => continue,
        };

        if let Some(status) = r.status {
            if let Some(ref mut codes) = diff.status_codes {
                codes.insert(target.clone(), status);
            }
            if let Some(base_status) = base.status {
                if status != base_status {
                    diff.status_mismatch = true;
                }
            }
        }

        if let Some(ref mut latency) = diff.latency_diff {
            latency.insert(target.clone(), r.latency_ms);
        }
    }

    let mut volatile_only_flag = true;

    for target in &targets[1..] {
        let r = match responses.get(target) {
            Some(r) => r,
            None => continue,
        };
        let body = r.body.as_deref().unwrap_or("");

        if let Some(vc) = volatile_config {
            match detailed_compare(base_body, body, vc) {
                Ok(d) => {
                    if d.stable_fields_diff {
                        diff.body_mismatch = true;
                        volatile_only_flag = false;
                        if let Some(ref mut diffs) = diff.body_diffs {
                            diffs.insert(target.clone(), truncate(body, 200));
                        }
                    } else if d.volatile_only {
                        diff.body_mismatch = true;
                        if let Some(ref mut diffs) = diff.body_diffs {
                            diffs.insert(target.clone(), "<volatile-only>".to_string());
                        }
                    }
                }
                Err(_) => {
                    diff.body_mismatch = true;
                    volatile_only_flag = false;
                    if let Some(ref mut diffs) = diff.body_diffs {
                        diffs.insert(target.clone(), truncate(body, 200));
                    }
                }
            }
        } else if base_body != body {
            diff.body_mismatch = true;
            volatile_only_flag = false;
            if let Some(ref mut diffs) = diff.body_diffs {
                diffs.insert(target.clone(), truncate(body, 200));
            }
        }
    }

    if diff.body_mismatch {
        if let Some(ref mut diffs) = diff.body_diffs {
            diffs.insert(baseline.clone(), truncate(base_body, 200));
        }
    }

    diff.volatile_only = volatile_only_flag && diff.body_mismatch;

    if (!diff.status_mismatch && !diff.body_mismatch)
        || (diff.volatile_only && !show_volatile_diffs)
    {
        return None;
    }

    Some(diff)
}

pub fn has_diffs(results: &[MultiEnvResult]) -> bool {
    results.iter().any(|r| r.diff.is_some())
}

pub fn truncate(s: &str, max: usize) -> String {
    if s.len() <= max {
        s.to_string()
    } else {
        format!("{}...", &s[..max])
    }
}

fn normalize_latency(ms: i64) -> i64 {
    (ms / LATENCY_BUCKET_MS) * LATENCY_BUCKET_MS
}

fn wrap_error(index: i32, err: &str, latency: i64) -> ReplayResult {
    ReplayResult {
        index,
        status: None,
        latency_ms: normalize_latency(latency),
        body: None,
        error: Some(err.to_string()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashMap;

    fn entry(method: &str, path: &str) -> LogEntry {
        LogEntry {
            method: method.to_string(),
            path: path.to_string(),
            headers: HashMap::new(),
            body: String::new(),
            status: 0,
            response_headers: HashMap::new(),
            response_body: String::new(),
            timestamp: Default::default(),
            latency_ms: 0,
        }
    }

    fn default_args(targets: Vec<String>) -> CliArgs {
        CliArgs {
            input_file: String::new(),
            concurrency: 1,
            timeout: 5000,
            delay: 0,
            limit: 0,
            filter_method: String::new(),
            filter_path: String::new(),
            dry_run: false,
            summary_only: false,
            output_json: false,
            compare: false,
            rate_limit: 0,
            progress_bar: false,
            auth_header: String::new(),
            headers: vec![],
            html_report: String::new(),
            parse_nginx: String::new(),
            nginx_format: "combined".to_string(),
            ignore_volatile: false,
            ignore_fields: vec![],
            ignore_patterns: vec![],
            show_volatile_diffs: false,
            listen_addr: ":8080".to_string(),
            upstream: String::new(),
            capture_out: "captured.json".to_string(),
            capture_mode: false,
            capture_stream: false,
            tls_cert: String::new(),
            tls_key: String::new(),
            rules_file: String::new(),
            baseline_file: String::new(),
            cloud_upload: false,
            cloud_url: String::new(),
            cloud_api_key: String::new(),
            cloud_env: "default".to_string(),
            cloud_label_args: vec![],
            cloud_labels: HashMap::new(),
            targets,
        }
    }

    #[tokio::test]
    async fn test_run_single_target_success() {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();

        tokio::spawn(async move {
            let app = axum::Router::new().route(
                "/",
                axum::routing::get(|| async { axum::Json(serde_json::json!({"success": true})) }),
            );
            axum::serve(listener, app).await.unwrap();
        });
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;

        let entries = vec![entry("GET", "/")];
        let args = default_args(vec![addr.to_string()]);
        let results = run(&entries, &args).await;

        assert_eq!(results.len(), 1);
        let status = results[0].responses[&addr.to_string()].status;
        assert_eq!(status, Some(200));
    }

    #[tokio::test]
    async fn test_run_multiple_targets_comparison() {
        let listener1 = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr1 = listener1.local_addr().unwrap();
        let listener2 = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr2 = listener2.local_addr().unwrap();

        tokio::spawn(async move {
            let app = axum::Router::new().route(
                "/",
                axum::routing::get(|| async { axum::Json(serde_json::json!({"version": "v1"})) }),
            );
            axum::serve(listener1, app).await.unwrap();
        });
        tokio::spawn(async move {
            let app = axum::Router::new().route(
                "/",
                axum::routing::get(|| async { axum::Json(serde_json::json!({"version": "v2"})) }),
            );
            axum::serve(listener2, app).await.unwrap();
        });
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;

        let entries = vec![entry("GET", "/")];
        let mut args = default_args(vec![addr1.to_string(), addr2.to_string()]);
        args.concurrency = 2;
        args.compare = true;

        let results = run(&entries, &args).await;
        assert_eq!(results.len(), 1);
        assert!(results[0].diff.is_some());
        assert!(results[0].diff.as_ref().unwrap().body_mismatch);
    }

    #[tokio::test]
    async fn test_run_delay_between_requests() {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();

        tokio::spawn(async move {
            let app = axum::Router::new().route("/", axum::routing::get(|| async { "ok" }));
            axum::serve(listener, app).await.unwrap();
        });
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;

        let entries = vec![entry("GET", "/"), entry("GET", "/")];
        let mut args = default_args(vec![addr.to_string()]);
        args.delay = 200;

        let start = std::time::Instant::now();
        run(&entries, &args).await;
        assert!(start.elapsed() >= std::time::Duration::from_millis(200));
    }

    #[tokio::test]
    async fn test_replay_single_success() {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();

        tokio::spawn(async move {
            let app = axum::Router::new().route(
                "/",
                axum::routing::get(|| async { axum::Json(serde_json::json!({"ok": true})) }),
            );
            axum::serve(listener, app).await.unwrap();
        });
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;

        let e = entry("GET", "/");
        let client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(5))
            .build()
            .unwrap();
        let res = replay_single(0, &e, &client, &addr.to_string(), "", "", "", &[]).await;
        assert_eq!(res.status, Some(200));
    }

    #[tokio::test]
    async fn test_replay_single_body_sent() {
        use std::sync::{Arc, Mutex};

        let body_received = Arc::new(Mutex::new(String::new()));
        let body_clone = body_received.clone();

        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();

        tokio::spawn(async move {
            let app = axum::Router::new().route(
                "/",
                axum::routing::post(move |body: String| {
                    let bc = body_clone.clone();
                    async move {
                        *bc.lock().unwrap() = body;
                        "ok"
                    }
                }),
            );
            axum::serve(listener, app).await.unwrap();
        });
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;

        let payload = r#"{"a":1}"#;
        let encoded = base64::engine::general_purpose::STANDARD.encode(payload.as_bytes());
        let mut e = entry("POST", "/");
        e.body = encoded;

        let client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(5))
            .build()
            .unwrap();
        replay_single(0, &e, &client, &addr.to_string(), "", "", "", &[]).await;

        let received = body_received.lock().unwrap().clone();
        assert_eq!(received, payload);
    }

    #[test]
    fn test_fingerprint_deterministic() {
        let e = entry("POST", "/x");
        let f1 = fingerprint(&e);
        let f2 = fingerprint(&e);
        assert_eq!(f1, f2);
        assert_eq!(f1.len(), 16);
    }

    #[test]
    fn test_fingerprint_differs_for_different_entries() {
        let e1 = entry("GET", "/a");
        let e2 = entry("POST", "/b");
        assert_ne!(fingerprint(&e1), fingerprint(&e2));
    }

    #[test]
    fn test_compare_responses_no_diff() {
        let mut responses = HashMap::new();
        responses.insert(
            "target1".to_string(),
            ReplayResult {
                index: 0,
                status: Some(200),
                latency_ms: 10,
                body: Some(r#"{"ok":true}"#.to_string()),
                error: None,
            },
        );
        responses.insert(
            "target2".to_string(),
            ReplayResult {
                index: 0,
                status: Some(200),
                latency_ms: 12,
                body: Some(r#"{"ok":true}"#.to_string()),
                error: None,
            },
        );

        let targets = vec!["target1".to_string(), "target2".to_string()];
        let diff = compare_responses_deterministic(&responses, &targets, None, false);
        assert!(diff.is_none());
    }

    #[test]
    fn test_compare_responses_body_mismatch() {
        let mut responses = HashMap::new();
        responses.insert(
            "target1".to_string(),
            ReplayResult {
                index: 0,
                status: Some(200),
                latency_ms: 10,
                body: Some(r#"{"version":"v1"}"#.to_string()),
                error: None,
            },
        );
        responses.insert(
            "target2".to_string(),
            ReplayResult {
                index: 0,
                status: Some(200),
                latency_ms: 12,
                body: Some(r#"{"version":"v2"}"#.to_string()),
                error: None,
            },
        );

        let targets = vec!["target1".to_string(), "target2".to_string()];
        let diff = compare_responses_deterministic(&responses, &targets, None, false);
        assert!(diff.is_some());
        assert!(diff.unwrap().body_mismatch);
    }

    #[test]
    fn test_compare_responses_status_mismatch() {
        let mut responses = HashMap::new();
        responses.insert(
            "target1".to_string(),
            ReplayResult {
                index: 0,
                status: Some(200),
                latency_ms: 10,
                body: Some("ok".to_string()),
                error: None,
            },
        );
        responses.insert(
            "target2".to_string(),
            ReplayResult {
                index: 0,
                status: Some(500),
                latency_ms: 12,
                body: Some("ok".to_string()),
                error: None,
            },
        );

        let targets = vec!["target1".to_string(), "target2".to_string()];
        let diff = compare_responses_deterministic(&responses, &targets, None, false);
        assert!(diff.is_some());
        assert!(diff.unwrap().status_mismatch);
    }

    #[test]
    fn test_compare_responses_single_target() {
        let mut responses = HashMap::new();
        responses.insert(
            "target1".to_string(),
            ReplayResult {
                index: 0,
                status: Some(200),
                latency_ms: 10,
                body: Some("ok".to_string()),
                error: None,
            },
        );

        let targets = vec!["target1".to_string()];
        let diff = compare_responses_deterministic(&responses, &targets, None, false);
        assert!(diff.is_none());
    }

    #[test]
    fn test_has_diffs_true() {
        let results = vec![MultiEnvResult {
            index: 0,
            request: entry("GET", "/"),
            request_id: "abc".to_string(),
            responses: HashMap::new(),
            diff: Some(ResponseDiff {
                status_mismatch: true,
                status_codes: None,
                body_mismatch: false,
                body_diffs: None,
                latency_diff: None,
                volatile_only: false,
                ignored_fields: None,
            }),
        }];
        assert!(has_diffs(&results));
    }

    #[test]
    fn test_has_diffs_false() {
        let results = vec![MultiEnvResult {
            index: 0,
            request: entry("GET", "/"),
            request_id: "abc".to_string(),
            responses: HashMap::new(),
            diff: None,
        }];
        assert!(!has_diffs(&results));
    }

    #[test]
    fn test_truncate_short_string() {
        assert_eq!(truncate("hello", 10), "hello");
    }

    #[test]
    fn test_truncate_long_string() {
        let s = "a".repeat(300);
        let result = truncate(&s, 200);
        assert!(result.ends_with("..."));
        assert_eq!(result.len(), 203); // 200 + "..."
    }

    #[test]
    fn test_normalize_latency() {
        assert_eq!(normalize_latency(0), 0);
        assert_eq!(normalize_latency(3), 0);
        assert_eq!(normalize_latency(5), 5);
        assert_eq!(normalize_latency(7), 5);
        assert_eq!(normalize_latency(10), 10);
    }
}
