package models

import "encoding/json"

type LogEntry struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
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
}
