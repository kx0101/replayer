use std::collections::HashMap;
use std::fmt::Write as FmtWrite;
use std::fs::File;
use std::io::Write;

use anyhow::{bail, Context};
use chrono::Utc;
use replayer_core::latency::calculate_latency_stats;
use replayer_core::models::{LatencyStats, MultiEnvResult, TargetStats};

struct ReportData {
    generated_at: String,
    input_file: String,
    targets: Vec<String>,
    total_requests: i32,
    succeeded: i32,
    failed: i32,
    diff_count: i32,
    latency: LatencyStats,
    by_target: HashMap<String, TargetStats>,
    results: Vec<MultiEnvResult>,
    comparison_mode: bool,
}

pub fn generate_html(
    results: &[MultiEnvResult],
    input_file: &str,
    targets: &[String],
    compare: bool,
    output_path: &str,
) -> anyhow::Result<()> {
    let data = build_report_data(results, input_file, targets, compare);
    let html = render_html(&data);

    if output_path.contains("..") {
        bail!("invalid output path: {}", output_path);
    }

    let mut file = File::create(output_path).context("failed to create report file")?;
    file.write_all(html.as_bytes())
        .context("failed to write report file")?;

    Ok(())
}

pub fn status_color(status: Option<i32>) -> &'static str {
    match status {
        None => "error",
        Some(s) if s < 400 => "success",
        Some(s) if s < 500 => "warning",
        _ => "error",
    }
}

pub fn format_path(path: &str) -> &str {
    if path.len() > 50 {
        &path[..47]
    } else {
        path
    }
}

fn format_path_owned(path: &str) -> String {
    if path.len() > 50 {
        format!("{}...", &path[..47])
    } else {
        path.to_string()
    }
}

fn escape_html(s: &str) -> String {
    s.replace('&', "&amp;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
        .replace('"', "&quot;")
        .replace('\'', "&#39;")
}

fn build_report_data(
    results: &[MultiEnvResult],
    input_file: &str,
    targets: &[String],
    compare: bool,
) -> ReportData {
    let mut total_requests = 0i32;
    let mut succeeded = 0i32;
    let mut failed = 0i32;
    let mut diff_count = 0i32;
    let mut latencies = Vec::new();

    let mut target_succeeded: HashMap<String, i32> = HashMap::new();
    let mut target_failed: HashMap<String, i32> = HashMap::new();
    let mut target_latencies: HashMap<String, Vec<i64>> = HashMap::new();

    if let Some(first) = results.first() {
        for target in first.responses.keys() {
            target_succeeded.insert(target.clone(), 0);
            target_failed.insert(target.clone(), 0);
            target_latencies.insert(target.clone(), Vec::new());
        }
    }

    for r in results {
        for (target, replay) in &r.responses {
            total_requests += 1;

            if replay.status.is_some_and(|s| s < 400) {
                succeeded += 1;
                *target_succeeded.entry(target.clone()).or_insert(0) += 1;
            } else {
                failed += 1;
                *target_failed.entry(target.clone()).or_insert(0) += 1;
            }

            latencies.push(replay.latency_ms);
            target_latencies
                .entry(target.clone())
                .or_default()
                .push(replay.latency_ms);
        }

        if r.diff.is_some() {
            diff_count += 1;
        }
    }

    latencies.sort_unstable();
    let overall_latency = calculate_latency_stats(&latencies);

    let mut by_target = HashMap::new();
    for (target, lats) in &target_latencies {
        by_target.insert(
            target.clone(),
            TargetStats {
                succeeded: *target_succeeded.get(target).unwrap_or(&0),
                failed: *target_failed.get(target).unwrap_or(&0),
                latency: calculate_latency_stats(lats),
            },
        );
    }

    ReportData {
        generated_at: Utc::now().format("%Y-%m-%d %H:%M:%S").to_string(),
        input_file: input_file.to_string(),
        targets: targets.to_vec(),
        total_requests,
        succeeded,
        failed,
        diff_count,
        latency: overall_latency,
        by_target,
        results: results.to_vec(),
        comparison_mode: compare,
    }
}

fn render_html(data: &ReportData) -> String {
    let mut html = String::with_capacity(64 * 1024);

    html.push_str(HTML_HEAD);

    let _ = write!(
        html,
        r#"    <div class="container">
        <div class="header">
            <h1>🔄 HTTP Replay Report</h1>
            <div class="meta">
                Generated: {} | Input: {}"#,
        escape_html(&data.generated_at),
        escape_html(&data.input_file)
    );

    if data.comparison_mode {
        html.push_str(" | Mode: Comparison");
    }

    html.push_str(
        r#"
            </div>
        </div>
"#,
    );

    let _ = write!(
        html,
        r#"
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value">{}</div>
                <div class="stat-label">Total Requests</div>
            </div>
            <div class="stat-card">
                <div class="stat-value success">{}</div>
                <div class="stat-label">Succeeded</div>
            </div>
            <div class="stat-card">
                <div class="stat-value error">{}</div>
                <div class="stat-label">Failed</div>
            </div>"#,
        data.total_requests, data.succeeded, data.failed
    );

    if data.comparison_mode {
        let _ = write!(
            html,
            r#"
            <div class="stat-card">
                <div class="stat-value warning">{}</div>
                <div class="stat-label">Differences Found</div>
            </div>"#,
            data.diff_count
        );
    }

    html.push_str(
        r#"
        </div>
"#,
    );

    let _ = write!(
        html,
        r#"
        <div class="section">
            <div class="section-title">⚡ Overall Latency Statistics</div>
            <div class="latency-row">
                <span class="latency-label">Minimum:</span>
                <span class="latency-value">{}ms</span>
            </div>
            <div class="latency-row">
                <span class="latency-label">Average:</span>
                <span class="latency-value">{}ms</span>
            </div>
            <div class="latency-row">
                <span class="latency-label">p50 (Median):</span>
                <span class="latency-value">{}ms</span>
            </div>
            <div class="latency-row">
                <span class="latency-label">p90:</span>
                <span class="latency-value">{}ms</span>
            </div>
            <div class="latency-row">
                <span class="latency-label">p95:</span>
                <span class="latency-value">{}ms</span>
            </div>
            <div class="latency-row">
                <span class="latency-label">p99:</span>
                <span class="latency-value">{}ms</span>
            </div>
            <div class="latency-row">
                <span class="latency-label">Maximum:</span>
                <span class="latency-value">{}ms</span>
            </div>
        </div>
"#,
        data.latency.min,
        data.latency.avg,
        data.latency.p50,
        data.latency.p90,
        data.latency.p95,
        data.latency.p99,
        data.latency.max
    );

    if data.by_target.len() > 1 {
        html.push_str(
            r#"
        <div class="section">
            <div class="section-title">🎯 Per-Target Statistics</div>
            <div class="target-grid">"#,
        );

        for (target, stats) in &data.by_target {
            let _ = write!(
                html,
                r#"
                <div class="target-card">
                    <div class="target-name">{}</div>
                    <div class="latency-row">
                        <span class="latency-label">Succeeded:</span>
                        <span class="latency-value">{}</span>
                    </div>
                    <div class="latency-row">
                        <span class="latency-label">Failed:</span>
                        <span class="latency-value">{}</span>
                    </div>
                    <div class="latency-row">
                        <span class="latency-label">Avg Latency:</span>
                        <span class="latency-value">{}ms</span>
                    </div>
                    <div class="latency-row">
                        <span class="latency-label">p95:</span>
                        <span class="latency-value">{}ms</span>
                    </div>
                </div>"#,
                escape_html(target),
                stats.succeeded,
                stats.failed,
                stats.latency.avg,
                stats.latency.p95
            );
        }

        html.push_str(
            r#"
            </div>
        </div>
"#,
        );
    }

    html.push_str(
        r#"
        <div class="section">
            <div class="section-title">📋 Request Details</div>
            <table>
                <thead>
                    <tr>
                        <th class="col-idx">#</th>
                        <th class="col-method">Method</th>
                        <th class="col-path">Path</th>"#,
    );

    for target in &data.targets {
        let _ = write!(
            html,
            r#"
                        <th class="col-target">{}</th>"#,
            escape_html(target)
        );
    }

    if data.comparison_mode {
        html.push_str(
            r#"
                        <th>Diff</th>"#,
        );
    }

    html.push_str(
        r#"
                    </tr>
                </thead>
                <tbody>"#,
    );

    for r in &data.results {
        let _ = write!(
            html,
            r#"
                    <tr>
                        <td>{}</td>
                        <td><span class="code">{}</span></td>
                        <td><span class="code">{}</span></td>"#,
            r.index,
            escape_html(&r.request.method),
            escape_html(&format_path_owned(&r.request.path))
        );

        for (target, response) in &r.responses {
            let _ = target;
            if let Some(status) = response.status {
                let _ = write!(
                    html,
                    r#"
                        <td>
                            <span class="status-badge status-{}">
                                {}
                            </span>
                            <br><small>{}ms</small>
                        </td>"#,
                    status_color(Some(status)),
                    status,
                    response.latency_ms
                );
            } else {
                let _ = write!(
                    html,
                    r#"
                        <td>
                            <span class="status-badge status-error">ERR</span>
                            <br><small>{}ms</small>
                        </td>"#,
                    response.latency_ms
                );
            }
        }

        if data.comparison_mode {
            html.push_str(
                r#"
                        <td>"#,
            );
            if r.diff.is_some() {
                html.push_str(
                    r#"
                            <span class="diff-badge">⚠ DIFF</span>"#,
                );
            }
            html.push_str(
                r#"
                        </td>"#,
            );
        }

        html.push_str(
            r#"
                    </tr>"#,
        );

        if let Some(ref diff) = r.diff {
            html.push_str(
                r#"
                    <tr>
                        <td colspan="100">
                            <div class="diff-section">
                                <div class="diff-title">Mismatch Details</div>"#,
            );

            if diff.status_mismatch {
                html.push_str(
                    r#"
                                <div style="margin-bottom: 1rem;">
                                    <strong>Status Code Mismatch:</strong>"#,
                );
                if let Some(ref status_codes) = diff.status_codes {
                    for (target, status) in status_codes {
                        let _ = write!(
                            html,
                            r#"
                                        <span class="status-badge status-{}" style="margin-left: 0.5rem;">
                                            {}: {}
                                        </span>"#,
                            status_color(Some(*status)),
                            escape_html(target),
                            status
                        );
                    }
                }
                html.push_str(
                    r#"
                                </div>"#,
                );
            }

            if diff.body_mismatch {
                html.push_str(
                    r#"
                                <div><strong>Response Bodies:</strong></div>
                                <div class="diff-grid">"#,
                );
                for (target, response) in &r.responses {
                    let _ = write!(
                        html,
                        r#"
                                    <div class="diff-col">
                                        <div class="diff-col-header">{}</div>
                                        <div class="diff-body">"#,
                        escape_html(target)
                    );
                    match &response.body {
                        Some(body) if !body.is_empty() => {
                            let _ = write!(html, "{}", escape_html(body));
                        }
                        _ => {
                            html.push_str(r#"<span class="empty-body">&lt;empty body&gt;</span>"#);
                        }
                    }
                    html.push_str(
                        r#"</div>
                                    </div>"#,
                    );
                }
                html.push_str(
                    r#"
                                </div>"#,
                );
            }

            html.push_str(
                r#"
                            </div>
                        </td>
                    </tr>"#,
            );
        }
    }

    html.push_str(
        r#"
                </tbody>
            </table>
        </div>
    </div>
</body>
</html>"#,
    );

    html
}

const HTML_HEAD: &str = r#"<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>HTTP Replay Report</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: #f5f7fa;
            color: #2d3748;
            padding: 2rem;
        }
        .container { max-width: 1400px; margin: 0 auto; }

        .header {
            background: white;
            padding: 2rem;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 2rem;
        }
        h1 { color: #1a202c; font-size: 2rem; margin-bottom: 0.5rem; }
        .meta { color: #718096; font-size: 0.9rem; }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .stat-card {
            background: white;
            padding: 1.5rem;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .stat-value { font-size: 2rem; font-weight: bold; margin-bottom: 0.25rem; }
        .stat-label { color: #718096; font-size: 0.875rem; }
        .stat-value.success { color: #48bb78; }
        .stat-value.error { color: #f56565; }
        .stat-value.warning { color: #ed8936; }

        .section {
            background: white;
            padding: 1.5rem;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 2rem;
            overflow-x: auto;
        }
        .section-title {
            font-size: 1.25rem;
            font-weight: 600;
            margin-bottom: 1rem;
            color: #2d3748;
        }

        table {
            width: 100%;
            border-collapse: collapse;
        }
        th, td {
            padding: 1rem;
            text-align: left;
            border-bottom: 1px solid #e2e8f0;
            vertical-align: top;
        }
        th {
            background: #f7fafc;
            font-weight: 600;
            color: #4a5568;
            font-size: 0.875rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            white-space: nowrap;
        }
        tr:hover { background: #f7fafc; }

        .col-idx { width: 50px; }
        .col-method { width: 80px; }
        .col-path { min-width: 200px; }
        .col-target { min-width: 150px; }

        .status-badge {
            display: inline-block;
            padding: 0.25rem 0.75rem;
            border-radius: 9999px;
            font-size: 0.875rem;
            font-weight: 500;
        }
        .status-success { background: #c6f6d5; color: #22543d; }
        .status-warning { background: #feebc8; color: #7c2d12; }
        .status-error { background: #fed7d7; color: #742a2a; }

        .diff-badge {
            background: #fef5e7;
            color: #d97706;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.75rem;
            font-weight: 600;
            white-space: nowrap;
        }

        .target-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 1rem;
            margin-top: 1rem;
        }
        .target-card {
            background: #f7fafc;
            padding: 1rem;
            border-radius: 6px;
            border-left: 4px solid #4299e1;
        }
        .target-name { font-weight: 600; color: #2d3748; margin-bottom: 0.5rem; }
        .latency-row {
            display: flex;
            justify-content: space-between;
            font-size: 0.875rem;
            margin: 0.25rem 0;
        }
        .latency-label { color: #718096; }
        .latency-value { font-weight: 600; }

        .code {
            background: #f7fafc;
            padding: 0.25rem 0.5rem;
            border-radius: 3px;
            font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
            font-size: 0.85rem;
            word-break: break-word;
            display: inline-block;
        }

        .diff-section {
            background: #fffbeb;
            border-left: 4px solid #f59e0b;
            padding: 1rem;
            margin: 0.5rem 0;
            border-radius: 4px;
        }
        .diff-title { font-weight: 600; color: #92400e; margin-bottom: 0.5rem; }

        .diff-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
            gap: 1rem;
            margin-top: 0.5rem;
        }
        .diff-col {
            background: rgba(255,255,255,0.7);
            padding: 0.75rem;
            border: 1px solid #e2e8f0;
            border-radius: 4px;
        }
        .diff-col-header {
            font-weight: bold;
            font-size: 0.8rem;
            color: #718096;
            margin-bottom: 0.5rem;
            text-transform: uppercase;
            border-bottom: 1px solid #edf2f7;
            padding-bottom: 0.25rem;
        }
        .diff-body {
            font-size: 0.85rem;
            color: #2d3748;
            font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
            white-space: pre-wrap;
            word-break: break-all;
            max-height: 300px;
            overflow-y: auto;
        }
        .empty-body { color: #a0aec0; font-style: italic; }
    </style>
</head>
<body>
"#;
