use replayer_core::models::LogEntry;

pub fn apply(entries: Vec<LogEntry>, filter_method: &str, filter_path: &str) -> Vec<LogEntry> {
    if filter_method.is_empty() && filter_path.is_empty() {
        return entries;
    }

    entries
        .into_iter()
        .filter(|entry| {
            if !filter_method.is_empty() && !entry.method.eq_ignore_ascii_case(filter_method) {
                return false;
            }

            if !filter_path.is_empty() && !entry.path.contains(filter_path) {
                return false;
            }

            true
        })
        .collect()
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

    #[test]
    fn test_no_filters_applied() {
        let entries = vec![
            entry("GET", "/users"),
            entry("POST", "/users"),
            entry("DELETE", "/users/123"),
        ];
        let filtered = apply(entries, "", "");
        assert_eq!(filtered.len(), 3);
    }

    #[test]
    fn test_filter_by_method_get() {
        let entries = vec![
            entry("GET", "/users"),
            entry("POST", "/users"),
            entry("GET", "/posts"),
            entry("DELETE", "/users/123"),
            entry("GET", "/comments"),
        ];
        let filtered = apply(entries, "GET", "");
        assert_eq!(filtered.len(), 3);
        for e in &filtered {
            assert_eq!(e.method, "GET");
        }
    }

    #[test]
    fn test_filter_by_method_post() {
        let entries = vec![
            entry("GET", "/users"),
            entry("POST", "/users"),
            entry("POST", "/posts"),
            entry("DELETE", "/users/123"),
        ];
        let filtered = apply(entries, "POST", "");
        assert_eq!(filtered.len(), 2);
        for e in &filtered {
            assert_eq!(e.method, "POST");
        }
    }

    #[test]
    fn test_filter_by_path_substring() {
        let entries = vec![
            entry("GET", "/api/users"),
            entry("POST", "/api/users"),
            entry("GET", "/api/posts"),
            entry("DELETE", "/api/users/123"),
            entry("GET", "/health"),
        ];
        let filtered = apply(entries, "", "/api/users");
        assert_eq!(filtered.len(), 3);
        for e in &filtered {
            assert!(
                e.path == "/api/users" || e.path == "/api/users/123",
                "unexpected path: {}",
                e.path
            );
        }
    }

    #[test]
    fn test_filter_by_both_method_and_path() {
        let entries = vec![
            entry("GET", "/api/users"),
            entry("POST", "/api/users"),
            entry("GET", "/api/posts"),
            entry("DELETE", "/api/users/123"),
            entry("POST", "/api/users/456"),
        ];
        let filtered = apply(entries, "POST", "/api/users");
        assert_eq!(filtered.len(), 2);
        for e in &filtered {
            assert_eq!(e.method, "POST");
            assert!(
                e.path == "/api/users" || e.path == "/api/users/456",
                "unexpected path: {}",
                e.path
            );
        }
    }

    #[test]
    fn test_filter_matches_nothing() {
        let entries = vec![entry("GET", "/users"), entry("POST", "/posts")];
        let filtered = apply(entries, "DELETE", "");
        assert_eq!(filtered.len(), 0);
    }

    #[test]
    fn test_case_insensitive_method_filtering() {
        let entries = vec![
            entry("GET", "/users"),
            entry("get", "/posts"),
            entry("Get", "/comments"),
        ];
        let filtered = apply(entries, "GET", "");
        assert_eq!(filtered.len(), 3);
    }

    #[test]
    fn test_partial_path_matching() {
        let entries = vec![
            entry("GET", "/api/v1/users"),
            entry("GET", "/api/v2/users"),
            entry("GET", "/users"),
            entry("GET", "/api/posts"),
        ];
        let filtered = apply(entries, "", "users");
        assert_eq!(filtered.len(), 3);
        for e in &filtered {
            assert!(e.path.contains("users"), "unexpected path: {}", e.path);
        }
    }

    #[test]
    fn test_empty_entries_list() {
        let entries: Vec<LogEntry> = vec![];
        let filtered = apply(entries, "GET", "/test");
        assert_eq!(filtered.len(), 0);
    }

    #[test]
    fn test_filter_specific_endpoint() {
        let entries = vec![
            entry("GET", "/api/checkout"),
            entry("POST", "/api/checkout"),
            entry("GET", "/api/cart"),
            entry("POST", "/api/orders"),
            entry("GET", "/api/checkout/success"),
        ];
        let filtered = apply(entries, "POST", "/api/checkout");
        assert_eq!(filtered.len(), 1);
        assert_eq!(filtered[0].method, "POST");
        assert_eq!(filtered[0].path, "/api/checkout");
    }

    #[test]
    fn test_filter_with_special_characters_in_path() {
        let entries = vec![
            entry("GET", "/api/users?page=1"),
            entry("GET", "/api/users?page=2"),
            entry("GET", "/api/posts"),
        ];
        let filtered = apply(entries, "", "?page=");
        assert_eq!(filtered.len(), 2);
    }

    #[test]
    fn test_all_entries_filtered_out() {
        let entries = vec![
            entry("GET", "/users"),
            entry("GET", "/posts"),
            entry("GET", "/comments"),
        ];
        let filtered = apply(entries, "POST", "/nonexistent");
        assert_eq!(filtered.len(), 0);
    }

    #[test]
    fn test_method_filter_with_lowercase_input() {
        let entries = vec![entry("GET", "/users"), entry("POST", "/users")];
        let filtered = apply(entries, "get", "");
        assert_eq!(filtered.len(), 1);
        assert_eq!(filtered[0].method, "GET");
    }

    #[test]
    fn test_complex_filtering_scenario() {
        let entries = vec![
            entry("GET", "/api/v1/users/123"),
            entry("POST", "/api/v1/users"),
            entry("GET", "/api/v2/users/456"),
            entry("PUT", "/api/v1/users/123"),
            entry("GET", "/api/v1/posts"),
            entry("DELETE", "/api/v1/users/123"),
            entry("GET", "/health"),
        ];
        let filtered = apply(entries, "GET", "/api/v1/users");
        assert_eq!(filtered.len(), 1);
        assert_eq!(filtered[0].path, "/api/v1/users/123");
    }
}
