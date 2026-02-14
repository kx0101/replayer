package output

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/kx0101/replayer/internal/models"
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

	agg := AggregateResults(results)
	diffCount := 0

	if compare {
		for _, r := range results {
			if r.Diff != nil {
				diffCount++
			}
		}
	}

	printResults(results, diffCount, compare, agg)
}

func AggregateResults(results []models.MultiEnvResult) models.AggregatedStats {
	targetStats := map[string]*models.TargetStats{}
	if len(results) > 0 {
		for target := range results[0].Responses {
			targetStats[target] = &models.TargetStats{}
		}
	}

	var totalRequests, succeeded, failed int
	var latencies []int64

	for _, r := range results {
		for target, replay := range r.Responses {
			totalRequests++
			ts := targetStats[target]

			if replay.Status != nil && *replay.Status < 400 {
				succeeded++
				ts.Succeeded++
			} else {
				failed++
				ts.Failed++
			}

			latencies = append(latencies, replay.LatencyMs)
		}
	}

	for target, ts := range targetStats {
		var targetLat []int64
		for _, r := range results {
			if replay, ok := r.Responses[target]; ok {
				targetLat = append(targetLat, replay.LatencyMs)
			}
		}

		ts.Latency = models.CalculateLatencyStats(targetLat)
	}

	return models.AggregatedStats{
		TotalRequests: totalRequests,
		Succeeded:     succeeded,
		Failed:        failed,
		Latencies:     latencies,
		TargetStats:   targetStats,
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

func printResults(results []models.MultiEnvResult, diffCount int, compare bool, agg models.AggregatedStats) {
	slices.Sort(agg.Latencies)
	overallLatency := models.CalculateLatencyStats(agg.Latencies)

	fmt.Printf("Total Requests: %d\nSucceeded: %s%d%s\nFailed: %s%d%s\n",
		agg.TotalRequests, ColorGreen, agg.Succeeded, ColorReset, ColorRed, agg.Failed, ColorReset)

	if compare && diffCount > 0 {
		fmt.Printf("Differences: %s%d%s\n", ColorYellow, diffCount, ColorReset)
	}

	fmt.Println("\nLatency (ms):")
	printLatencyStats(overallLatency)

	if len(agg.TargetStats) > 1 {
		fmt.Println("\nPer-Target Statistics:")

		for target, ts := range agg.TargetStats {
			fmt.Printf("\n%s%s:%s\n  Succeeded: %d\n  Failed: %d\n  Latency:\n", ColorCyan, target, ColorReset, ts.Succeeded, ts.Failed)
			printLatencyStats(ts.Latency)
		}
	}

	if len(results) == 0 {
		return
	}

	for _, r := range results {
		for target, replay := range r.Responses {
			statusStr, color := formatStatus(replay.Status)
			errMsg := ""
			if replay.Error != nil {
				errMsg = fmt.Sprintf(" (%s)", *replay.Error)
			}

			fmt.Printf("[%d][%s] %s%s%s -> %dms%s\n", r.Index, target, color, statusStr, ColorReset, replay.LatencyMs, errMsg)
		}

		if compare && r.Diff != nil {
			printDiff(r)
		}
	}
}

func printLatencyStats(stats models.LatencyStats) {
	fmt.Printf("  min: %d  avg: %d  p50: %d  p90: %d  p95: %d  p99: %d  max: %d\n", stats.Min, stats.Avg, stats.P50, stats.P90, stats.P95, stats.P99, stats.Max)
}

func formatStatus(status *int) (string, string) {
	if status == nil {
		return "ERR", ColorRed
	}

	if *status < 400 {
		return fmt.Sprintf("%d", *status), ColorGreen
	} else if *status < 500 {
		return fmt.Sprintf("%d", *status), ColorYellow
	}

	return fmt.Sprintf("%d", *status), ColorRed
}

func printDiff(result models.MultiEnvResult) {
	diff := result.Diff
	if diff == nil {
		return
	}

	diffType := ""
	if diff.VolatileOnly {
		diffType = " (volatile fields only)"
	}

	fmt.Printf("%s  [DIFF] Request %d%s:%s\n", ColorYellow, result.Index, diffType, ColorReset)
	if diff.StatusMismatch {
		fmt.Printf("    Status codes differ: ")
		for target, status := range diff.StatusCodes {
			fmt.Printf("%s=%d ", target, status)
		}

		fmt.Println()
	}

	if diff.BodyMismatch {
		fmt.Printf("    Response bodies differ\n")
		for target, body := range diff.BodyDiffs {
			fmt.Printf("      %s: %s\n", target, body)
		}
	}

	if len(diff.IgnoredFields) > 0 {
		fmt.Printf("    %sIgnored fields:%s ", ColorCyan, ColorReset)
		if len(diff.IgnoredFields) <= 5 {
			fmt.Printf("%v\n", diff.IgnoredFields)
		} else {
			fmt.Printf("%v and %d more...\n", diff.IgnoredFields[:5], len(diff.IgnoredFields)-5)
		}
	}

	if len(diff.LatencyDiff) > 1 {
		fmt.Printf("    Latency: ")
		for target, lat := range diff.LatencyDiff {
			fmt.Printf("%s=%dms ", target, lat)
		}

		fmt.Println()
	}
}

func ConvertToSummary(agg models.AggregatedStats) models.Summary {
	byTarget := make(map[string]models.TargetStats)
	for target, stats := range agg.TargetStats {
		byTarget[target] = *stats
	}

	return models.Summary{
		TotalRequests: agg.TotalRequests,
		Succeeded:     agg.Succeeded,
		Failed:        agg.Failed,
		Latency:       models.CalculateLatencyStats(agg.Latencies),
		ByTarget:      byTarget,
	}
}

func generateSummary(results []models.MultiEnvResult) models.Summary {
	agg := AggregateResults(results)
	byTarget := map[string]models.TargetStats{}

	for k, v := range agg.TargetStats {
		byTarget[k] = *v
	}

	return models.Summary{
		TotalRequests: agg.TotalRequests,
		Succeeded:     agg.Succeeded,
		Failed:        agg.Failed,
		Latency:       models.CalculateLatencyStats(agg.Latencies),
		ByTarget:      byTarget,
	}
}
