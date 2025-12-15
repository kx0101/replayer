package main

type RulesConfig struct {
	Rules Rules `yaml:"rules"`
}

type Rules struct {
	StatusMismatch *StatusMismatchRule `yaml:"status_mismatch,omitempty"`
	BodyDiff       *BodyDiffRule       `yaml:"body_diff,omitempty"`
	Latency        *LatencyRule        `yaml:"latency,omitempty"`
	EndpointRules  []EndpointRule      `yaml:"endpoint_rules,omitempty"`
}

type StatusMismatchRule struct {
	Max int `yaml:"max"`
}

type BodyDiffRule struct {
	Allowed bool     `yaml:"allowed"`
	Ignore  []string `yaml:"ignore,omitempty"`
}

type LatencyRule struct {
	Metric            string  `yaml:"metric"`
	RegressionPercent float64 `yaml:"regression_percent"`
}

type EndpointRule struct {
	Path           string              `yaml:"path"`
	Method         string              `yaml:"method,omitempty"`
	Latency        *LatencyRule        `yaml:"latency,omitempty"`
	StatusMismatch *StatusMismatchRule `yaml:"status_mismatch,omitempty"`
}

type RuleEvaluationResult struct {
	Passed   bool
	Failures []RuleFailure
}

type RuleFailure struct {
	Rule    string
	Scope   string
	Message string
	Details map[string]any
}

type ReplayRunData struct {
	Results []MultiEnvResult
	Summary Summary
}
