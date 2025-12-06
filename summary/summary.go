package summary

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/kx0101/replayer/models"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
	ColorBold   = "\033[1m"
)

func PrintSummary(results []models.MultiEnvResult, compare bool) {
	fmt.Println(ColorBold + "==== Summary ====" + ColorReset)

	var totalRequests int
	var succeeded int
	var failed int
	var latencies []int64
	targetStats := make(map[string]*models.TargetStats)

	if len(results) > 0 {
		for target := range results[0].Responses {
			targetStats[target] = &models.TargetStats{
				Succeeded: 0,
				Failed:    0,
			}
		}
	}

	diffCount := 0

	for _, result := range results {
		for target, replay := range result.Responses {
			totalRequests++

			stats := targetStats[target]

			if replay.Status != nil && *replay.Status < 400 {
				succeeded++
				stats.Succeeded++
			} else {
				failed++
				stats.Failed++
			}

			latencies = append(latencies, replay.LatencyMs)

			var statusColor string
			var statusStr string

			if replay.Status != nil {
				statusStr = fmt.Sprintf("%d", *replay.Status)
				if *replay.Status < 400 {
					statusColor = ColorGreen
				} else {
					statusColor = ColorYellow
				}
			} else {
				statusStr = "ERR"
				statusColor = ColorRed
			}

			errMsg := ""
			if replay.Error != nil {
				errMsg = fmt.Sprintf(" (%s)", *replay.Error)
			}

			fmt.Printf("[%d][%s] %s%s%s -> %dms%s\n",
				result.Index,
				target,
				statusColor,
				statusStr,
				ColorReset,
				replay.LatencyMs,
				errMsg,
			)
		}

		if compare && result.Diff != nil {
			diffCount++
			printDiff(result)
		}
	}

	slices.Sort(latencies)

	overallLatency := calculateLatencyStats(latencies)

	for target, stats := range targetStats {
		var targetLatencies []int64

		for _, result := range results {
			if replay, ok := result.Responses[target]; ok {
				targetLatencies = append(targetLatencies, replay.LatencyMs)
			}
		}

		stats.Latency = calculateLatencyStats(targetLatencies)
	}

	fmt.Println()
	fmt.Printf("%sOverall Statistics:%s\n", ColorBold, ColorReset)
	fmt.Printf("Total Requests: %d\n", totalRequests)
	fmt.Printf("Succeeded:      %s%d%s\n", ColorGreen, succeeded, ColorReset)
	fmt.Printf("Failed:         %s%d%s\n", ColorRed, failed, ColorReset)

	if compare && diffCount > 0 {
		fmt.Printf("Differences:    %s%d%s\n", ColorYellow, diffCount, ColorReset)
	}

	fmt.Println()
	fmt.Println("Latency (ms):")
	printLatencyStats(overallLatency)

	if len(targetStats) > 1 {
		fmt.Println()
		fmt.Printf("%sPer-Target Statistics:%s\n", ColorBold, ColorReset)

		for target, stats := range targetStats {
			fmt.Printf("\n%s%s:%s\n", ColorCyan, target, ColorReset)
			fmt.Printf("  Succeeded: %d\n", stats.Succeeded)
			fmt.Printf("  Failed:    %d\n", stats.Failed)
			fmt.Printf("  Latency:\n")

			printLatencyStats(stats.Latency)
		}
	}
}

func printLatencyStats(stats models.LatencyStats) {
	fmt.Printf("  min: %d\n", stats.Min)
	fmt.Printf("  avg: %d\n", stats.Avg)
	fmt.Printf("  p50: %d\n", stats.P50)
	fmt.Printf("  p90: %d\n", stats.P90)
	fmt.Printf("  p95: %d\n", stats.P95)
	fmt.Printf("  p99: %d\n", stats.P99)
	fmt.Printf("  max: %d\n", stats.Max)
}

func calculateLatencyStats(latencies []int64) models.LatencyStats {
	if len(latencies) == 0 {
		return models.LatencyStats{}
	}

	sorted := make([]int64, len(latencies))
	copy(sorted, latencies)
	slices.Sort(sorted)

	var sum int64
	for _, lat := range sorted {
		sum += lat
	}

	return models.LatencyStats{
		P50: percentile(sorted, 50),
		P90: percentile(sorted, 90),
		P95: percentile(sorted, 95),
		P99: percentile(sorted, 99),
		Min: sorted[0],
		Max: sorted[len(sorted)-1],
		Avg: sum / int64(len(sorted)),
	}
}

func percentile(latencies []int64, p int) int64 {
	if len(latencies) == 0 {
		return 0
	}

	idx := (len(latencies) * p / 100)
	if idx >= len(latencies) {
		idx = len(latencies) - 1
	}

	return latencies[idx]
}

func printDiff(result models.MultiEnvResult) {
	if result.Diff == nil {
		return
	}

	fmt.Printf("%s  [DIFF] Request %d:%s\n", ColorYellow, result.Index, ColorReset)

	if result.Diff.StatusMismatch {
		fmt.Printf("    Status codes differ: ")

		for target, status := range result.Diff.StatusCodes {
			fmt.Printf("%s=%d ", target, status)
		}

		fmt.Println()
	}

	if result.Diff.BodyMismatch {
		fmt.Printf("    Response bodies differ\n")

		if len(result.Diff.BodyDiffs) > 0 {
			for target, body := range result.Diff.BodyDiffs {
				fmt.Printf("      %s: %s\n", target, body)
			}
		}
	}

	if len(result.Diff.LatencyDiff) > 1 {
		fmt.Printf("    Latency: ")

		for target, lat := range result.Diff.LatencyDiff {
			fmt.Printf("%s=%dms ", target, lat)
		}

		fmt.Println()
	}
}

func PrintJSONOutput(results []models.MultiEnvResult) {
	output := map[string]any{
		"results": results,
		"summary": generateSummary(results),
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}

func generateSummary(results []models.MultiEnvResult) models.Summary {
	var totalRequests int
	var succeeded int
	var failed int
	var latencies []int64
	targetStats := make(map[string]*models.TargetStats)

	if len(results) > 0 {
		for target := range results[0].Responses {
			targetStats[target] = &models.TargetStats{}
		}
	}

	for _, result := range results {
		for target, replay := range result.Responses {
			totalRequests++

			stats := targetStats[target]

			if replay.Status != nil && *replay.Status < 400 {
				succeeded++
				stats.Succeeded++
			} else {
				failed++
				stats.Failed++
			}

			latencies = append(latencies, replay.LatencyMs)
		}
	}

	slices.Sort(latencies)

	overallLatency := calculateLatencyStats(latencies)

	for target, stats := range targetStats {
		var targetLatencies []int64
		for _, result := range results {
			if replay, ok := result.Responses[target]; ok {
				targetLatencies = append(targetLatencies, replay.LatencyMs)
			}
		}

		stats.Latency = calculateLatencyStats(targetLatencies)
	}

	return models.Summary{
		TotalRequests: totalRequests,
		Succeeded:     succeeded,
		Failed:        failed,
		Latency:       overallLatency,
		ByTarget:      map[string]models.TargetStats{},
	}
}
