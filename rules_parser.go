package main

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func ParseRulesFile(path string) (*RulesConfig, error) {
	data, err := ReadFileSafe(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}

	var config RulesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse rules YAML: %w", err)
	}

	if err := validateRules(&config); err != nil {
		return nil, fmt.Errorf("invalid rules configuration: %w", err)
	}

	return &config, nil
}

func validateRules(config *RulesConfig) error {
	rules := config.Rules

	if rules.Latency != nil {
		if err := validateLatencyRule(rules.Latency); err != nil {
			return fmt.Errorf("global latency rule: %w", err)
		}
	}

	for i, endpoint := range rules.EndpointRules {
		if endpoint.Path == "" {
			return fmt.Errorf("endpoint_rules[%d]: path is required", i)
		}

		if endpoint.Latency != nil {
			if err := validateLatencyRule(endpoint.Latency); err != nil {
				return fmt.Errorf("endpoint_rules[%d].latency: %w", i, err)
			}
		}
	}

	return nil
}

func validateLatencyRule(rule *LatencyRule) error {
	validMetrics := map[string]bool{
		"p50": true, "p90": true, "p95": true, "p99": true,
		"avg": true, "max": true, "min": true,
	}

	if !validMetrics[rule.Metric] {
		return fmt.Errorf("invalid metric '%s', must be one of: p50, p90, p95, p99, avg, max, min", rule.Metric)
	}

	if rule.RegressionPercent < 0 {
		return fmt.Errorf("regression_percent cannot be negative: %.2f", rule.RegressionPercent)
	}

	return nil
}
