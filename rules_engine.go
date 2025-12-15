package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func EvaluateRules(config *RulesConfig, current *ReplayRunData, baseline *ReplayRunData) *RuleEvaluationResult {
	result := &RuleEvaluationResult{
		Passed:   true,
		Failures: []RuleFailure{},
	}

	rules := &config.Rules

	if rules.StatusMismatch != nil {
		failures := evaluateStatusMismatchRule(rules.StatusMismatch, current.Results, "global")
		result.Failures = append(result.Failures, failures...)
	}

	if rules.BodyDiff != nil {
		failures := evaluateBodyDiffRule(rules.BodyDiff, current.Results, "global")
		result.Failures = append(result.Failures, failures...)
	}

	if rules.Latency != nil && baseline != nil {
		failure := evaluateLatencyRule(rules.Latency, current.Summary.Latency, baseline.Summary.Latency, "global")
		if failure != nil {
			result.Failures = append(result.Failures, *failure)
		}
	}

	for _, endpointRule := range rules.EndpointRules {
		failures := evaluateEndpointRule(&endpointRule, current, baseline)
		result.Failures = append(result.Failures, failures...)
	}

	sort.Slice(result.Failures, func(i, j int) bool {
		if result.Failures[i].Scope != result.Failures[j].Scope {
			return result.Failures[i].Scope < result.Failures[j].Scope
		}

		return result.Failures[i].Rule < result.Failures[j].Rule
	})

	result.Passed = len(result.Failures) == 0

	return result
}

func evaluateStatusMismatchRule(rule *StatusMismatchRule, results []MultiEnvResult, scope string) []RuleFailure {
	count := 0
	affectedRequests := []int{}

	for _, result := range results {
		if result.Diff != nil && result.Diff.StatusMismatch {
			count++
			affectedRequests = append(affectedRequests, result.Index)
		}
	}

	if count > rule.Max {
		return []RuleFailure{{
			Rule:    "status_mismatch",
			Scope:   scope,
			Message: fmt.Sprintf("Found %d status mismatches, maximum allowed is %d", count, rule.Max),
			Details: map[string]any{
				"count":             count,
				"max_allowed":       rule.Max,
				"affected_requests": affectedRequests,
			},
		}}
	}

	return nil
}

func evaluateBodyDiffRule(rule *BodyDiffRule, results []MultiEnvResult, scope string) []RuleFailure {
	if rule.Allowed {
		return nil
	}

	count := 0
	affectedRequests := []int{}

	for _, result := range results {
		if result.Diff != nil && result.Diff.BodyMismatch {
			if result.Diff.VolatileOnly {
				continue
			}

			if shouldIgnoreDiff(result.Diff, rule.Ignore) {
				continue
			}

			count++
			affectedRequests = append(affectedRequests, result.Index)
		}
	}

	if count > 0 {
		return []RuleFailure{{
			Rule:    "body_diff",
			Scope:   scope,
			Message: fmt.Sprintf("Found %d body differences (body diffs not allowed)", count),
			Details: map[string]any{
				"count":             count,
				"allowed":           false,
				"affected_requests": affectedRequests,
			},
		}}
	}

	return nil
}

func shouldIgnoreDiff(diff *ResponseDiff, ignorePatterns []string) bool {
	if len(ignorePatterns) == 0 {
		return false
	}

	for _, ignoredField := range diff.IgnoredFields {
		for _, pattern := range ignorePatterns {
			if matchPattern(ignoredField, pattern) {
				return true
			}
		}
	}

	return false
}

func matchPattern(field, pattern string) bool {
	if after, ok := strings.CutPrefix(pattern, "*."); ok {
		suffix := after
		return strings.HasSuffix(field, suffix)
	}

	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return strings.HasPrefix(field, prefix)
	}

	return field == pattern
}

func evaluateLatencyRule(rule *LatencyRule, current, baseline LatencyStats, scope string) *RuleFailure {
	currentValue := getLatencyMetric(current, rule.Metric)
	baselineValue := getLatencyMetric(baseline, rule.Metric)

	if baselineValue == 0 {
		return nil
	}

	regression := ((float64(currentValue) - float64(baselineValue)) / float64(baselineValue)) * 100

	if regression > rule.RegressionPercent {
		return &RuleFailure{
			Rule:    "latency",
			Scope:   scope,
			Message: fmt.Sprintf("Latency regression of %.2f%% exceeds threshold of %.2f%% (%s: %dms -> %dms)", regression, rule.RegressionPercent, rule.Metric, baselineValue, currentValue),
			Details: map[string]any{
				"metric":             rule.Metric,
				"baseline_ms":        baselineValue,
				"current_ms":         currentValue,
				"regression_percent": regression,
				"threshold_percent":  rule.RegressionPercent,
			},
		}
	}

	return nil
}

func getLatencyMetric(stats LatencyStats, metric string) int64 {
	switch metric {
	case "p50":
		return stats.P50
	case "p90":
		return stats.P90
	case "p95":
		return stats.P95
	case "p99":
		return stats.P99
	case "avg":
		return stats.Avg
	case "max":
		return stats.Max
	case "min":
		return stats.Min
	default:
		return 0
	}
}

func evaluateEndpointRule(rule *EndpointRule, current, baseline *ReplayRunData) []RuleFailure {
	matchingResults := filterResultsByEndpoint(current.Results, rule.Path, rule.Method)

	if len(matchingResults) == 0 {
		return nil
	}

	scope := fmt.Sprintf("endpoint:%s", rule.Path)
	if rule.Method != "" {
		scope = fmt.Sprintf("endpoint:%s %s", rule.Method, rule.Path)
	}

	var failures []RuleFailure

	if rule.StatusMismatch != nil {
		failures = append(failures, evaluateStatusMismatchRule(rule.StatusMismatch, matchingResults, scope)...)
	}

	if rule.Latency != nil && baseline != nil {
		currentLatency := calculateEndpointLatency(matchingResults)

		baselineMatchingResults := filterResultsByEndpoint(baseline.Results, rule.Path, rule.Method)
		if len(baselineMatchingResults) > 0 {
			baselineLatency := calculateEndpointLatency(baselineMatchingResults)

			failure := evaluateLatencyRule(rule.Latency, currentLatency, baselineLatency, scope)
			if failure != nil {
				failures = append(failures, *failure)
			}
		}
	}

	return failures
}

func filterResultsByEndpoint(results []MultiEnvResult, path, method string) []MultiEnvResult {
	var filtered []MultiEnvResult

	for _, result := range results {
		if !strings.HasPrefix(result.Request.Path, path) {
			continue
		}

		if method != "" && result.Request.Method != method {
			continue
		}

		filtered = append(filtered, result)
	}

	return filtered
}

func calculateEndpointLatency(results []MultiEnvResult) LatencyStats {
	if len(results) == 0 {
		return LatencyStats{}
	}

	var latencies []int64
	for _, result := range results {
		for _, response := range result.Responses {
			latencies = append(latencies, response.LatencyMs)
		}
	}

	return CalculateLatencyStats(latencies)
}

func ReadFileSafe(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	cleanPath := filepath.Clean(path)

	if cleanPath == string(filepath.Separator) {
		return nil, fmt.Errorf("invalid path")
	}

	return os.ReadFile(cleanPath)
}

func LoadBaselineFile(path string) (*ReplayRunData, error) {
	data, err := ReadFileSafe(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read baseline file: %w", err)
	}

	var baseline struct {
		Results []MultiEnvResult `json:"results"`
		Summary Summary          `json:"summary"`
	}

	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("failed to parse baseline JSON: %w", err)
	}

	return &ReplayRunData{
		Results: baseline.Results,
		Summary: baseline.Summary,
	}, nil
}
