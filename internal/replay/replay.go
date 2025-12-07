package replay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kx0101/replayer/internal/cli"
	"github.com/kx0101/replayer/internal/comparison"
	"github.com/kx0101/replayer/internal/models"
	"github.com/kx0101/replayer/internal/progress"
)

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

	var pBar *progress.ProgressBar
	if args.ProgressBar && !args.OutputJSON {
		pBar = progress.NewProgressBar(len(entries))
	}

	var volatileConfig *comparison.VolatileConfig
	if args.IgnoreVolatile {
		volatileConfig = comparison.ConfigFromFlags(args.IgnoreFields, args.IgnorePatterns)
	}

	for i, entry := range entries {
		if rateLimiter != nil {
			<-rateLimiter
		}

		responses := make(map[string]models.ReplayResult)
		resCh := make(chan struct {
			target string
			res    models.ReplayResult
		}, len(args.Targets))

		var wg sync.WaitGroup
		for _, target := range args.Targets {
			wg.Add(1)
			semaphore <- struct{}{}

			go func(target string) {
				defer wg.Done()
				defer func() { <-semaphore }()

				resCh <- struct {
					target string
					res    models.ReplayResult
				}{target, replaySingle(i, entry, client, target, args)}
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
			Responses: responses,
		}

		if args.Compare && len(args.Targets) > 1 {
			result.Diff = compareResponses(responses, volatileConfig, args.ShowVolatileDiffs)
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

func replaySingle(index int, entry models.LogEntry, client *http.Client, target string, args *cli.CliArgs) models.ReplayResult {
	req, err := buildRequest(entry, target, args)
	if err != nil {
		return wrapError(index, err, 0)
	}

	body, status, latency, err := doRequest(client, req)
	if err != nil {
		return wrapError(index, err, latency)
	}

	bodyStr := string(body)
	return models.ReplayResult{Index: index, Status: &status, LatencyMs: latency, Body: &bodyStr}
}

func buildRequest(entry models.LogEntry, target string, args *cli.CliArgs) (*http.Request, error) {
	url := fmt.Sprintf("http://%s%s", target, entry.Path)
	var bodyReader io.Reader
	if len(entry.Body) > 0 && string(entry.Body) != "null" {
		bodyReader = bytes.NewReader(entry.Body)
	}

	req, err := http.NewRequest(entry.Method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	for k, v := range entry.Headers {
		req.Header.Set(k, v)
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

	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "go-http-replayer/1.0")
	}

	return req, nil
}

func doRequest(client *http.Client, req *http.Request) (body []byte, statusCode int, latencyMs int64, err error) {
	start := time.Now()
	resp, err := client.Do(req)
	latencyMs = time.Since(start).Milliseconds()

	if err != nil {
		return nil, 0, latencyMs, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, latencyMs, err
	}

	return body, resp.StatusCode, latencyMs, nil
}

func wrapError(index int, err error, latency int64) models.ReplayResult {
	if err == nil {
		return models.ReplayResult{}
	}

	s := err.Error()
	return models.ReplayResult{Index: index, LatencyMs: latency, Error: &s}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}

	return s[:max] + "..."
}

func compareResponses(responses map[string]models.ReplayResult, volatileConfig *comparison.VolatileConfig, showVolatileDiffs bool) *models.ResponseDiff {
	if len(responses) < 2 {
		return nil
	}

	diff := &models.ResponseDiff{
		StatusCodes: make(map[string]int),
		BodyDiffs:   make(map[string]string),
		LatencyDiff: make(map[string]int64),
	}

	var firstTarget string
	var firstStatus *int
	var firstBody string
	bodies := make(map[string]string)

	for target, r := range responses {
		if r.Status != nil {
			diff.StatusCodes[target] = *r.Status
			if firstStatus == nil {
				firstStatus = r.Status
				firstTarget = target
			} else if *firstStatus != *r.Status {
				diff.StatusMismatch = true
			}
		}

		if r.Body != nil {
			bodies[target] = *r.Body
			if firstBody == "" {
				firstBody = *r.Body
			}
		}

		diff.LatencyDiff[target] = r.LatencyMs
	}

	volatileOnly := true
	for target, body := range bodies {
		if target == firstTarget {
			continue
		}

		if volatileConfig != nil {
			detailedDiff, err := comparison.DetailedCompare(firstBody, body, volatileConfig)
			if err != nil {
				if firstBody != body {
					diff.BodyMismatch = true
					volatileOnly = false
					diff.BodyDiffs[target] = truncate(body, 200)
				}
				continue
			}

			if detailedDiff.StableFieldsDiff {
				diff.BodyMismatch = true
				volatileOnly = false
				diff.BodyDiffs[target] = truncate(body, 200)
				diff.IgnoredFields = detailedDiff.IgnoredFields
			} else if detailedDiff.VolatileOnly {
				diff.BodyMismatch = true
				diff.BodyDiffs[target] = "<volatile-only>"
				diff.IgnoredFields = detailedDiff.IgnoredFields
			}

		} else if firstBody != body {
			diff.BodyMismatch = true
			volatileOnly = false
			diff.BodyDiffs[target] = truncate(body, 200)
		}
	}

	if diff.BodyMismatch && firstTarget != "" {
		diff.BodyDiffs[firstTarget] = truncate(firstBody, 200)
	}

	diff.VolatileOnly = volatileOnly && diff.BodyMismatch

	if (!diff.StatusMismatch && !diff.BodyMismatch) || (diff.VolatileOnly && !showVolatileDiffs) {
		return nil
	}

	return diff
}
