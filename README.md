# Replayer

An HTTP request replay and comparison tool written in Go. Perfect for testing API changes, comparing environments, load testing, validating migrations, and generating detailed reports

## Features

### Core
- **Replay HTTP requests** from JSON log files
- **Multi-target support** - test multiple environments simultaneously
- **Concurrent execution** with configurable limits
- **Smart filtering** by method, path, and limits
- **Ignore rules** for skipping noisy or irrelevant fields during diffing
- **Regression rules**: to automatically fail when behavioral or performance regressions are detected

### Performance & Load
- **Rate limiting** - control requests per second
- **Configurable timeouts** and delays
- **Real-time progress tracking** with ETA
- **Detailed latency statistics** (p50, p90, p95, p99, min, max, avg)

### Response Comparison
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

### Output
- **Colorized console output** for easy reading
- **JSON output** for programmatic use and CI/CD
- **HTML reports** with executive summary, latency charts, per-target breakdown, and difference highlighting
- **Summary-only mode** for quick overview

### Logs
- **Nginx log conversion** to JSON Lines format (combined/common)
- Supports filtering and replay directly from raw logs
- Fully replayable: captured logs can be replayed or compared after the fact

### Exit Codes

Replayer returns specific exit codes to allow CI/CD pipelines and scripts to react programmatically:

| Exit Code | Meaning                                                            |
| --------- | ------------------------------------------------------------------ |
| 0         | Run completed successfully, no differences or errors               |
| 1         | Differences detected between targets (used with `--compare`)       |
| 2         | One or more regression rules were violated                         |
| 3         | Invalid arguments or command-line usage                            |
| 4         | Runtime error occurred (network, file I/O, or unexpected failure)  |

## 🚀 Quick Start

### Installation

```bash
# Clone the repository
git clone <repo-url>
cd replayer

# Build all components
make build

make demo
```

Once it's finished, the demo.html will open up on your browser

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

cat results.json | jq '.summary.succeeded'
```

### Live Capture Mode

Capture requests in real-time from a running service or proxy and replay/compare them on the fly

```bash
# HTTP capture
./replayer --capture \
  --listen :8080 \
  --upstream http://staging.api \
  --output traffic.json \
  --stream

# HTTPS capture
./replayer --capture \
  --listen :8080 \
  --upstream https://staging.api \
  --output traffic.json \
  --stream \
  --tls-cert proxy.crt \
  --tls-key proxy.key

# Replay captured traffic
./replayer --input-file traffic.json staging.api

# Compare captured traffic between two environments
./replayer --input-file traffic.json --compare staging.api production.api
```

When you finish capturing you may use the generated `traffic.json` file to replay or compare as usual

### Regression Rules (Contract & Performance)

Declare regression rules via a yaml file. Replayer allows you to fail runs automatically when behavioral or performance regressions are detected

```bash
./replayer \
  --input-file traffic.json \
  --compare \
  --rules rules.yaml \
  staging.api \
  production.api
```

If any rule is violated the run **fails** and violations are reported

**rules.yaml** example
```yaml
rules:
  status_mismatch:
    max: 0

  body_diff:
    allowed: false
    ignore:
      - "*.timestamp"
      - "request_id"

  latency:
    metric: p95
    regression_percent: 20

  endpoint_rules:
    - path: /users
      method: GET
      status_mismatch:
        max: 0

    - path: /slow
      latency:
        metric: p95
        regression_percent: 10
```

- **Status**: fails if response status differ
- **Body**: exact fields, or prefix/suffix wildcards
- **Latency**: you need a baseline for this (available metrics: min, max, avg, p50, p90, p95, p99)

Example:

```bash
./replayer \
  --input-file traffic.json \
  --compare \
  --output-json \
  staging.api production.api > baseline.json

./replayer \
  --input-file traffic.json \
  --compare \
  --rules rules.yaml \
  --baseline baseline.json \
  staging.api production.api
```

### Dry Run Mode

Preview what will be replayed without sending requests:

```bash
./replayer --input-file test_logs.json --dry-run
```

### Cloud Upload

Upload replay results to Replayer Cloud for tracking, comparison, and team collaboration:

```bash
# set your API key (or use --cloud-api-key flag)
export REPLAYER_API_KEY="rp_your_api_key_here"

# upload results to cloud
./replayer \
  --input-file traffic.json \
  --compare \
  --cloud \
  --cloud-env production \
  --cloud-label "version=v1.2.3" \
  --cloud-label "branch=main" \
  staging.api \
  production.api

**What you get:**
- Historical tracking of all replay runs
- Web UI for viewing results and diffs
- Baseline comparison across runs
- Team collaboration with shared results

## Replayer Cloud

Replayer Cloud is a self-hosted SaaS platform for storing, comparing, and analyzing replay results with:

- Register, login, email verification
- Generate API keys for CLI access
- Browse and search all replay runs
- Set baselines and compare new runs
- Organize runs by environment

### Running Replayer Cloud

```bash
export DATABASE_URL="postgres://user:pass@localhost/replayer?sslmode=disable"
export SESSION_SECRET="your-32-character-secret-key-here"
export BASE_URL="http://localhost:8090"
go run ./cmd/server
```

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `SESSION_SECRET` | Yes | 32+ character secret for session encryption |
| `BASE_URL` | Yes | Public URL for email links |
| `SECURE_COOKIES` | No | Set to `true` in production (default: false) |
| `SMTP_HOST` | No | SMTP server host for email verification |
| `SMTP_PORT` | No | SMTP server port (default: 587) |
| `SMTP_USER` | No | SMTP username |
| `SMTP_PASSWORD` | No | SMTP password |
| `SMTP_FROM` | No | From address for emails |

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/runs` | Upload a new run |
| GET | `/api/v1/runs` | List runs (paginated) |
| GET | `/api/v1/runs/{id}` | Get run details |
| POST | `/api/v1/runs/{id}/baseline` | Set run as baseline |
| GET | `/api/v1/compare/{id}` | Compare run with baseline |

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
| `--capture` | | | Enable live capture mode |
| `--listen` | string | "" | Port to listen for incoming requests |
| `--upstream` | string | "" | URL of the real service to forward requests to |
| `--output` | string | "" | Path to save captured requests in JSON format |
| `--stream` | | | Optionally stream captured requests to stdout as they happen |
| `--tls-cert` | string | "" | TLS certification |
| `--tls-key` | string | "" | TLS key |
| `--rules` | string | "" | Path to rules.yaml file for regression testing |
| `--baseline` | string | "" | Path to baseline results JSON for comparison |
| `--cloud` | bool | false | Upload results to Replayer Cloud |
| `--cloud-url` | string | `$REPLAYER_CLOUD_URL` or `http://localhost:8090` | Replayer Cloud server URL |
| `--cloud-api-key` | string | `$REPLAYER_API_KEY` | API key for cloud authentication |
| `--cloud-env` | string | "default" | Environment name for cloud upload |
| `--cloud-label` | string | "" | Label in `key=value` format (repeatable) |

## Log File Format

- Each line is a single JSON object (JSON Lines)
- Request/response bodies are base64-encoded
- Headers are arrays to support multiple values per key

```json
{"timestamp":"2025-12-10T17:12:48.377+02:00","method":"POST","path":"/test","headers":{"Content-Type":["application/json"]},"body":"SGVsbG8gd29ybGQ=","status":200,"response_headers":{"Content-Type":["application/json"]},"response_body":"eyJzdWNjZXNzIjp0cnVlfQ==","latency_ms":12}
```

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

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request

## Show Your Support

If this tool helped you catch bugs or validate deployments, give it a star! ⭐
