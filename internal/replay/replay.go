package replay

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kx0101/replayer/internal/cli"
	"github.com/kx0101/replayer/internal/models"
)

const latencyBucketMs int64 = 5

func Run(entries []models.LogEntry, args *cli.CliArgs) []models.MultiEnvResult {
	semaphore := make(chan struct{}, args.Concurrency)
	client := &http.Client{Timeout: time.Duration(args.Timeout) * time.Millisecond}

	results := make([]models.MultiEnvResult, len(entries))

	var rateLimiter <-chan time.Time
	if args.RateLimit > 0 {
		ticker := time.NewTicker(time.Second / time.Duration(args.RateLimit))
		defer ticker.Stop()
		rateLimiter = ticker.C
	}

	var pBar *ProgressBar
	if args.ProgressBar && !args.OutputJSON {
		pBar = NewProgressBar(len(entries))
	}

	var volatileConfig *VolatileConfig
	if args.IgnoreVolatile {
		volatileConfig = ConfigFromFlags(args.IgnoreFields, args.IgnorePatterns)
	}

	targets := append([]string{}, args.Targets...)
	sort.Strings(targets)

	for i, entry := range entries {
		if rateLimiter != nil {
			<-rateLimiter
		}

		responses := make(map[string]models.ReplayResult, len(targets))
		resCh := make(chan struct {
			target string
			res    models.ReplayResult
		}, len(targets))

		var wg sync.WaitGroup
		for _, target := range targets {
			wg.Add(1)
			semaphore <- struct{}{}

			go func(target string) {
				defer wg.Done()
				defer func() { <-semaphore }()

				res := ReplaySingle(i, entry, client, target, args)
				resCh <- struct {
					target string
					res    models.ReplayResult
				}{target, res}
			}(target)
		}

		wg.Wait()
		close(resCh)

		for r := range resCh {
			responses[r.target] = r.res
		}

		result := models.MultiEnvResult{
			Index:     i,
			Request:   entry,
			RequestID: Fingerprint(entry),
			Responses: responses,
		}

		if args.Compare && len(targets) > 1 {
			result.Diff = CompareResponsesDeterministic(
				responses,
				targets,
				volatileConfig,
				args.ShowVolatileDiffs,
			)
		}

		results[i] = result

		if pBar != nil {
			pBar.Increment()
		}

		if args.Delay > 0 {
			time.Sleep(time.Duration(args.Delay) * time.Millisecond)
		}
	}

	if pBar != nil {
		pBar.Finish()
	}

	return results
}

func ReplaySingle(index int, entry models.LogEntry, client *http.Client, target string, args *cli.CliArgs) models.ReplayResult {
	req, err := BuildRequest(entry, target, args)
	if err != nil {
		return WrapError(index, err, 0)
	}

	body, status, latency, err := doRequest(client, req)
	if err != nil {
		return WrapError(index, err, latency)
	}

	latency = normalizeLatency(latency)
	bodyStr := string(body)

	return models.ReplayResult{
		Index:     index,
		Status:    &status,
		LatencyMs: latency,
		Body:      &bodyStr,
	}
}

func BuildRequest(entry models.LogEntry, target string, args *cli.CliArgs) (*http.Request, error) {
	scheme := "http"
	if args.TLSCert != "" && args.TLSKey != "" {
		scheme = "https"
	}

	url := fmt.Sprintf("%s://%s%s", scheme, target, entry.Path)

	var r io.Reader
	if entry.Body != "" && entry.Body != "null" {
		b, err := base64.StdEncoding.DecodeString(entry.Body)
		if err != nil {
			b = []byte(entry.Body)
		}

		r = bytes.NewReader(b)
	}

	req, err := http.NewRequest(entry.Method, url, r)
	if err != nil {
		return nil, err
	}

	headerKeys := make([]string, 0, len(entry.Headers))
	for k := range entry.Headers {
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)

	for _, k := range headerKeys {
		values := append([]string{}, entry.Headers[k]...)
		sort.Strings(values)
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	if args.AuthHeader != "" {
		req.Header.Set("Authorization", args.AuthHeader)
	}

	for _, h := range args.Headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	if r != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "go-http-replayer/1.0")
	}

	return req, nil
}

func doRequest(client *http.Client, req *http.Request) ([]byte, int, int64, error) {
	start := time.Now()
	resp, err := client.Do(req) //#nosec G704 -- Target URLs are user-configured replay targets, SSRF is intentional
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		return nil, 0, latencyMs, err
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			return
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, latencyMs, err
	}

	return body, resp.StatusCode, latencyMs, nil
}

func normalizeLatency(ms int64) int64 {
	return (ms / latencyBucketMs) * latencyBucketMs
}

func Fingerprint(entry models.LogEntry) string {
	h := sha256.New()
	h.Write([]byte(entry.Method))
	h.Write([]byte(entry.Path))
	h.Write([]byte(entry.Body))

	keys := make([]string, 0, len(entry.Headers))
	for k := range entry.Headers {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		values := append([]string{}, entry.Headers[k]...)
		sort.Strings(values)

		for _, v := range values {
			h.Write([]byte(k))
			h.Write([]byte(v))
		}
	}

	return hex.EncodeToString(h.Sum(nil))[:16]
}

func WrapError(index int, err error, latency int64) models.ReplayResult {
	if err == nil {
		return models.ReplayResult{}
	}

	s := err.Error()
	return models.ReplayResult{
		Index:     index,
		LatencyMs: normalizeLatency(latency),
		Error:     &s,
	}
}

func CompareResponsesDeterministic(
	responses map[string]models.ReplayResult,
	targets []string,
	volatileConfig *VolatileConfig,
	showVolatileDiffs bool,
) *models.ResponseDiff {

	if len(targets) < 2 {
		return nil
	}

	baseline := targets[0]
	base := responses[baseline]

	diff := &models.ResponseDiff{
		StatusCodes: make(map[string]int),
		BodyDiffs:   make(map[string]string),
		LatencyDiff: make(map[string]int64),
	}

	baseBody := deref(base.Body)

	for _, target := range targets {
		r := responses[target]

		if r.Status != nil {
			diff.StatusCodes[target] = *r.Status
			if base.Status != nil && *r.Status != *base.Status {
				diff.StatusMismatch = true
			}
		}

		diff.LatencyDiff[target] = r.LatencyMs
	}

	volatileOnly := true

	for _, target := range targets[1:] {
		r := responses[target]
		body := deref(r.Body)

		if volatileConfig != nil {
			d, err := DetailedCompare(baseBody, body, volatileConfig)
			if err != nil || d.StableFieldsDiff {
				diff.BodyMismatch = true
				volatileOnly = false
				diff.BodyDiffs[target] = Truncate(body, 200)
			} else if d.VolatileOnly {
				diff.BodyMismatch = true
				diff.BodyDiffs[target] = "<volatile-only>"
			}
		} else if baseBody != body {
			diff.BodyMismatch = true
			volatileOnly = false
			diff.BodyDiffs[target] = Truncate(body, 200)
		}
	}

	if diff.BodyMismatch {
		diff.BodyDiffs[baseline] = Truncate(baseBody, 200)
	}

	diff.VolatileOnly = volatileOnly && diff.BodyMismatch

	if (!diff.StatusMismatch && !diff.BodyMismatch) || (diff.VolatileOnly && !showVolatileDiffs) {
		return nil
	}

	return diff
}

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}

	return s[:max] + "..."
}

func HasDiffs(results []models.MultiEnvResult) bool {
	for _, r := range results {
		if r.Diff != nil {
			return true
		}
	}

	return false
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
