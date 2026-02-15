package models

import (
	"slices"
)

func CalculateLatencyStats(latencies []int64) LatencyStats {
	if len(latencies) == 0 {
		return LatencyStats{}
	}

	sorted := make([]int64, len(latencies))
	copy(sorted, latencies)
	slices.Sort(sorted)

	var sum int64
	for _, lat := range sorted {
		sum += lat
	}

	return LatencyStats{
		P50: Percentile(sorted, 50),
		P90: Percentile(sorted, 90),
		P95: Percentile(sorted, 95),
		P99: Percentile(sorted, 99),
		Min: sorted[0],
		Max: sorted[len(sorted)-1],
		Avg: sum / int64(len(sorted)),
	}
}

func Percentile(latencies []int64, p int) int64 {
	if len(latencies) == 0 {
		return 0
	}

	idx := (len(latencies) * p / 100)
	if idx >= len(latencies) {
		idx = len(latencies) - 1
	}

	return latencies[idx]
}
