use std::collections::HashMap;

use replayer_core::latency::calculate_latency_stats;
use replayer_core::models::{LatencyStats, MultiEnvResult};

use super::types::{
    BodyDiffRule, EndpointRule, LatencyRule, ReplayRunData, RuleEvaluationResult, RuleFailure,
    RulesConfig, StatusMismatchRule,
};

pub fn evaluate_rules(
    config: &RulesConfig,
    current: &ReplayRunData,
    baseline: Option<&ReplayRunData>,
) -> RuleEvaluationResult {
    let mut result = RuleEvaluationResult {
        passed: true,
        failures: Vec::new(),
    };

    let rules = match &config.rules {
        Some(r) => r,
        None => return result,
    };

    if let Some(ref rule) = rules.status_mismatch {
        let failures = evaluate_status_mismatch_rule(rule, &current.results, "global");
        result.failures.extend(failures);
    }

    if let Some(ref rule) = rules.body_diff {
        let failures = evaluate_body_diff_rule(rule, &current.results, "global");
        result.failures.extend(failures);
    }

    if let Some(ref rule) = rules.latency {
        if let Some(baseline) = baseline {
            if let Some(failure) = evaluate_latency_rule(
                rule,
                &current.summary.latency,
                &baseline.summary.latency,
                "global",
            ) {
                result.failures.push(failure);
            }
        }
    }

    if let Some(ref endpoints) = rules.endpoint_rules {
        for endpoint_rule in endpoints {
            let failures = evaluate_endpoint_rule(endpoint_rule, current, baseline);
            result.failures.extend(failures);
        }
    }

    result
        .failures
        .sort_by(|a, b| a.scope.cmp(&b.scope).then_with(|| a.rule.cmp(&b.rule)));

    result.passed = result.failures.is_empty();

    result
}

fn evaluate_status_mismatch_rule(
    rule: &StatusMismatchRule,
    results: &[MultiEnvResult],
    scope: &str,
) -> Vec<RuleFailure> {
    let mut count = 0;
    let mut affected_requests = Vec::new();

    for result in results {
        if let Some(ref diff) = result.diff {
            if diff.status_mismatch {
                count += 1;
                affected_requests.push(result.index);
            }
        }
    }

    if count > rule.max {
        vec![RuleFailure {
            rule: "status_mismatch".to_string(),
            scope: scope.to_string(),
            message: format!(
                "Found {} status mismatches, maximum allowed is {}",
                count, rule.max
            ),
            details: HashMap::from([
                ("count".to_string(), serde_json::json!(count)),
                ("max_allowed".to_string(), serde_json::json!(rule.max)),
                (
                    "affected_requests".to_string(),
                    serde_json::json!(affected_requests),
                ),
            ]),
        }]
    } else {
        Vec::new()
    }
}

fn evaluate_body_diff_rule(
    rule: &BodyDiffRule,
    results: &[MultiEnvResult],
    scope: &str,
) -> Vec<RuleFailure> {
    if rule.allowed {
        return Vec::new();
    }

    let mut count = 0;
    let mut affected_requests = Vec::new();

    for result in results {
        if let Some(ref diff) = result.diff {
            if diff.body_mismatch {
                if diff.volatile_only {
                    continue;
                }

                if should_ignore_diff(diff, &rule.ignore) {
                    continue;
                }

                count += 1;
                affected_requests.push(result.index);
            }
        }
    }

    if count > 0 {
        vec![RuleFailure {
            rule: "body_diff".to_string(),
            scope: scope.to_string(),
            message: format!("Found {} body differences (body diffs not allowed)", count),
            details: HashMap::from([
                ("count".to_string(), serde_json::json!(count)),
                ("allowed".to_string(), serde_json::json!(false)),
                (
                    "affected_requests".to_string(),
                    serde_json::json!(affected_requests),
                ),
            ]),
        }]
    } else {
        Vec::new()
    }
}

fn should_ignore_diff(
    diff: &replayer_core::models::ResponseDiff,
    ignore_patterns: &[String],
) -> bool {
    if ignore_patterns.is_empty() {
        return false;
    }

    if let Some(ref ignored_fields) = diff.ignored_fields {
        for field in ignored_fields {
            for pattern in ignore_patterns {
                if match_pattern(field, pattern) {
                    return true;
                }
            }
        }
    }

    false
}

fn match_pattern(field: &str, pattern: &str) -> bool {
    if let Some(suffix) = pattern.strip_prefix("*.") {
        return field.ends_with(suffix);
    }

    if let Some(prefix) = pattern.strip_suffix(".*") {
        return field.starts_with(prefix);
    }

    field == pattern
}

fn evaluate_latency_rule(
    rule: &LatencyRule,
    current: &LatencyStats,
    baseline: &LatencyStats,
    scope: &str,
) -> Option<RuleFailure> {
    let current_value = get_latency_metric(current, &rule.metric);
    let baseline_value = get_latency_metric(baseline, &rule.metric);

    if baseline_value == 0 {
        return None;
    }

    let regression =
        ((current_value as f64 - baseline_value as f64) / baseline_value as f64) * 100.0;

    if regression > rule.regression_percent {
        Some(RuleFailure {
            rule: "latency".to_string(),
            scope: scope.to_string(),
            message: format!(
                "Latency regression of {:.2}% exceeds threshold of {:.2}% ({}: {}ms -> {}ms)",
                regression, rule.regression_percent, rule.metric, baseline_value, current_value
            ),
            details: HashMap::from([
                ("metric".to_string(), serde_json::json!(rule.metric)),
                ("baseline_ms".to_string(), serde_json::json!(baseline_value)),
                ("current_ms".to_string(), serde_json::json!(current_value)),
                (
                    "regression_percent".to_string(),
                    serde_json::json!(regression),
                ),
                (
                    "threshold_percent".to_string(),
                    serde_json::json!(rule.regression_percent),
                ),
            ]),
        })
    } else {
        None
    }
}

fn get_latency_metric(stats: &LatencyStats, metric: &str) -> i64 {
    match metric {
        "p50" => stats.p50,
        "p90" => stats.p90,
        "p95" => stats.p95,
        "p99" => stats.p99,
        "avg" => stats.avg,
        "max" => stats.max,
        "min" => stats.min,
        _ => 0,
    }
}

fn evaluate_endpoint_rule(
    rule: &EndpointRule,
    current: &ReplayRunData,
    baseline: Option<&ReplayRunData>,
) -> Vec<RuleFailure> {
    let method = rule.method.as_deref().unwrap_or("");
    let matching_results = filter_results_by_endpoint(&current.results, &rule.path, method);

    if matching_results.is_empty() {
        return Vec::new();
    }

    let scope = if method.is_empty() {
        format!("endpoint:{}", rule.path)
    } else {
        format!("endpoint:{} {}", method, rule.path)
    };

    let mut failures = Vec::new();

    if let Some(ref status_rule) = rule.status_mismatch {
        failures.extend(evaluate_status_mismatch_rule(
            status_rule,
            &matching_results,
            &scope,
        ));
    }

    if let Some(ref latency_rule) = rule.latency {
        if let Some(baseline) = baseline {
            let current_latency = calculate_endpoint_latency(&matching_results);
            let baseline_matching =
                filter_results_by_endpoint(&baseline.results, &rule.path, method);

            if !baseline_matching.is_empty() {
                let baseline_latency = calculate_endpoint_latency(&baseline_matching);
                if let Some(failure) =
                    evaluate_latency_rule(latency_rule, &current_latency, &baseline_latency, &scope)
                {
                    failures.push(failure);
                }
            }
        }
    }

    failures
}

fn filter_results_by_endpoint(
    results: &[MultiEnvResult],
    path: &str,
    method: &str,
) -> Vec<MultiEnvResult> {
    results
        .iter()
        .filter(|r| {
            if !r.request.path.starts_with(path) {
                return false;
            }
            if !method.is_empty() && r.request.method != method {
                return false;
            }
            true
        })
        .cloned()
        .collect()
}

fn calculate_endpoint_latency(results: &[MultiEnvResult]) -> LatencyStats {
    if results.is_empty() {
        return LatencyStats::default();
    }

    let latencies: Vec<i64> = results
        .iter()
        .flat_map(|r| r.responses.values().map(|resp| resp.latency_ms))
        .collect();

    calculate_latency_stats(&latencies)
}
