package models

import (
	"time"
)

type LogEntry struct {
	Method          string              `json:"method"`
	Path            string              `json:"path"`
	Headers         map[string][]string `json:"headers"`
	Body            string              `json:"body"`
	Status          int                 `json:"status"`
	ResponseHeaders map[string][]string `json:"response_headers"`
	ResponseBody    string              `json:"response_body"`
	Timestamp       time.Time           `json:"timestamp"`
	LatencyMs       int64               `json:"latency_ms"`
}

type ReplayResult struct {
	Index     int
	Status    *int
	LatencyMs int64
	Error     *string
	Body      *string
}

type MultiEnvResult struct {
	Index     int
	Request   LogEntry
	Responses map[string]ReplayResult
	RequestID string
	Diff      *ResponseDiff `json:"diff,omitempty"`
}

type ResponseDiff struct {
	StatusMismatch bool              `json:"status_mismatch"`
	StatusCodes    map[string]int    `json:"status_codes,omitempty"`
	BodyMismatch   bool              `json:"body_mismatch"`
	BodyDiffs      map[string]string `json:"body_diffs,omitempty"`
	LatencyDiff    map[string]int64  `json:"latency_diff,omitempty"`
	VolatileOnly   bool              `json:"volatile_only"`
	IgnoredFields  []string          `json:"ignored_fields,omitempty"`
}

type Summary struct {
	TotalRequests int                    `json:"total_requests"`
	Succeeded     int                    `json:"succeeded"`
	Failed        int                    `json:"failed"`
	Latency       LatencyStats           `json:"latency"`
	ByTarget      map[string]TargetStats `json:"by_target"`
}

type LatencyStats struct {
	P50 int64 `json:"p50"`
	P90 int64 `json:"p90"`
	P95 int64 `json:"p95"`
	P99 int64 `json:"p99"`
	Min int64 `json:"min"`
	Max int64 `json:"max"`
	Avg int64 `json:"avg"`
}

type TargetStats struct {
	Succeeded int          `json:"succeeded"`
	Failed    int          `json:"failed"`
	Latency   LatencyStats `json:"latency"`
}

type AggregatedStats struct {
	TotalRequests int
	Succeeded     int
	Failed        int
	Latencies     []int64
	TargetStats   map[string]*TargetStats
}
