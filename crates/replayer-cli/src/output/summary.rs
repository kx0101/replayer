use std::collections::HashMap;

use replayer_core::latency::calculate_latency_stats;
use replayer_core::models::{AggregatedStats, LatencyStats, MultiEnvResult, Summary, TargetStats};

pub const RESET: &str = "\x1b[0m";
pub const RED: &str = "\x1b[31m";
pub const GREEN: &str = "\x1b[32m";
pub const YELLOW: &str = "\x1b[33m";
pub const CYAN: &str = "\x1b[36m";
pub const BOLD: &str = "\x1b[1m";

pub fn print_summary(results: &[MultiEnvResult], compare: bool) {
    println!("{}==== Summary ===={}", BOLD, RESET);

    let agg = aggregate_results(results);
    let mut diff_count = 0;

    if compare {
        for r in results {
            if r.diff.is_some() {
                diff_count += 1;
            }
        }
    }

    print_results(results, diff_count, compare, &agg);
}

pub fn aggregate_results(results: &[MultiEnvResult]) -> AggregatedStats {
    let mut target_succeeded: HashMap<String, i32> = HashMap::new();
    let mut target_failed: HashMap<String, i32> = HashMap::new();
    let mut target_latencies: HashMap<String, Vec<i64>> = HashMap::new();

    if let Some(first) = results.first() {
        for target in first.responses.keys() {
            target_succeeded.insert(target.clone(), 0);
            target_failed.insert(target.clone(), 0);
            target_latencies.insert(target.clone(), Vec::new());
        }
    }

    let mut total_requests = 0i32;
    let mut succeeded = 0i32;
    let mut failed = 0i32;
    let mut latencies = Vec::new();

    for r in results {
        for (target, replay) in &r.responses {
            total_requests += 1;

            if replay.status.is_some_and(|s| s < 400) {
                succeeded += 1;
                *target_succeeded.entry(target.clone()).or_insert(0) += 1;
            } else {
                failed += 1;
                *target_failed.entry(target.clone()).or_insert(0) += 1;
            }

            latencies.push(replay.latency_ms);
            target_latencies
                .entry(target.clone())
                .or_default()
                .push(replay.latency_ms);
        }
    }

    let mut target_stats = HashMap::new();
    for (target, lats) in &target_latencies {
        target_stats.insert(
            target.clone(),
            TargetStats {
                succeeded: *target_succeeded.get(target).unwrap_or(&0),
                failed: *target_failed.get(target).unwrap_or(&0),
                latency: calculate_latency_stats(lats),
            },
        );
    }

    AggregatedStats {
        total_requests,
        succeeded,
        failed,
        latencies,
        target_stats,
    }
}

pub fn print_json_output(results: &[MultiEnvResult]) {
    let summary = generate_summary(results);
    let output = serde_json::json!({
        "results": results,
        "summary": summary,
    });

    match serde_json::to_string_pretty(&output) {
        Ok(json) => println!("{}", json),
        Err(e) => eprintln!("Error encoding JSON: {}", e),
    }
}

pub fn convert_to_summary(agg: &AggregatedStats) -> Summary {
    Summary {
        total_requests: agg.total_requests,
        succeeded: agg.succeeded,
        failed: agg.failed,
        latency: calculate_latency_stats(&agg.latencies),
        by_target: Some(agg.target_stats.clone()),
    }
}

pub fn format_status(status: Option<i32>) -> (String, &'static str) {
    match status {
        None => ("ERR".to_string(), RED),
        Some(s) if s < 400 => (s.to_string(), GREEN),
        Some(s) if s < 500 => (s.to_string(), YELLOW),
        Some(s) => (s.to_string(), RED),
    }
}

fn generate_summary(results: &[MultiEnvResult]) -> Summary {
    let agg = aggregate_results(results);
    convert_to_summary(&agg)
}

fn print_results(
    results: &[MultiEnvResult],
    diff_count: i32,
    compare: bool,
    agg: &AggregatedStats,
) {
    let mut sorted_latencies = agg.latencies.clone();
    sorted_latencies.sort_unstable();
    let overall_latency = calculate_latency_stats(&sorted_latencies);

    println!(
        "Total Requests: {}\nSucceeded: {}{}{}\nFailed: {}{}{}",
        agg.total_requests, GREEN, agg.succeeded, RESET, RED, agg.failed, RESET
    );

    if compare && diff_count > 0 {
        println!("Differences: {}{}{}", YELLOW, diff_count, RESET);
    }

    println!("\nLatency (ms):");
    print_latency_stats(&overall_latency);

    if agg.target_stats.len() > 1 {
        println!("\nPer-Target Statistics:");

        for (target, ts) in &agg.target_stats {
            println!(
                "\n{}{}:{}\n  Succeeded: {}\n  Failed: {}\n  Latency:",
                CYAN, target, RESET, ts.succeeded, ts.failed
            );
            print_latency_stats(&ts.latency);
        }
    }

    if results.is_empty() {
        return;
    }

    for r in results {
        for (target, replay) in &r.responses {
            let (status_str, color) = format_status(replay.status);
            let err_msg = replay
                .error
                .as_ref()
                .map(|e| format!(" ({})", e))
                .unwrap_or_default();

            println!(
                "[{}][{}] {}{}{} -> {}ms{}",
                r.index, target, color, status_str, RESET, replay.latency_ms, err_msg
            );
        }

        if compare {
            if let Some(ref diff) = r.diff {
                print_diff(r.index, diff, &r.responses);
            }
        }
    }
}

fn print_latency_stats(stats: &LatencyStats) {
    println!(
        "  min: {}  avg: {}  p50: {}  p90: {}  p95: {}  p99: {}  max: {}",
        stats.min, stats.avg, stats.p50, stats.p90, stats.p95, stats.p99, stats.max
    );
}

fn print_diff(
    index: i32,
    diff: &replayer_core::models::ResponseDiff,
    _responses: &HashMap<String, replayer_core::models::ReplayResult>,
) {
    let diff_type = if diff.volatile_only {
        " (volatile fields only)"
    } else {
        ""
    };

    println!(
        "{}  [DIFF] Request {}{}:{}",
        YELLOW, index, diff_type, RESET
    );

    if diff.status_mismatch {
        print!("    Status codes differ: ");
        if let Some(ref status_codes) = diff.status_codes {
            for (target, status) in status_codes {
                print!("{}={} ", target, status);
            }
        }
        println!();
    }

    if diff.body_mismatch {
        println!("    Response bodies differ");
        if let Some(ref body_diffs) = diff.body_diffs {
            for (target, body) in body_diffs {
                println!("      {}: {}", target, body);
            }
        }
    }

    if let Some(ref fields) = diff.ignored_fields {
        if !fields.is_empty() {
            print!("    {}Ignored fields:{} ", CYAN, RESET);
            if fields.len() <= 5 {
                println!("{:?}", fields);
            } else {
                println!("{:?} and {} more...", &fields[..5], fields.len() - 5);
            }
        }
    }

    if let Some(ref latency_diff) = diff.latency_diff {
        if latency_diff.len() > 1 {
            print!("    Latency: ");
            for (target, lat) in latency_diff {
                print!("{}={}ms ", target, lat);
            }
            println!();
        }
    }
}
