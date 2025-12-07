package report

import (
	"fmt"
	"html/template"
	"os"
	"slices"
	"time"

	"github.com/kx0101/replayer/internal/cli"
	"github.com/kx0101/replayer/internal/models"
	"github.com/kx0101/replayer/internal/stats"
)

type ReportData struct {
	GeneratedAt    string
	InputFile      string
	Targets        []string
	TotalRequests  int
	Succeeded      int
	Failed         int
	DiffCount      int
	Latency        models.LatencyStats
	ByTarget       map[string]models.TargetStats
	Results        []models.MultiEnvResult
	ComparisonMode bool
}

func GenerateHTML(results []models.MultiEnvResult, args *cli.CliArgs, outputPath string) error {
	data := buildReportData(results, args)

	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"statusColor": statusColor,
		"formatPath":  formatPath,
	}).Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func buildReportData(results []models.MultiEnvResult, args *cli.CliArgs) ReportData {
	var totalRequests, succeeded, failed, diffCount int
	var latencies []int64
	targetStats := map[string]*models.TargetStats{}

	if len(results) > 0 {
		for target := range results[0].Responses {
			targetStats[target] = &models.TargetStats{}
		}
	}

	for _, r := range results {
		for target, replay := range r.Responses {
			totalRequests++
			ts := targetStats[target]

			if replay.Status != nil && *replay.Status < 400 {
				succeeded++
				ts.Succeeded++
			} else {
				failed++
				ts.Failed++
			}
			latencies = append(latencies, replay.LatencyMs)
		}

		if r.Diff != nil {
			diffCount++
		}
	}

	slices.Sort(latencies)
	overallLatency := stats.CalculateLatencyStats(latencies)

	for target, ts := range targetStats {
		var tLat []int64
		for _, r := range results {
			if replay, ok := r.Responses[target]; ok {
				tLat = append(tLat, replay.LatencyMs)
			}
		}

		ts.Latency = stats.CalculateLatencyStats(tLat)
	}

	byTarget := map[string]models.TargetStats{}
	for k, v := range targetStats {
		byTarget[k] = *v
	}

	return ReportData{
		GeneratedAt:    time.Now().Format("2006-01-02 15:04:05"),
		InputFile:      args.InputFile,
		Targets:        args.Targets,
		TotalRequests:  totalRequests,
		Succeeded:      succeeded,
		Failed:         failed,
		DiffCount:      diffCount,
		Latency:        overallLatency,
		ByTarget:       byTarget,
		Results:        results,
		ComparisonMode: args.Compare,
	}
}

func statusColor(v any) string {
	var status int
	switch val := v.(type) {
	case *int:
		if val == nil {
			return "error"
		}
		status = *val
	case int:
		status = val
	default:
		return "error"
	}

	switch {
	case status < 400:
		return "success"
	case status < 500:
		return "warning"
	default:
		return "error"
	}
}

func formatPath(path string) string {
	if len(path) > 50 {
		return path[:47] + "..."
	}

	return path
}

const htmlTemplate = `<!DOCTYPE html>
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
            grid-template-columns: repeat(auto-fit, minmax(400px, 1fr)); /* Force wider columns for diffs */
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
    <div class="container">
        <div class="header">
            <h1>🔄 HTTP Replay Report</h1>
            <div class="meta">
                Generated: {{.GeneratedAt}} | Input: {{.InputFile}}
                {{if .ComparisonMode}}| Mode: Comparison{{end}}
            </div>
        </div>

        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value">{{.TotalRequests}}</div>
                <div class="stat-label">Total Requests</div>
            </div>
            <div class="stat-card">
                <div class="stat-value success">{{.Succeeded}}</div>
                <div class="stat-label">Succeeded</div>
            </div>
            <div class="stat-card">
                <div class="stat-value error">{{.Failed}}</div>
                <div class="stat-label">Failed</div>
            </div>
            {{if .ComparisonMode}}
            <div class="stat-card">
                <div class="stat-value warning">{{.DiffCount}}</div>
                <div class="stat-label">Differences Found</div>
            </div>
            {{end}}
        </div>

        <div class="section">
            <div class="section-title">⚡ Overall Latency Statistics</div>
            <div class="latency-row">
                <span class="latency-label">Minimum:</span>
                <span class="latency-value">{{.Latency.Min}}ms</span>
            </div>
            <div class="latency-row">
                <span class="latency-label">Average:</span>
                <span class="latency-value">{{.Latency.Avg}}ms</span>
            </div>
            <div class="latency-row">
                <span class="latency-label">p50 (Median):</span>
                <span class="latency-value">{{.Latency.P50}}ms</span>
            </div>
            <div class="latency-row">
                <span class="latency-label">p90:</span>
                <span class="latency-value">{{.Latency.P90}}ms</span>
            </div>
            <div class="latency-row">
                <span class="latency-label">p95:</span>
                <span class="latency-value">{{.Latency.P95}}ms</span>
            </div>
            <div class="latency-row">
                <span class="latency-label">p99:</span>
                <span class="latency-value">{{.Latency.P99}}ms</span>
            </div>
            <div class="latency-row">
                <span class="latency-label">Maximum:</span>
                <span class="latency-value">{{.Latency.Max}}ms</span>
            </div>
        </div>

        {{if gt (len .ByTarget) 1}}
        <div class="section">
            <div class="section-title">🎯 Per-Target Statistics</div>
            <div class="target-grid">
                {{range $target, $stats := .ByTarget}}
                <div class="target-card">
                    <div class="target-name">{{$target}}</div>
                    <div class="latency-row">
                        <span class="latency-label">Succeeded:</span>
                        <span class="latency-value">{{$stats.Succeeded}}</span>
                    </div>
                    <div class="latency-row">
                        <span class="latency-label">Failed:</span>
                        <span class="latency-value">{{$stats.Failed}}</span>
                    </div>
                    <div class="latency-row">
                        <span class="latency-label">Avg Latency:</span>
                        <span class="latency-value">{{$stats.Latency.Avg}}ms</span>
                    </div>
                    <div class="latency-row">
                        <span class="latency-label">p95:</span>
                        <span class="latency-value">{{$stats.Latency.P95}}ms</span>
                    </div>
                </div>
                {{end}}
            </div>
        </div>
        {{end}}

        <div class="section">
            <div class="section-title">📋 Request Details</div>
            <table>
                <thead>
                    <tr>
                        <th class="col-idx">#</th>
                        <th class="col-method">Method</th>
                        <th class="col-path">Path</th>
                        {{range .Targets}}
                        <th class="col-target">{{.}}</th>
                        {{end}}
                        {{if .ComparisonMode}}<th>Diff</th>{{end}}
                    </tr>
                </thead>
                <tbody>
                    {{range .Results}}
                    <tr>
                        <td>{{.Index}}</td>
                        <td><span class="code">{{.Request.Method}}</span></td>
                        <td><span class="code">{{formatPath .Request.Path}}</span></td>
                        {{range $target, $response := .Responses}}
                        <td>
                            {{if $response.Status}}
                            <span class="status-badge status-{{statusColor $response.Status}}">
                                {{$response.Status}}
                            </span>
                            {{else}}
                            <span class="status-badge status-error">ERR</span>
                            {{end}}
                            <br><small>{{$response.LatencyMs}}ms</small>
                        </td>
                        {{end}}
                        {{if $.ComparisonMode}}
                        <td>
                            {{if .Diff}}
                            <span class="diff-badge">⚠ DIFF</span>
                            {{end}}
                        </td>
                        {{end}}
                    </tr>
                    {{if .Diff}}
                    <tr>
                        <td colspan="100">
                            <div class="diff-section">
                                <div class="diff-title">Mismatch Details</div>
                                
                                {{if .Diff.StatusMismatch}}
                                <div style="margin-bottom: 1rem;">
                                    <strong>Status Code Mismatch:</strong> 
                                    {{range $target, $status := .Diff.StatusCodes}}
                                        <span class="status-badge status-{{statusColor $status}}" style="margin-left: 0.5rem;">
                                            {{$target}}: {{$status}}
                                        </span>
                                    {{end}}
                                </div>
                                {{end}}

                                {{if .Diff.BodyMismatch}}
                                <div><strong>Response Bodies:</strong></div>
                                <div class="diff-grid">
                                    {{range $target, $response := .Responses}}
                                    <div class="diff-col">
                                        <div class="diff-col-header">{{$target}}</div>
                                        <div class="diff-body">{{if $response.Body}}{{$response.Body}}{{else}}<span class="empty-body">&lt;empty body&gt;</span>{{end}}</div>
                                    </div>
                                    {{end}}
                                </div>
                                {{end}}
                            </div>
                        </td>
                    </tr>
                    {{end}}
                    {{end}}
                </tbody>
            </table>
        </div>
    </div>
</body>
</html>`
