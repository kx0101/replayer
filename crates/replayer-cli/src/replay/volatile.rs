use std::collections::HashMap;

use regex::Regex;
use serde_json::Value;

pub struct VolatileConfig {
    pub ignore_fields: Vec<String>,
    pub ignore_patterns: Vec<Regex>,
}

pub struct VolatileDiff {
    pub has_diff: bool,
    pub volatile_only: bool,
    pub stable_fields_diff: bool,
    pub normalized_body1: String,
    pub normalized_body2: String,
    pub ignored_fields: Vec<String>,
}

pub fn default_volatile_config() -> VolatileConfig {
    VolatileConfig {
        ignore_fields: vec![
            "timestamp".to_string(),
            "createdAt".to_string(),
            "updatedAt".to_string(),
            "id".to_string(),
            "uuid".to_string(),
            "requestId".to_string(),
            "traceId".to_string(),
            "spanId".to_string(),
            "date".to_string(),
            "time".to_string(),
            "version".to_string(),
        ],
        ignore_patterns: vec![
            Regex::new(r"(?i).*_at$").unwrap(),
            Regex::new(r"(?i).*_id$").unwrap(),
            Regex::new(r"(?i).*timestamp.*").unwrap(),
            Regex::new(r"(?i).*uuid.*").unwrap(),
        ],
    }
}

pub fn config_from_flags(ignore_fields: &[String], ignore_patterns: &[String]) -> VolatileConfig {
    let mut config = default_volatile_config();

    if !ignore_fields.is_empty() {
        config.ignore_fields.extend(ignore_fields.iter().cloned());
    }

    for pattern in ignore_patterns {
        if let Ok(re) = Regex::new(pattern) {
            config.ignore_patterns.push(re);
        }
    }

    config
}

pub fn normalize_json(json_str: &str, config: &VolatileConfig) -> anyhow::Result<String> {
    let data: Value = match serde_json::from_str(json_str) {
        Ok(v) => v,
        Err(e) => return Err(anyhow::anyhow!("{}", e)),
    };

    let normalized = remove_volatile_fields(&data, config);

    match serde_json::to_string(&normalized) {
        Ok(s) => Ok(s),
        Err(e) => Err(anyhow::anyhow!("{}", e)),
    }
}

pub fn detailed_compare(
    body1: &str,
    body2: &str,
    config: &VolatileConfig,
) -> anyhow::Result<VolatileDiff> {
    let raw_diff = body1 != body2;

    let data1: Value = serde_json::from_str(body1)?;
    let data2: Value = serde_json::from_str(body2)?;

    let normalized1 = remove_volatile_fields(&data1, config);
    let normalized2 = remove_volatile_fields(&data2, config);

    let normalized_equal = normalized1 == normalized2;

    let nb1 = serde_json::to_string(&normalized1).unwrap_or_default();
    let nb2 = serde_json::to_string(&normalized2).unwrap_or_default();

    Ok(VolatileDiff {
        has_diff: raw_diff,
        volatile_only: raw_diff && normalized_equal,
        stable_fields_diff: !normalized_equal,
        normalized_body1: nb1,
        normalized_body2: nb2,
        ignored_fields: collect_ignored_fields(body1, body2, config),
    })
}

pub fn should_ignore_field(field_name: &str, config: &VolatileConfig) -> bool {
    for ignore in &config.ignore_fields {
        if field_name.eq_ignore_ascii_case(ignore) {
            return true;
        }
    }

    for pattern in &config.ignore_patterns {
        if pattern.is_match(field_name) {
            return true;
        }
    }

    false
}

pub fn collect_ignored_fields(body1: &str, body2: &str, config: &VolatileConfig) -> Vec<String> {
    let mut fields = Vec::new();
    let mut seen = HashMap::new();

    for body in [body1, body2] {
        if let Ok(data) = serde_json::from_str::<Value>(body) {
            collect_field_names(&data, "", config, &mut seen, &mut fields);
        }
    }

    fields
}

fn remove_volatile_fields(data: &Value, config: &VolatileConfig) -> Value {
    match data {
        Value::Object(map) => {
            let mut result = serde_json::Map::new();
            for (key, value) in map {
                if should_ignore_field(key, config) {
                    continue;
                }
                result.insert(key.clone(), remove_volatile_fields(value, config));
            }
            Value::Object(result)
        }
        Value::Array(arr) => Value::Array(
            arr.iter()
                .map(|item| remove_volatile_fields(item, config))
                .collect(),
        ),
        other => other.clone(),
    }
}

fn collect_field_names(
    data: &Value,
    prefix: &str,
    config: &VolatileConfig,
    seen: &mut HashMap<String, bool>,
    fields: &mut Vec<String>,
) {
    match data {
        Value::Object(map) => {
            for (key, value) in map {
                let full_path = if prefix.is_empty() {
                    key.clone()
                } else {
                    format!("{}.{}", prefix, key)
                };

                if should_ignore_field(key, config) && !seen.contains_key(&full_path) {
                    seen.insert(full_path.clone(), true);
                    fields.push(full_path.clone());
                }

                collect_field_names(value, &full_path, config, seen, fields);
            }
        }
        Value::Array(arr) => {
            for item in arr {
                collect_field_names(item, prefix, config, seen, fields);
            }
        }
        _ => {}
    }
}
