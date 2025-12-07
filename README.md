# HTTP Replay Tool

An HTTP request replay and comparison tool written in Go. Perfect for testing API changes, comparing environments, load testing, validating migrations, and generating detailed reports

## Key Features

### Core Functionality
- **Replay HTTP requests** from JSON log files
- **Multi-target support** - test multiple environments simultaneously
- **Concurrent execution** with configurable limits
- **Smart filtering** by method, path, and limits
- **Ignore rules** for skipping noisy or irrelevant fields during diffing

### Performance & Load Testing
- **Rate limiting** - control requests per second
- **Configurable timeouts** and delays
- **Real-time progress tracking** with ETA
- **Detailed latency statistics** (p50, p90, p95, p99, min, max, avg)

### Response Comparison & Analysis
- **Automatic diff detection** between targets
- **Status code mismatch** reporting
- **Response body comparison**
- **Latency comparison** across targets
- **Per-target statistics** breakdown
- **Ignore fields** during comparison

### Authentication & Headers

- **Bearer token authentication**
- **Custom API headers** (repeatable)
- **Supports multiple headers simultaneously**

### Output Formats
- **Colorized console output** for easy reading
- **JSON output** for programmatic use and CI/CD
- **HTML reports** with executive summary, latency charts, per-target breakdown, and difference highlighting
- **Summary-only mode** for quick overview

### Log Parsing
- **Nginx log conversion** to JSON Lines format (combined/common)
- Supports filtering and replay directly from raw logs

## 🚀 Quick Start

### Installation

```bash
# Clone the repository
git clone <repo-url>
cd replayer

# Build all components
go build -o replayer .
go build -o mock-server cmd/mock-server/mock-server.go
go build -o mock-server-v2 cmd/mock-server/mock-server-v2.go
go build -o generate-logs cmd/generate-logs/generate-logs.go
go build -o nginx-converter cmd/nginx-converter/nginx-converter.go
```

### 5-Minute Demo

```bash
make build
make clean
make install
make demo
```

Once it's finished, the report.html will open up on your browser

## Usage Guide

### Basic Replay

Replay requests against a single target:

```bash
./replayer --input-file test_logs.json --concurrency 5 localhost:8080
```

### Compare Staging vs Production

The killer feature - compare two environments side-by-side:

```bash
./replayer \
  --input-file prod_logs.json \
  --compare \
  --concurrency 10 \
  staging.example.com \
  production.example.com
```

### Load Testing

Simulate realistic load patterns:

```bash
./replayer \
  --input-file logs.json \
  --rate-limit 1000 \
  --concurrency 50 \
  --timeout 10000 \
  localhost:8080
```

### Authentication & Custom Headers

Provide auth token or custom headers:

```bash
# Bearer token
./replayer --input-file logs.json --auth "Bearer token123" api.example.com

# Custom headers
./replayer --input-file logs.json --header "X-API-Key: abc" --header "X-Env: staging" api.example.com
```

### HTML Report Generation

```bash
# Single target
./replayer --input-file logs.json --html-report report.html localhost:8080

# Comparison mode
./replayer --input-file logs.json --compare --html-report comparison_report.html staging.api production.api
```

### Nginx Log Parsing

```bash
# Convert nginx logs to JSON Lines
./replayer --input-file /var/log/nginx/access.log --parse-nginx traffic.json --nginx-format combined

# Replay converted logs
./replayer --input-file traffic.json --concurrency 10 staging.api.com
```

### Filter Specific Requests

Test only certain endpoints:

```bash
# Only replay POST requests to /checkout
./replayer \
  --input-file test_logs.json \
  --filter-method POST \
  --filter-path /checkout \
  --limit 100 \
  localhost:8080
```

### Ignore Rules

Ignore specific JSON fields when comparing responses

| Type | Example |
|------|------|
| Exact field | `--ignore status.updated_at` |
| Wildcard | `--ignore '*.timestamp'` |
| Multiple fields | `--ignore x --ignore y --ignore z`|

```bash
# Ignore timestamps, request IDs, metadata
./replayer \
  --input-file logs.json \
  --compare \
  --ignore "*.timestamp" \
  --ignore "request_id" \
  --ignore "metadata.*" \
  staging.api prod.api

# Ignore an entire object subtree
--ignore "debug_info"
```

### JSON Output for Automation

Perfect for CI/CD pipelines:

```bash
./replayer \
  --input-file test_logs.json \
  --output-json \
  --compare \
  staging.api \
  production.api > results.json

# Process with jq
cat results.json | jq '.summary.succeeded'
```

### Dry Run Mode

Preview what will be replayed without sending requests:

```bash
./replayer --input-file test_logs.json --dry-run
```

## Command-Line Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--input-file` | string | **required** | Path to the input log file |
| `--concurrency` | int | 1 | Number of concurrent requests |
| `--timeout` | int | 5000 | Request timeout in milliseconds |
| `--delay` | int | 0 | Delay between requests in milliseconds |
| `--rate-limit` | int | 0 | Maximum requests per second (0 = unlimited) |
| `--limit` | int | 0 | Limit number of requests to replay (0 = all) |
| `--filter-method` | string | "" | Filter by HTTP method (GET, POST, etc.) |
| `--filter-path` | string | "" | Filter by path substring |
| `--compare` | bool | false | Compare responses between targets |
| `--output-json` | bool | false | Output results as JSON |
| `--progress` | bool | true | Show progress bar |
| `--dry-run` | bool | false | Preview mode - don't send requests |
| `--summary-only` | bool | false | Output summary only |
| `--auth` | string | "" | Authorization header value |
| `--header` | string | "" | Custom header (repeatable) |
| `--html-report` | string | "" | Generate HTML report |
| `--parse-nginx` | string | "" | Convert nginx log to JSON Lines |
| `--nginx-format` | string | "combined" | Nginx format: combined/common |
| `--ignore` | string | "" | Ignore fields during diff (repeatable) |

## Log File Format

The tool expects **JSON Lines** format (one JSON object per line):

```json
{"method":"GET","path":"/users/123","headers":{"Content-Type":"application/json"},"body":null}
{"method":"POST","path":"/checkout","headers":{"Content-Type":"application/json"},"body":{"user_id":42,"items":[1,2,3]}}
{"method":"GET","path":"/status","headers":{},"body":null}
```

### Generating Test Logs

Use the included log generator:

```bash
./generate_logs --output test.json --count 1000
```

This creates a realistic distribution of requests:
- 40% `GET /users/{id}` 
- 20% `POST /checkout`
- 30% `GET /status`
- 10% `GET /slow` (2-3 second delay)

## Output Examples

### Console Output (Default)

```
[████████████████████████░░░░░░░░░░░░] 150/200 (75.0%) | Elapsed: 15s | ETA: 5s

[0][localhost:8080] 200 -> 45ms
[0][localhost:8081] 200 -> 47ms

[12][localhost:8080] 200 -> 5ms
[12][localhost:8081] 200 -> 6ms
  [DIFF] Request 12 - GET /users/42:
    Response bodies differ:
      localhost:8080: {"id":42,"name":"Liakos koulaxis"}
      localhost:8081: {"id":42,"name":"Liakos Koulaxis Jr.","version":"v2"}

[45][localhost:8080] 200 -> 3ms
[45][localhost:8081] 404 -> 2ms
  [DIFF] Request 45 - GET /users/678:
    Status codes differ: localhost:8080=200 localhost:8081=404

==== Summary ====
Overall Statistics:
Total Requests: 200
Succeeded:      195
Failed:         5
Differences:    23

Latency (ms):
  min: 2
  avg: 45
  p50: 42
  p90: 78
  p95: 95
  p99: 124
  max: 2001

Per-Target Statistics:

localhost:8080:
  Succeeded: 98
  Failed:    2
  Latency:
    min: 2
    avg: 43
    p50: 40
    p90: 75
    p95: 92
    p99: 120
    max: 2001

localhost:8081:
  Succeeded: 97
  Failed:    3
  Latency:
    min: 2
    avg: 47
    p50: 44
    p90: 81
    p95: 98
    p99: 128
    max: 3002
```

### JSON Output

```json
{
  "results": [
    {
      "index": 0,
      "request": {
        "method": "GET",
        "path": "/users/123",
        "headers": {"Content-Type": "application/json"},
        "body": null
      },
      "responses": {
        "localhost:8080": {
          "index": 0,
          "status": 200,
          "latency_ms": 45,
          "body": "{\"id\":123,\"name\":\"Liakos koulaxis\"}"
        },
        "localhost:8081": {
          "index": 0,
          "status": 200,
          "latency_ms": 47,
          "body": "{\"id\":123,\"name\":\"Liakos koulaxis\",\"version\":\"v2\"}"
        }
      },
      "diff": {
        "status_mismatch": false,
        "body_mismatch": true,
        "body_diffs": {
          "localhost:8080": "{\"id\":123,\"name\":\"Liakos koulaxis\"}",
          "localhost:8081": "{\"id\":123,\"name\":\"Liakos koulaxis\",\"version\":\"v2\"}"
        }
      }
    }
  ],
  "summary": {
    "total_requests": 200,
    "succeeded": 195,
    "failed": 5,
    "latency": {
      "p50": 42,
      "p90": 78,
      "p95": 95,
      "p99": 124,
      "min": 2,
      "max": 2001,
      "avg": 45
    },
    "by_target": {
      "localhost:8080": {
        "succeeded": 98,
        "failed": 2,
        "latency": {...}
      }
    }
  }
}
```

## Real-World Use Cases

### 1. Production to Staging Validation with Auth & HTML Report

**Problem:** Is staging behaving exactly like production?

```bash
# Parse logs
./replayer --input-file prod_traffic.log --parse-nginx prod_traffic.json

# Replay and compare with auth
./replayer \
  --input-file prod_traffic.json \
  --auth "Bearer ${STAGING_TOKEN}" \
  --compare \
  --html-report staging_validation.html \
  --rate-limit 100 \
  staging.api.example.com \
  production.api.example.com
```

**What you get:** Instant visibility into any behavioral differences between environments

### 2. Performance Regression Testing
**Problem:** Did the new version slow down any endpoints?

```bash
./replayer --input-file baseline_traffic.json --compare old-api.com new-api.com
```

**What you get:** Side-by-side latency comparison for every endpoint

### 3. Migration Validation
**Problem:** Can the new infrastructure handle production load?

```bash
./replayer --input-file prod_logs.json --rate-limit 1000 --concurrency 50 new-infra.com
```

**What you get:** Confidence that your new infrastructure can handle real traffic patterns

### 4. API Contract Testing
**Problem:** Did the API response format change?

```bash
./replayer --input-file api_calls.json --compare --output-json v1.api v2.api > diff.json
```

**What you get:** Automated detection of breaking changes

### 5. Load Testing with Real Patterns
**Problem:** Synthetic load tests don't match real usage.

```bash
./replayer --input-file peak_hour_traffic.json --rate-limit 500 api.example.com
```

**What you get:** Load testing based on actual production traffic patterns

## Tips

### Filtering for Specific Scenarios

```bash
# Only test authentication endpoints
./replayer --input-file logs.json --filter-path /auth localhost:8080

# Only test write operations
./replayer --input-file logs.json --filter-method POST localhost:8080

# Test just the first 50 requests
./replayer --input-file logs.json --limit 50 localhost:8080
```

### Rate Limiting Strategies

```bash
# Gentle ramp-up: 10 req/s
./replayer --input-file logs.json --rate-limit 10 --concurrency 5 api.com

# Stress test: 1000 req/s
./replayer --input-file logs.json --rate-limit 1000 --concurrency 100 api.com

# Sustained load test with unlimited rate
./replayer --input-file logs.json --concurrency 50 api.com
```

### CI/CD Integration

```bash
#!/bin/bash
# compare staging to production and fail if differences found

./replayer --input-file smoke_tests.json --compare --output-json \
  staging.api production.api > results.json

DIFFS=$(cat results.json | jq '[.results[] | select(.diff != null)] | length')

if [ "$DIFFS" -gt 0 ]; then
  echo "Found $DIFFS differences between staging and production"
  exit 1
else
  echo "Staging matches production"
fi
```

### Analyzing Results with jq

```bash
# Get all failed requests
cat results.json | jq '.results[] | select(.responses[].status >= 400)'

# Get average latency per target
cat results.json | jq '.summary.by_target | to_entries[] | {target: .key, avg_latency: .value.latency.avg}'

# Find slowest requests
cat results.json | jq '[.results[] | {index, path: .request.path, max_latency: [.responses[].latency_ms] | max}] | sort_by(.max_latency) | reverse | .[0:10]'
```

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request

## Show Your Support

If this tool helped you catch bugs or validate deployments, give it a star! ⭐
