use crate::models::LatencyStats;

pub fn calculate_latency_stats(latencies: &[i64]) -> LatencyStats {
    if latencies.is_empty() {
        return LatencyStats::default();
    }

    let mut sorted: Vec<i64> = latencies.to_vec();
    sorted.sort_unstable();

    let sum: i64 = sorted.iter().sum();

    LatencyStats {
        p50: percentile(&sorted, 50),
        p90: percentile(&sorted, 90),
        p95: percentile(&sorted, 95),
        p99: percentile(&sorted, 99),
        min: sorted[0],
        max: sorted[sorted.len() - 1],
        avg: sum / sorted.len() as i64,
    }
}

pub fn percentile(latencies: &[i64], p: usize) -> i64 {
    if latencies.is_empty() {
        return 0;
    }

    let mut idx = latencies.len() * p / 100;
    if idx >= latencies.len() {
        idx = latencies.len() - 1;
    }

    latencies[idx]
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_percentile_empty() {
        assert_eq!(percentile(&[], 50), 0);
    }

    #[test]
    fn test_percentile_single() {
        assert_eq!(percentile(&[42], 50), 42);
        assert_eq!(percentile(&[42], 99), 42);
    }

    #[test]
    fn test_calculate_latency_stats() {
        let latencies: Vec<i64> = (1..=100).collect();
        let stats = calculate_latency_stats(&latencies);
        assert_eq!(stats.min, 1);
        assert_eq!(stats.max, 100);
        assert_eq!(stats.avg, 50);
        assert_eq!(stats.p50, 51);
        assert_eq!(stats.p90, 91);
        assert_eq!(stats.p95, 96);
        assert_eq!(stats.p99, 100);
    }

    #[test]
    fn test_calculate_latency_stats_empty() {
        let stats = calculate_latency_stats(&[]);
        assert_eq!(stats.min, 0);
        assert_eq!(stats.max, 0);
        assert_eq!(stats.avg, 0);
    }
}
