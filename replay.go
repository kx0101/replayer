package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

func Run(entries []LogEntry, args *CliArgs) []MultiEnvResult {
	semaphore := make(chan struct{}, args.Concurrency)
	client := &http.Client{Timeout: time.Duration(args.Timeout) * time.Millisecond}

	results := make([]MultiEnvResult, len(entries))
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

	for i, entry := range entries {
		if rateLimiter != nil {
			<-rateLimiter
		}

		responses := make(map[string]ReplayResult)
		resCh := make(chan struct {
			target string
			res    ReplayResult
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
					res    ReplayResult
				}{target, ReplaySingle(i, entry, client, target, args)}
			}(target)
		}

		wg.Wait()
		close(resCh)
		for r := range resCh {
			responses[r.target] = r.res
		}

		result := MultiEnvResult{
			Index:     i,
			Request:   entry,
			Responses: responses,
		}

		if args.Compare && len(args.Targets) > 1 {
			result.Diff = CompareResponses(responses, volatileConfig, args.ShowVolatileDiffs)
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

func ReplaySingle(index int, entry LogEntry, client *http.Client, target string, args *CliArgs) ReplayResult {
	req, err := BuildRequest(entry, target, args)
	if err != nil {
		return WrapError(index, err, 0)
	}

	body, status, latency, err := doRequest(client, req)
	if err != nil {
		return WrapError(index, err, latency)
	}

	bodyStr := string(body)
	return ReplayResult{Index: index, Status: &status, LatencyMs: latency, Body: &bodyStr}
}

func BuildRequest(entry LogEntry, target string, args *CliArgs) (*http.Request, error) {
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

	for k, values := range entry.Headers {
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

func doRequest(client *http.Client, req *http.Request) (body []byte, statusCode int, latencyMs int64, err error) {
	start := time.Now()
	resp, err := client.Do(req)
	latencyMs = time.Since(start).Milliseconds()

	if err != nil {
		return nil, 0, latencyMs, err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			fmt.Printf("Failed to close response body: %v\n", err)
		}
	}()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, latencyMs, err
	}

	return body, resp.StatusCode, latencyMs, nil
}

func WrapError(index int, err error, latency int64) ReplayResult {
	if err == nil {
		return ReplayResult{}
	}

	s := err.Error()
	return ReplayResult{Index: index, LatencyMs: latency, Error: &s}
}

func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}

	return s[:max] + "..."
}

func CompareResponses(responses map[string]ReplayResult, volatileConfig *VolatileConfig, showVolatileDiffs bool) *ResponseDiff {
	if len(responses) < 2 {
		return nil
	}

	diff := &ResponseDiff{
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
			detailedDiff, err := DetailedCompare(firstBody, body, volatileConfig)
			if err != nil {
				if firstBody != body {
					diff.BodyMismatch = true
					volatileOnly = false
					diff.BodyDiffs[target] = Truncate(body, 200)
				}
				continue
			}

			if detailedDiff.StableFieldsDiff {
				diff.BodyMismatch = true
				volatileOnly = false
				diff.BodyDiffs[target] = Truncate(body, 200)
				diff.IgnoredFields = detailedDiff.IgnoredFields
			} else if detailedDiff.VolatileOnly {
				diff.BodyMismatch = true
				diff.BodyDiffs[target] = "<volatile-only>"
				diff.IgnoredFields = detailedDiff.IgnoredFields
			}

		} else if firstBody != body {
			diff.BodyMismatch = true
			volatileOnly = false
			diff.BodyDiffs[target] = Truncate(body, 200)
		}
	}

	if diff.BodyMismatch && firstTarget != "" {
		diff.BodyDiffs[firstTarget] = Truncate(firstBody, 200)
	}

	diff.VolatileOnly = volatileOnly && diff.BodyMismatch

	if (!diff.StatusMismatch && !diff.BodyMismatch) || (diff.VolatileOnly && !showVolatileDiffs) {
		return nil
	}

	return diff
}

func HasDiffs(results []MultiEnvResult) bool {
	for _, r := range results {
		if r.Diff != nil {
			return true
		}
	}

	return false
}
