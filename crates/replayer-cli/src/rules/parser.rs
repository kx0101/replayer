use std::path::Path;

use super::types::{ReplayRunData, RulesConfig};

pub fn parse_rules_file(path: &str) -> anyhow::Result<RulesConfig> {
    let data = read_file_safe(path)?;

    let config: RulesConfig = serde_yaml::from_slice(&data)
        .map_err(|e| anyhow::anyhow!("failed to parse rules YAML: {}", e))?;

    validate_rules(&config)?;

    Ok(config)
}

pub fn load_baseline_file(path: &str) -> anyhow::Result<ReplayRunData> {
    let data =
        read_file_safe(path).map_err(|e| anyhow::anyhow!("failed to read baseline file: {}", e))?;

    #[derive(serde::Deserialize)]
    struct Baseline {
        results: Vec<replayer_core::models::MultiEnvResult>,
        summary: replayer_core::models::Summary,
    }

    let baseline: Baseline = serde_json::from_slice(&data)
        .map_err(|e| anyhow::anyhow!("failed to parse baseline JSON: {}", e))?;

    Ok(ReplayRunData {
        results: baseline.results,
        summary: baseline.summary,
    })
}

fn validate_rules(config: &RulesConfig) -> anyhow::Result<()> {
    let rules = match &config.rules {
        Some(r) => r,
        None => return Ok(()),
    };

    if let Some(ref latency) = rules.latency {
        validate_latency_rule(latency).map_err(|e| {
            anyhow::anyhow!("invalid rules configuration: global latency rule: {}", e)
        })?;
    }

    if let Some(ref endpoints) = rules.endpoint_rules {
        for (i, endpoint) in endpoints.iter().enumerate() {
            if endpoint.path.is_empty() {
                anyhow::bail!(
                    "invalid rules configuration: endpoint_rules[{}]: path is required",
                    i
                );
            }

            if let Some(ref latency) = endpoint.latency {
                validate_latency_rule(latency).map_err(|e| {
                    anyhow::anyhow!(
                        "invalid rules configuration: endpoint_rules[{}].latency: {}",
                        i,
                        e
                    )
                })?;
            }
        }
    }

    Ok(())
}

fn validate_latency_rule(rule: &super::types::LatencyRule) -> anyhow::Result<()> {
    let valid_metrics = ["p50", "p90", "p95", "p99", "avg", "max", "min"];

    if !valid_metrics.contains(&rule.metric.as_str()) {
        anyhow::bail!(
            "invalid metric '{}', must be one of: p50, p90, p95, p99, avg, max, min",
            rule.metric
        );
    }

    if rule.regression_percent < 0.0 {
        anyhow::bail!(
            "regression_percent cannot be negative: {:.2}",
            rule.regression_percent
        );
    }

    Ok(())
}

fn read_file_safe(path: &str) -> anyhow::Result<Vec<u8>> {
    if path.is_empty() {
        anyhow::bail!("path cannot be empty");
    }

    let clean = Path::new(path)
        .canonicalize()
        .unwrap_or_else(|_| Path::new(path).to_path_buf());

    if clean == Path::new(std::path::MAIN_SEPARATOR_STR) {
        anyhow::bail!("invalid path");
    }

    Ok(std::fs::read(path)?)
}

#[cfg(test)]
mod tests {
    use super::super::types::*;
    use super::*;

    fn write_rules_file(dir: &tempfile::TempDir, content: &str) -> String {
        let path = dir.path().join("rules.yaml");
        std::fs::write(&path, content).unwrap();
        path.to_str().unwrap().to_string()
    }

    #[test]
    fn test_parse_rules_file_valid_config() {
        let yaml = r#"rules:
  status_mismatch:
    max: 5
  body_diff:
    allowed: false
  latency:
    metric: p95
    regression_percent: 20.0
"#;
        let dir = tempfile::tempdir().unwrap();
        let path = write_rules_file(&dir, yaml);

        let config = parse_rules_file(&path).unwrap();
        let rules = config.rules.unwrap();

        let sm = rules.status_mismatch.unwrap();
        assert_eq!(sm.max, 5);

        let bd = rules.body_diff.unwrap();
        assert!(!bd.allowed);

        let lat = rules.latency.unwrap();
        assert_eq!(lat.metric, "p95");
        assert!((lat.regression_percent - 20.0).abs() < f64::EPSILON);
    }

    #[test]
    fn test_parse_rules_file_with_endpoint_rules() {
        let yaml = r#"rules:
  endpoint_rules:
    - path: /api/users
      method: GET
      latency:
        metric: p99
        regression_percent: 15.0
      status_mismatch:
        max: 2
    - path: /api/orders
      latency:
        metric: p90
        regression_percent: 25.0
"#;
        let dir = tempfile::tempdir().unwrap();
        let path = write_rules_file(&dir, yaml);

        let config = parse_rules_file(&path).unwrap();
        let rules = config.rules.unwrap();
        let endpoints = rules.endpoint_rules.unwrap();

        assert_eq!(endpoints.len(), 2);

        assert_eq!(endpoints[0].path, "/api/users");
        assert_eq!(endpoints[0].method.as_deref(), Some("GET"));
        assert_eq!(endpoints[0].latency.as_ref().unwrap().metric, "p99");
        assert_eq!(endpoints[0].status_mismatch.as_ref().unwrap().max, 2);

        assert_eq!(endpoints[1].path, "/api/orders");
        assert!(endpoints[1].method.is_none());
    }

    #[test]
    fn test_parse_rules_file_with_body_diff_ignore() {
        let yaml = r#"rules:
  body_diff:
    allowed: false
    ignore:
      - "*.timestamp"
      - "metadata.*"
      - "exact.field.name"
"#;
        let dir = tempfile::tempdir().unwrap();
        let path = write_rules_file(&dir, yaml);

        let config = parse_rules_file(&path).unwrap();
        let rules = config.rules.unwrap();
        let bd = rules.body_diff.unwrap();

        assert_eq!(bd.ignore.len(), 3);
        assert_eq!(bd.ignore[0], "*.timestamp");
        assert_eq!(bd.ignore[1], "metadata.*");
        assert_eq!(bd.ignore[2], "exact.field.name");
    }

    #[test]
    fn test_parse_rules_file_not_found() {
        let result = parse_rules_file("nonexistent_rules_12345.yaml");
        assert!(result.is_err());
    }

    #[test]
    fn test_parse_rules_file_invalid_yaml() {
        let yaml = r#"rules:
  status_mismatch:
    max: invalid_number
  - this is broken yaml
"#;
        let dir = tempfile::tempdir().unwrap();
        let path = write_rules_file(&dir, yaml);

        let result = parse_rules_file(&path);
        assert!(result.is_err());
    }

    #[test]
    fn test_parse_rules_file_invalid_latency_metric() {
        let yaml = r#"rules:
  latency:
    metric: p85
    regression_percent: 20.0
"#;
        let dir = tempfile::tempdir().unwrap();
        let path = write_rules_file(&dir, yaml);

        let result = parse_rules_file(&path);
        assert!(result.is_err());
    }

    #[test]
    fn test_parse_rules_file_negative_regression_percent() {
        let yaml = r#"rules:
  latency:
    metric: p95
    regression_percent: -10.0
"#;
        let dir = tempfile::tempdir().unwrap();
        let path = write_rules_file(&dir, yaml);

        let result = parse_rules_file(&path);
        assert!(result.is_err());
    }

    #[test]
    fn test_parse_rules_file_endpoint_missing_path() {
        let yaml = r#"rules:
  endpoint_rules:
    - method: GET
      latency:
        metric: p95
        regression_percent: 20.0
"#;
        let dir = tempfile::tempdir().unwrap();
        let path = write_rules_file(&dir, yaml);

        let result = parse_rules_file(&path);
        assert!(result.is_err());
    }

    #[test]
    fn test_validate_latency_rule_all_valid_metrics() {
        let valid_metrics = ["p50", "p90", "p95", "p99", "avg", "max", "min"];
        for metric in valid_metrics {
            let rule = LatencyRule {
                metric: metric.to_string(),
                regression_percent: 20.0,
            };
            assert!(
                validate_latency_rule(&rule).is_ok(),
                "expected metric '{}' to be valid",
                metric
            );
        }
    }

    #[test]
    fn test_validate_latency_rule_invalid_metrics() {
        let invalid_metrics = ["p85", "median", "average", "P95", ""];
        for metric in invalid_metrics {
            let rule = LatencyRule {
                metric: metric.to_string(),
                regression_percent: 20.0,
            };
            assert!(
                validate_latency_rule(&rule).is_err(),
                "expected metric '{}' to be invalid",
                metric
            );
        }
    }

    #[test]
    fn test_validate_latency_rule_zero_regression_percent() {
        let rule = LatencyRule {
            metric: "p95".to_string(),
            regression_percent: 0.0,
        };
        assert!(validate_latency_rule(&rule).is_ok());
    }

    #[test]
    fn test_validate_rules_complex_valid() {
        let config = RulesConfig {
            rules: Some(Rules {
                status_mismatch: Some(StatusMismatchRule { max: 5 }),
                body_diff: Some(BodyDiffRule {
                    allowed: false,
                    ignore: vec![],
                }),
                latency: Some(LatencyRule {
                    metric: "p95".to_string(),
                    regression_percent: 20.0,
                }),
                endpoint_rules: Some(vec![
                    EndpointRule {
                        path: "/api/users".to_string(),
                        method: Some("GET".to_string()),
                        latency: Some(LatencyRule {
                            metric: "p99".to_string(),
                            regression_percent: 15.0,
                        }),
                        status_mismatch: None,
                    },
                    EndpointRule {
                        path: "/api/orders".to_string(),
                        method: None,
                        latency: Some(LatencyRule {
                            metric: "avg".to_string(),
                            regression_percent: 30.0,
                        }),
                        status_mismatch: None,
                    },
                ]),
            }),
        };
        assert!(validate_rules(&config).is_ok());
    }

    #[test]
    fn test_validate_rules_endpoint_missing_path() {
        let config = RulesConfig {
            rules: Some(Rules {
                status_mismatch: None,
                body_diff: None,
                latency: None,
                endpoint_rules: Some(vec![
                    EndpointRule {
                        path: "/api/users".to_string(),
                        method: Some("GET".to_string()),
                        latency: Some(LatencyRule {
                            metric: "p99".to_string(),
                            regression_percent: 15.0,
                        }),
                        status_mismatch: None,
                    },
                    EndpointRule {
                        path: String::new(),
                        method: Some("POST".to_string()),
                        latency: Some(LatencyRule {
                            metric: "p95".to_string(),
                            regression_percent: 20.0,
                        }),
                        status_mismatch: None,
                    },
                ]),
            }),
        };
        assert!(validate_rules(&config).is_err());
    }

    #[test]
    fn test_parse_rules_file_empty_rules() {
        let yaml = "rules: {}";
        let dir = tempfile::tempdir().unwrap();
        let path = write_rules_file(&dir, yaml);

        let config = parse_rules_file(&path).unwrap();
        assert!(config.rules.is_some());
    }
}
