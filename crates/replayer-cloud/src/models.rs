use chrono::{DateTime, Utc};
use replayer_core::models::{LatencyStats, MultiEnvResult, Summary, TargetStats};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use uuid::Uuid;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Run {
    pub id: Uuid,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub user_id: Option<Uuid>,
    pub environment: String,
    pub targets: Vec<String>,
    pub created_at: DateTime<Utc>,
    pub total_requests: i32,
    pub succeeded: i32,
    pub failed: i32,
    pub latency_stats: LatencyStats,
    pub by_target: HashMap<String, TargetStats>,
    pub results: Vec<MultiEnvResult>,
    pub is_baseline: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub baseline_id: Option<Uuid>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub labels: Option<HashMap<String, String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RunListItem {
    pub id: Uuid,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub user_id: Option<Uuid>,
    pub environment: String,
    pub targets: Vec<String>,
    pub created_at: DateTime<Utc>,
    pub total_requests: i32,
    pub succeeded: i32,
    pub failed: i32,
    pub latency_stats: LatencyStats,
    pub by_target: HashMap<String, TargetStats>,
    pub is_baseline: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub baseline_id: Option<Uuid>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub labels: Option<HashMap<String, String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ComparisonResult {
    pub run_id: Uuid,
    pub baseline_id: Uuid,
    pub run_summary: Summary,
    pub baseline_summary: Summary,
    pub diff_count: i32,
    pub latency_delta: HashMap<String, LatencyDelta>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LatencyDelta {
    pub current: LatencyStats,
    pub baseline: LatencyStats,
    pub p50_change_pct: f64,
    pub p90_change_pct: f64,
    pub p95_change_pct: f64,
    pub p99_change_pct: f64,
    pub avg_change_pct: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct User {
    pub id: Uuid,
    pub email: String,
    #[serde(skip_serializing)]
    pub password_hash: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub verified_at: Option<DateTime<Utc>>,
    #[serde(skip)]
    pub verify_token: Option<String>,
    pub created_at: DateTime<Utc>,
    pub updated_at: DateTime<Utc>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct APIKey {
    pub id: Uuid,
    pub user_id: Uuid,
    #[serde(skip_serializing)]
    pub key_hash: String,
    pub key_prefix: String,
    pub name: String,
    pub created_at: DateTime<Utc>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_used_at: Option<DateTime<Utc>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub expires_at: Option<DateTime<Utc>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionData {
    pub user_id: Uuid,
    pub email: String,
    pub expires_at: DateTime<Utc>,
}
