package models

import (
	"time"

	"github.com/google/uuid"
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
	Index     int     `json:"index"`
	Status    *int    `json:"status"`
	LatencyMs int64   `json:"latency_ms"`
	Error     *string `json:"error"`
	Body      *string `json:"body"`
}

type MultiEnvResult struct {
	Index     int                      `json:"index"`
	Request   LogEntry                 `json:"request"`
	Responses map[string]ReplayResult  `json:"responses"`
	RequestID string                   `json:"request_id"`
	Diff      *ResponseDiff            `json:"diff,omitempty"`
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

type Summary struct {
	TotalRequests int                    `json:"total_requests"`
	Succeeded     int                    `json:"succeeded"`
	Failed        int                    `json:"failed"`
	Latency       LatencyStats           `json:"latency"`
	ByTarget      map[string]TargetStats `json:"by_target"`
}

type Run struct {
	ID            uuid.UUID              `json:"id"`
	UserID        *uuid.UUID             `json:"user_id,omitempty"`
	Environment   string                 `json:"environment"`
	Targets       []string               `json:"targets"`
	CreatedAt     time.Time              `json:"created_at"`
	TotalRequests int                    `json:"total_requests"`
	Succeeded     int                    `json:"succeeded"`
	Failed        int                    `json:"failed"`
	LatencyStats  LatencyStats           `json:"latency_stats"`
	ByTarget      map[string]TargetStats `json:"by_target"`
	Results       []MultiEnvResult       `json:"results"`
	IsBaseline    bool                   `json:"is_baseline"`
	BaselineID    *uuid.UUID             `json:"baseline_id,omitempty"`
	Labels        map[string]string      `json:"labels,omitempty"`
}

type RunListItem struct {
	ID            uuid.UUID              `json:"id"`
	UserID        *uuid.UUID             `json:"user_id,omitempty"`
	Environment   string                 `json:"environment"`
	Targets       []string               `json:"targets"`
	CreatedAt     time.Time              `json:"created_at"`
	TotalRequests int                    `json:"total_requests"`
	Succeeded     int                    `json:"succeeded"`
	Failed        int                    `json:"failed"`
	LatencyStats  LatencyStats           `json:"latency_stats"`
	ByTarget      map[string]TargetStats `json:"by_target"`
	IsBaseline    bool                   `json:"is_baseline"`
	BaselineID    *uuid.UUID             `json:"baseline_id,omitempty"`
	Labels        map[string]string      `json:"labels,omitempty"`
}

type ComparisonResult struct {
	RunID        uuid.UUID              `json:"run_id"`
	BaselineID   uuid.UUID              `json:"baseline_id"`
	RunSummary   Summary                `json:"run_summary"`
	BaseSummary  Summary                `json:"baseline_summary"`
	DiffCount    int                    `json:"diff_count"`
	LatencyDelta map[string]LatencyDelta `json:"latency_delta"`
}

type LatencyDelta struct {
	Current    LatencyStats `json:"current"`
	Baseline   LatencyStats `json:"baseline"`
	P50Change  float64      `json:"p50_change_pct"`
	P90Change  float64      `json:"p90_change_pct"`
	P95Change  float64      `json:"p95_change_pct"`
	P99Change  float64      `json:"p99_change_pct"`
	AvgChange  float64      `json:"avg_change_pct"`
}

type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	VerifiedAt   *time.Time `json:"verified_at,omitempty"`
	VerifyToken  *string    `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type APIKey struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	KeyHash    string     `json:"-"`
	KeyPrefix  string     `json:"key_prefix"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

type SessionData struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
}
