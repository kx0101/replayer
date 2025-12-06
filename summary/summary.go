package summary

import (
	"fmt"
	"slices"

	"github.com/kx0101/replayer/models"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
)

func PrintSummary(results []models.MultiEnvResult) {
	fmt.Println("==== Summary ====")
	var totalRequests int
	var succeeded int
	var failed int
	var latencies []int64

	for _, result := range results {
		for target, replay := range result.Responses {
			totalRequests++

			if replay.Status != nil && *replay.Status < 400 {
				succeeded++
			} else {
				failed++
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
	}

	slices.Sort(latencies)

	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)
	p99 := percentile(latencies, 99)

	fmt.Println()
	fmt.Printf("Total Requests: %d\n", totalRequests)
	fmt.Printf("Succeeded:      %d\n", succeeded)
	fmt.Printf("Failed:         %d\n", failed)
	fmt.Println()
	fmt.Println("Latency (ms):")
	fmt.Printf("  p50: %d\n", p50)
	fmt.Printf("  p95: %d\n", p95)
	fmt.Printf("  p99: %d\n", p99)
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
