use std::collections::HashMap;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LogEntry {
    pub method: String,
    pub path: String,
    #[serde(default)]
    pub headers: HashMap<String, Vec<String>>,
    #[serde(default)]
    pub body: String,
    #[serde(default)]
    pub status: i32,
    #[serde(default)]
    pub response_headers: HashMap<String, Vec<String>>,
    #[serde(default)]
    pub response_body: String,
    #[serde(default)]
    pub timestamp: DateTime<Utc>,
    #[serde(default)]
    pub latency_ms: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ReplayResult {
    pub index: i32,
    pub status: Option<i32>,
    pub latency_ms: i64,
    pub error: Option<String>,
    pub body: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MultiEnvResult {
    pub index: i32,
    pub request: LogEntry,
    pub responses: HashMap<String, ReplayResult>,
    pub request_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub diff: Option<ResponseDiff>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResponseDiff {
    pub status_mismatch: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub status_codes: Option<HashMap<String, i32>>,
    pub body_mismatch: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub body_diffs: Option<HashMap<String, String>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub latency_diff: Option<HashMap<String, i64>>,
    pub volatile_only: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub ignored_fields: Option<Vec<String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Summary {
    pub total_requests: i32,
    pub succeeded: i32,
    pub failed: i32,
    pub latency: LatencyStats,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub by_target: Option<HashMap<String, TargetStats>>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct LatencyStats {
    pub p50: i64,
    pub p90: i64,
    pub p95: i64,
    pub p99: i64,
    pub min: i64,
    pub max: i64,
    pub avg: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TargetStats {
    pub succeeded: i32,
    pub failed: i32,
    pub latency: LatencyStats,
}

#[derive(Debug, Clone)]
pub struct AggregatedStats {
    pub total_requests: i32,
    pub succeeded: i32,
    pub failed: i32,
    pub latencies: Vec<i64>,
    pub target_stats: HashMap<String, TargetStats>,
}
