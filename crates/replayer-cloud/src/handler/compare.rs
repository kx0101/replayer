use std::collections::HashMap;

use axum::{extract::State, http::StatusCode, response::Response, Extension};
use uuid::Uuid;

use replayer_core::models::MultiEnvResult;

use crate::middleware::AuthUserId;
use crate::models::{ComparisonResult, LatencyDelta, Run};

use super::response::{respond_error, respond_json};
use super::AppState;

pub async fn compare_run(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    axum::extract::Path(id): axum::extract::Path<Uuid>,
) -> Response {
    let run = match state.store.get_run_for_user(user_id, id).await {
        Ok(Some(r)) => r,
        Ok(None) => return respond_error(StatusCode::NOT_FOUND, "run not found"),
        Err(e) => {
            tracing::error!("error getting run: {}", e);
            return respond_error(StatusCode::INTERNAL_SERVER_ERROR, "failed to get run");
        }
    };

    let baseline = match state
        .store
        .get_baseline_for_user(user_id, &run.environment)
        .await
    {
        Ok(Some(b)) => b,
        Ok(None) => return respond_error(StatusCode::NOT_FOUND, "no baseline set for environment"),
        Err(e) => {
            tracing::error!("error getting baseline: {}", e);
            return respond_error(StatusCode::INTERNAL_SERVER_ERROR, "failed to get baseline");
        }
    };

    let result = build_comparison(&run, &baseline);
    respond_json(StatusCode::OK, &result)
}

pub fn build_comparison(run: &Run, baseline: &Run) -> ComparisonResult {
    let mut result = ComparisonResult {
        run_id: run.id,
        baseline_id: baseline.id,
        run_summary: replayer_core::models::Summary {
            total_requests: run.total_requests,
            succeeded: run.succeeded,
            failed: run.failed,
            latency: run.latency_stats.clone(),
            by_target: Some(run.by_target.clone()),
        },
        baseline_summary: replayer_core::models::Summary {
            total_requests: baseline.total_requests,
            succeeded: baseline.succeeded,
            failed: baseline.failed,
            latency: baseline.latency_stats.clone(),
            by_target: Some(baseline.by_target.clone()),
        },
        diff_count: 0,
        latency_delta: HashMap::new(),
    };

    let baseline_by_req_id: HashMap<&str, &MultiEnvResult> = baseline
        .results
        .iter()
        .map(|r| (r.request_id.as_str(), r))
        .collect();

    for r in &run.results {
        if let Some(ref diff) = r.diff {
            if diff.status_mismatch || diff.body_mismatch {
                result.diff_count += 1;
                continue;
            }
        }
        if let Some(br) = baseline_by_req_id.get(r.request_id.as_str()) {
            if has_differences(r, br) {
                result.diff_count += 1;
            }
        }
    }

    for (target, run_stats) in &run.by_target {
        if let Some(base_stats) = baseline.by_target.get(target) {
            result.latency_delta.insert(
                target.clone(),
                LatencyDelta {
                    current: run_stats.latency.clone(),
                    baseline: base_stats.latency.clone(),
                    p50_change_pct: pct_change(base_stats.latency.p50, run_stats.latency.p50),
                    p90_change_pct: pct_change(base_stats.latency.p90, run_stats.latency.p90),
                    p95_change_pct: pct_change(base_stats.latency.p95, run_stats.latency.p95),
                    p99_change_pct: pct_change(base_stats.latency.p99, run_stats.latency.p99),
                    avg_change_pct: pct_change(base_stats.latency.avg, run_stats.latency.avg),
                },
            );
        }
    }

    result
}

fn has_differences(a: &MultiEnvResult, b: &MultiEnvResult) -> bool {
    for (target, a_resp) in &a.responses {
        if let Some(b_resp) = b.responses.get(target) {
            if let (Some(a_status), Some(b_status)) = (a_resp.status, b_resp.status) {
                if a_status != b_status {
                    return true;
                }
            }
        }
    }
    false
}

fn pct_change(base: i64, current: i64) -> f64 {
    if base == 0 {
        return 0.0;
    }
    (current - base) as f64 / base as f64 * 100.0
}

#[cfg(test)]
mod tests {
    use super::*;
    use replayer_core::models::{
        LatencyStats, LogEntry, MultiEnvResult, ReplayResult, TargetStats,
    };
    use std::collections::HashMap;

    #[test]
    fn test_pct_change_increase() {
        assert!((pct_change(10, 20) - 100.0).abs() < f64::EPSILON);
    }

    #[test]
    fn test_pct_change_decrease() {
        assert!((pct_change(20, 10) - (-50.0)).abs() < f64::EPSILON);
    }

    #[test]
    fn test_pct_change_zero_base() {
        assert!((pct_change(0, 10) - 0.0).abs() < f64::EPSILON);
    }

    #[test]
    fn test_pct_change_no_change() {
        assert!((pct_change(100, 100) - 0.0).abs() < f64::EPSILON);
    }

    fn make_log_entry() -> LogEntry {
        LogEntry {
            method: String::new(),
            path: String::new(),
            headers: HashMap::new(),
            body: String::new(),
            status: 0,
            response_headers: HashMap::new(),
            response_body: String::new(),
            timestamp: chrono::Utc::now(),
            latency_ms: 0,
        }
    }

    #[test]
    fn test_build_comparison_latency_deltas() {
        let run = Run {
            id: uuid::Uuid::new_v4(),
            user_id: None,
            environment: "staging".to_string(),
            targets: vec!["target1".to_string()],
            created_at: chrono::Utc::now(),
            total_requests: 10,
            succeeded: 9,
            failed: 1,
            latency_stats: LatencyStats {
                p50: 20,
                p90: 40,
                p95: 50,
                p99: 60,
                min: 0,
                max: 0,
                avg: 30,
            },
            by_target: HashMap::from([(
                "target1".to_string(),
                TargetStats {
                    succeeded: 9,
                    failed: 1,
                    latency: LatencyStats {
                        p50: 20,
                        p90: 40,
                        p95: 50,
                        p99: 60,
                        min: 0,
                        max: 0,
                        avg: 30,
                    },
                },
            )]),
            results: vec![],
            is_baseline: false,
            baseline_id: None,
            labels: None,
        };

        let baseline = Run {
            id: uuid::Uuid::new_v4(),
            user_id: None,
            environment: "staging".to_string(),
            targets: vec!["target1".to_string()],
            created_at: chrono::Utc::now(),
            total_requests: 10,
            succeeded: 10,
            failed: 0,
            latency_stats: LatencyStats {
                p50: 10,
                p90: 20,
                p95: 25,
                p99: 30,
                min: 0,
                max: 0,
                avg: 15,
            },
            by_target: HashMap::from([(
                "target1".to_string(),
                TargetStats {
                    succeeded: 10,
                    failed: 0,
                    latency: LatencyStats {
                        p50: 10,
                        p90: 20,
                        p95: 25,
                        p99: 30,
                        min: 0,
                        max: 0,
                        avg: 15,
                    },
                },
            )]),
            results: vec![],
            is_baseline: false,
            baseline_id: None,
            labels: None,
        };

        let result = build_comparison(&run, &baseline);

        let delta = result
            .latency_delta
            .get("target1")
            .expect("expected latency delta for target1");
        assert!(
            (delta.p50_change_pct - 100.0).abs() < f64::EPSILON,
            "expected P50 change 100%, got {}",
            delta.p50_change_pct
        );
        assert!(
            (delta.avg_change_pct - 100.0).abs() < f64::EPSILON,
            "expected Avg change 100%, got {}",
            delta.avg_change_pct
        );
    }

    #[test]
    fn test_build_comparison_diff_count_from_request_id_matching() {
        let run = Run {
            id: uuid::Uuid::new_v4(),
            user_id: None,
            environment: String::new(),
            targets: vec![],
            created_at: chrono::Utc::now(),
            total_requests: 0,
            succeeded: 0,
            failed: 0,
            latency_stats: LatencyStats::default(),
            by_target: HashMap::new(),
            results: vec![
                MultiEnvResult {
                    index: 0,
                    request: make_log_entry(),
                    responses: HashMap::from([(
                        "target1".to_string(),
                        ReplayResult {
                            index: 0,
                            status: Some(500),
                            latency_ms: 0,
                            error: None,
                            body: None,
                        },
                    )]),
                    request_id: "req-1".to_string(),
                    diff: None,
                },
                MultiEnvResult {
                    index: 1,
                    request: make_log_entry(),
                    responses: HashMap::from([(
                        "target1".to_string(),
                        ReplayResult {
                            index: 0,
                            status: Some(200),
                            latency_ms: 0,
                            error: None,
                            body: None,
                        },
                    )]),
                    request_id: "req-2".to_string(),
                    diff: None,
                },
            ],
            is_baseline: false,
            baseline_id: None,
            labels: None,
        };

        let baseline = Run {
            id: uuid::Uuid::new_v4(),
            user_id: None,
            environment: String::new(),
            targets: vec![],
            created_at: chrono::Utc::now(),
            total_requests: 0,
            succeeded: 0,
            failed: 0,
            latency_stats: LatencyStats::default(),
            by_target: HashMap::new(),
            results: vec![
                MultiEnvResult {
                    index: 0,
                    request: make_log_entry(),
                    responses: HashMap::from([(
                        "target1".to_string(),
                        ReplayResult {
                            index: 0,
                            status: Some(200),
                            latency_ms: 0,
                            error: None,
                            body: None,
                        },
                    )]),
                    request_id: "req-1".to_string(),
                    diff: None,
                },
                MultiEnvResult {
                    index: 1,
                    request: make_log_entry(),
                    responses: HashMap::from([(
                        "target1".to_string(),
                        ReplayResult {
                            index: 0,
                            status: Some(200),
                            latency_ms: 0,
                            error: None,
                            body: None,
                        },
                    )]),
                    request_id: "req-2".to_string(),
                    diff: None,
                },
            ],
            is_baseline: false,
            baseline_id: None,
            labels: None,
        };

        let result = build_comparison(&run, &baseline);
        assert_eq!(
            result.diff_count, 1,
            "expected 1 diff (req-1 status mismatch)"
        );
    }

    #[test]
    fn test_has_differences_status_mismatch() {
        let a = MultiEnvResult {
            index: 0,
            request: make_log_entry(),
            responses: HashMap::from([(
                "t".to_string(),
                ReplayResult {
                    index: 0,
                    status: Some(500),
                    latency_ms: 0,
                    error: None,
                    body: None,
                },
            )]),
            request_id: "r1".to_string(),
            diff: None,
        };
        let b = MultiEnvResult {
            index: 0,
            request: make_log_entry(),
            responses: HashMap::from([(
                "t".to_string(),
                ReplayResult {
                    index: 0,
                    status: Some(200),
                    latency_ms: 0,
                    error: None,
                    body: None,
                },
            )]),
            request_id: "r1".to_string(),
            diff: None,
        };
        assert!(has_differences(&a, &b));
    }

    #[test]
    fn test_has_differences_same_status() {
        let a = MultiEnvResult {
            index: 0,
            request: make_log_entry(),
            responses: HashMap::from([(
                "t".to_string(),
                ReplayResult {
                    index: 0,
                    status: Some(200),
                    latency_ms: 0,
                    error: None,
                    body: None,
                },
            )]),
            request_id: "r1".to_string(),
            diff: None,
        };
        let b = MultiEnvResult {
            index: 0,
            request: make_log_entry(),
            responses: HashMap::from([(
                "t".to_string(),
                ReplayResult {
                    index: 0,
                    status: Some(200),
                    latency_ms: 0,
                    error: None,
                    body: None,
                },
            )]),
            request_id: "r1".to_string(),
            diff: None,
        };
        assert!(!has_differences(&a, &b));
    }
}
