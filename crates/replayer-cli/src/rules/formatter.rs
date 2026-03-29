use std::fmt::Write;

use replayer_core::ExitCode;

use super::types::RuleEvaluationResult;

pub fn format_rule_result(result: &RuleEvaluationResult) -> String {
    let mut sb = String::new();

    writeln!(sb).unwrap();
    writeln!(
        sb,
        "═══════════════════════════════════════════════════════"
    )
    .unwrap();
    writeln!(sb, "            REGRESSION RULES EVALUATION").unwrap();
    writeln!(
        sb,
        "═══════════════════════════════════════════════════════"
    )
    .unwrap();
    writeln!(sb).unwrap();

    if result.passed {
        writeln!(sb, "PASSED - All rules satisfied").unwrap();
        writeln!(
            sb,
            "═══════════════════════════════════════════════════════"
        )
        .unwrap();
        return sb;
    }

    writeln!(
        sb,
        "FAILED - {} rule violation(s) detected",
        result.failures.len()
    )
    .unwrap();
    writeln!(sb).unwrap();

    for (i, failure) in result.failures.iter().enumerate() {
        writeln!(
            sb,
            "───────────────────────────────────────────────────────"
        )
        .unwrap();
        writeln!(sb, "Failure #{}", i + 1).unwrap();
        writeln!(
            sb,
            "───────────────────────────────────────────────────────"
        )
        .unwrap();
        writeln!(sb, "Rule:    {}", failure.rule).unwrap();
        writeln!(sb, "Scope:   {}", failure.scope).unwrap();
        writeln!(sb, "Message: {}", failure.message).unwrap();

        if !failure.details.is_empty() {
            writeln!(sb).unwrap();
            writeln!(sb, "Details:").unwrap();
            for (key, value) in &failure.details {
                writeln!(sb, "  {}: {}", key, value).unwrap();
            }
        }
        writeln!(sb).unwrap();
    }

    writeln!(
        sb,
        "═══════════════════════════════════════════════════════"
    )
    .unwrap();

    sb
}

pub fn format_rule_result_json(result: &RuleEvaluationResult) -> anyhow::Result<String> {
    let json = serde_json::to_string_pretty(result)
        .map_err(|e| anyhow::anyhow!("failed to marshal result: {}", e))?;
    Ok(json)
}

pub fn get_exit_code(result: &RuleEvaluationResult) -> ExitCode {
    if result.passed {
        ExitCode::Ok
    } else {
        ExitCode::Rules
    }
}
