package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRulesFile_ValidConfig(t *testing.T) {
	yamlContent := `rules:
  status_mismatch:
    max: 5
  body_diff:
    allowed: false
  latency:
    metric: p95
    regression_percent: 20.0
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "rules.yaml")

	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config, err := ParseRulesFile(filePath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	if config.Rules.StatusMismatch == nil || config.Rules.StatusMismatch.Max != 5 {
		t.Errorf("Expected StatusMismatch.Max=5, got: %v", config.Rules.StatusMismatch)
	}

	if config.Rules.BodyDiff == nil || config.Rules.BodyDiff.Allowed != false {
		t.Errorf("Expected BodyDiff.Allowed=false, got: %v", config.Rules.BodyDiff)
	}

	if config.Rules.Latency == nil {
		t.Fatal("Expected Latency rule to be present")
	}

	if config.Rules.Latency.Metric != "p95" {
		t.Errorf("Expected Latency.Metric=p95, got: %s", config.Rules.Latency.Metric)
	}

	if config.Rules.Latency.RegressionPercent != 20.0 {
		t.Errorf("Expected RegressionPercent=20.0, got: %f", config.Rules.Latency.RegressionPercent)
	}
}

func TestParseRulesFile_WithEndpointRules(t *testing.T) {
	yamlContent := `rules:
  endpoint_rules:
    - path: /api/users
      method: GET
      latency:
        metric: p99
        regression_percent: 15.0
      status_mismatch:
        max: 2
    - path: /api/orders
      latency:
        metric: p90
        regression_percent: 25.0
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "rules.yaml")

	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config, err := ParseRulesFile(filePath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(config.Rules.EndpointRules) != 2 {
		t.Fatalf("Expected 2 endpoint rules, got: %d", len(config.Rules.EndpointRules))
	}

	rule1 := config.Rules.EndpointRules[0]
	if rule1.Path != "/api/users" {
		t.Errorf("Expected path '/api/users', got: %s", rule1.Path)
	}

	if rule1.Method != "GET" {
		t.Errorf("Expected method 'GET', got: %s", rule1.Method)
	}

	if rule1.Latency == nil || rule1.Latency.Metric != "p99" {
		t.Errorf("Expected latency metric p99, got: %v", rule1.Latency)
	}

	if rule1.StatusMismatch == nil || rule1.StatusMismatch.Max != 2 {
		t.Errorf("Expected status_mismatch max=2, got: %v", rule1.StatusMismatch)
	}

	rule2 := config.Rules.EndpointRules[1]
	if rule2.Path != "/api/orders" {
		t.Errorf("Expected path '/api/orders', got: %s", rule2.Path)
	}

	if rule2.Method != "" {
		t.Errorf("Expected empty method, got: %s", rule2.Method)
	}
}

func TestParseRulesFile_WithBodyDiffIgnore(t *testing.T) {
	yamlContent := `rules:
  body_diff:
    allowed: false
    ignore:
      - "*.timestamp"
      - "metadata.*"
      - "exact.field.name"
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "rules.yaml")

	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config, err := ParseRulesFile(filePath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if config.Rules.BodyDiff == nil {
		t.Fatal("Expected BodyDiff rule to be present")
	}

	if len(config.Rules.BodyDiff.Ignore) != 3 {
		t.Fatalf("Expected 3 ignore patterns, got: %d", len(config.Rules.BodyDiff.Ignore))
	}

	expectedPatterns := []string{"*.timestamp", "metadata.*", "exact.field.name"}
	for i, expected := range expectedPatterns {
		if config.Rules.BodyDiff.Ignore[i] != expected {
			t.Errorf("Expected ignore[%d]='%s', got: '%s'", i, expected, config.Rules.BodyDiff.Ignore[i])
		}
	}
}

func TestParseRulesFile_FileNotFound(t *testing.T) {
	_, err := ParseRulesFile("/nonexistent/path/rules.yaml")
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}

	if !os.IsNotExist(err) && err.Error() != "" {
		if err.Error() == "" {
			t.Error("Expected error message about file reading")
		}
	}
}

func TestParseRulesFile_InvalidYAML(t *testing.T) {
	yamlContent := `rules:
  status_mismatch:
    max: invalid_number
  - this is broken yaml
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "invalid.yaml")

	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := ParseRulesFile(filePath)
	if err == nil {
		t.Fatal("Expected error for invalid YAML")
	}
}

func TestParseRulesFile_InvalidLatencyMetric(t *testing.T) {
	yamlContent := `rules:
  latency:
    metric: p85
    regression_percent: 20.0
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "rules.yaml")

	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := ParseRulesFile(filePath)
	if err == nil {
		t.Fatal("Expected error for invalid latency metric")
	}

	if err.Error() == "" {
		t.Error("Expected error message about invalid metric")
	}
}

func TestParseRulesFile_NegativeRegressionPercent(t *testing.T) {
	yamlContent := `rules:
  latency:
    metric: p95
    regression_percent: -10.0
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "rules.yaml")

	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := ParseRulesFile(filePath)
	if err == nil {
		t.Fatal("Expected error for negative regression_percent")
	}
}

func TestParseRulesFile_EndpointRuleMissingPath(t *testing.T) {
	yamlContent := `rules:
  endpoint_rules:
    - method: GET
      latency:
        metric: p95
        regression_percent: 20.0
`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "rules.yaml")

	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := ParseRulesFile(filePath)
	if err == nil {
		t.Fatal("Expected error for endpoint rule without path")
	}
}

func TestValidateLatencyRule_AllValidMetrics(t *testing.T) {
	validMetrics := []string{"p50", "p90", "p95", "p99", "avg", "max", "min"}

	for _, metric := range validMetrics {
		rule := &LatencyRule{
			Metric:            metric,
			RegressionPercent: 20.0,
		}

		err := validateLatencyRule(rule)
		if err != nil {
			t.Errorf("Expected metric '%s' to be valid, got error: %v", metric, err)
		}
	}
}

func TestValidateLatencyRule_InvalidMetric(t *testing.T) {
	invalidMetrics := []string{"p85", "median", "average", "P95", ""}

	for _, metric := range invalidMetrics {
		rule := &LatencyRule{
			Metric:            metric,
			RegressionPercent: 20.0,
		}

		err := validateLatencyRule(rule)
		if err == nil {
			t.Errorf("Expected metric '%s' to be invalid", metric)
		}
	}
}

func TestValidateLatencyRule_ZeroRegressionPercent(t *testing.T) {
	rule := &LatencyRule{
		Metric:            "p95",
		RegressionPercent: 0.0,
	}

	err := validateLatencyRule(rule)
	if err != nil {
		t.Errorf("Expected 0.0 regression_percent to be valid, got error: %v", err)
	}
}

func TestValidateRules_ComplexValid(t *testing.T) {
	config := &RulesConfig{
		Rules: Rules{
			StatusMismatch: &StatusMismatchRule{Max: 5},
			BodyDiff:       &BodyDiffRule{Allowed: false},
			Latency: &LatencyRule{
				Metric:            "p95",
				RegressionPercent: 20.0,
			},
			EndpointRules: []EndpointRule{
				{
					Path:   "/api/users",
					Method: "GET",
					Latency: &LatencyRule{
						Metric:            "p99",
						RegressionPercent: 15.0,
					},
				},
				{
					Path: "/api/orders",
					Latency: &LatencyRule{
						Metric:            "avg",
						RegressionPercent: 30.0,
					},
				},
			},
		},
	}

	err := validateRules(config)
	if err != nil {
		t.Errorf("Expected valid configuration, got error: %v", err)
	}
}

func TestValidateRules_MultipleEndpointErrors(t *testing.T) {
	config := &RulesConfig{
		Rules: Rules{
			EndpointRules: []EndpointRule{
				{
					Path:   "/api/users",
					Method: "GET",
					Latency: &LatencyRule{
						Metric:            "p99",
						RegressionPercent: 15.0,
					},
				},
				{
					Method: "POST",
					Latency: &LatencyRule{
						Metric:            "p95",
						RegressionPercent: 20.0,
					},
				},
			},
		},
	}

	err := validateRules(config)
	if err == nil {
		t.Fatal("Expected error for missing path in endpoint rule")
	}
}

func TestParseRulesFile_EmptyFile(t *testing.T) {
	yamlContent := `rules: {}`

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.yaml")

	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	config, err := ParseRulesFile(filePath)
	if err != nil {
		t.Fatalf("Unexpected error for empty rules: %v", err)
	}

	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	if config.Rules.StatusMismatch != nil {
		t.Error("Expected nil StatusMismatch for empty config")
	}
}
