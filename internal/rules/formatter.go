package rules

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kx0101/replayer/internal/cli"
)

func FormatRuleResult(result *RuleEvaluationResult) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("═══════════════════════════════════════════════════════\n")
	sb.WriteString("            REGRESSION RULES EVALUATION\n")
	sb.WriteString("═══════════════════════════════════════════════════════\n")
	sb.WriteString("\n")

	if result.Passed {
		sb.WriteString("PASSED - All rules satisfied\n")
		sb.WriteString("═══════════════════════════════════════════════════════\n")

		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("FAILED - %d rule violation(s) detected\n", len(result.Failures)))
	sb.WriteString("\n")

	for i, failure := range result.Failures {
		sb.WriteString("───────────────────────────────────────────────────────\n")
		sb.WriteString(fmt.Sprintf("Failure #%d\n", i+1))
		sb.WriteString("───────────────────────────────────────────────────────\n")
		sb.WriteString(fmt.Sprintf("Rule:    %s\n", failure.Rule))
		sb.WriteString(fmt.Sprintf("Scope:   %s\n", failure.Scope))
		sb.WriteString(fmt.Sprintf("Message: %s\n", failure.Message))

		if len(failure.Details) > 0 {
			sb.WriteString("\nDetails:\n")

			for key, value := range failure.Details {
				sb.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("═══════════════════════════════════════════════════════\n")

	return sb.String()
}

func FormatRuleResultJSON(result *RuleEvaluationResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(data), nil
}

func GetExitCode(result *RuleEvaluationResult) cli.ExitCode {
	if result.Passed {
		return cli.ExitOK
	}

	return cli.ExitRules
}
