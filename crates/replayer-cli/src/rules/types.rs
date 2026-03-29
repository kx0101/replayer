use std::collections::HashMap;

use serde::{Deserialize, Serialize};

use replayer_core::models::{MultiEnvResult, Summary};

#[derive(Debug, Clone, Deserialize)]
pub struct RulesConfig {
    pub rules: Option<Rules>,
}

#[derive(Debug, Clone, Deserialize)]
pub struct Rules {
    #[serde(default)]
    pub status_mismatch: Option<StatusMismatchRule>,
    #[serde(default)]
    pub body_diff: Option<BodyDiffRule>,
    #[serde(default)]
    pub latency: Option<LatencyRule>,
    #[serde(default)]
    pub endpoint_rules: Option<Vec<EndpointRule>>,
}

#[derive(Debug, Clone, Deserialize)]
pub struct StatusMismatchRule {
    pub max: i32,
}

#[derive(Debug, Clone, Deserialize)]
pub struct BodyDiffRule {
    pub allowed: bool,
    #[serde(default)]
    pub ignore: Vec<String>,
}

#[derive(Debug, Clone, Deserialize)]
pub struct LatencyRule {
    pub metric: String,
    pub regression_percent: f64,
}

#[derive(Debug, Clone, Deserialize)]
pub struct EndpointRule {
    pub path: String,
    #[serde(default)]
    pub method: Option<String>,
    #[serde(default)]
    pub latency: Option<LatencyRule>,
    #[serde(default)]
    pub status_mismatch: Option<StatusMismatchRule>,
}

#[derive(Debug, Clone, Serialize)]
pub struct RuleEvaluationResult {
    pub passed: bool,
    pub failures: Vec<RuleFailure>,
}

#[derive(Debug, Clone, Serialize)]
pub struct RuleFailure {
    pub rule: String,
    pub scope: String,
    pub message: String,
    pub details: HashMap<String, serde_json::Value>,
}

#[derive(Debug, Clone)]
pub struct ReplayRunData {
    pub results: Vec<MultiEnvResult>,
    pub summary: Summary,
}
