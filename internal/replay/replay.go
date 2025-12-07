package replay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/kx0101/replayer/internal/cli"
	"github.com/kx0101/replayer/internal/models"
	"github.com/kx0101/replayer/internal/progress"
)

func Run(entries []models.LogEntry, args *cli.CliArgs) []models.MultiEnvResult {
	semaphore := make(chan struct{}, args.Concurrency)
	client := &http.Client{
		Timeout: time.Duration(args.Timeout) * time.Millisecond,
	}

	results := make([]models.MultiEnvResult, 0, len(entries))

	var rateLimiter <-chan time.Time
	if args.RateLimit > 0 {
		interval := time.Second / time.Duration(args.RateLimit)
		rateLimiter = time.Tick(interval)
	}

	var pBar *progress.ProgressBar
	if args.ProgressBar && !args.OutputJSON {
		pBar = progress.NewProgressBar(len(entries))
	}

	for i, entry := range entries {
		if rateLimiter != nil {
			<-rateLimiter
		}

		responses := make(map[string]models.ReplayResult)
		var wg sync.WaitGroup
		var mu sync.Mutex

		for _, target := range args.Targets {
			wg.Add(1)
			semaphore <- struct{}{}

			go func(target string, idx int, e models.LogEntry) {
				defer wg.Done()
				defer func() {
					<-semaphore
				}()

				result := replaySingle(idx, e, client, target)

				mu.Lock()
				responses[target] = result
				mu.Unlock()
			}(target, i, entry)
		}

		wg.Wait()

		result := models.MultiEnvResult{
			Index:     i,
			Request:   entry,
			Responses: responses,
		}

		if args.Compare && len(args.Targets) > 1 {
			result.Diff = compareResponses(responses)
		}

		results = append(results, result)

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

func replaySingle(index int, entry models.LogEntry, client *http.Client, target string) models.ReplayResult {
	url := fmt.Sprintf("http://%s%s", target, entry.Path)

	var bodyReader io.Reader
	if len(entry.Body) > 0 && string(entry.Body) != "null" {
		bodyReader = bytes.NewReader(entry.Body)
	}

	req, err := http.NewRequest(entry.Method, url, bodyReader)
	if err != nil {
		errStr := err.Error()
		return models.ReplayResult{
			Index:     index,
			Status:    nil,
			LatencyMs: 0,
			Error:     &errStr,
			Body:      nil,
		}
	}

	for k, v := range entry.Headers {
		req.Header.Set(k, v)
	}

	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	start := time.Now()
	resp, err := client.Do(req)
	latencyMs := time.Since(start).Milliseconds()

	if err != nil {
		errStr := err.Error()
		return models.ReplayResult{
			Index:     index,
			Status:    nil,
			LatencyMs: latencyMs,
			Error:     &errStr,
			Body:      nil,
		}
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		errStr := err.Error()
		return models.ReplayResult{
			Index:     index,
			Status:    nil,
			LatencyMs: latencyMs,
			Error:     &errStr,
			Body:      nil,
		}
	}

	bodyStr := string(bodyBytes)

	return models.ReplayResult{
		Index:     index,
		Status:    &status,
		LatencyMs: latencyMs,
		Error:     nil,
		Body:      &bodyStr,
	}
}

func compareResponses(responses map[string]models.ReplayResult) *models.ResponseDiff {
	if len(responses) < 2 {
		return nil
	}

	diff := &models.ResponseDiff{
		StatusCodes: make(map[string]int),
		BodyDiffs:   make(map[string]string),
		LatencyDiff: make(map[string]int64),
	}

	var firstStatus *int
	var firstBody *string

	for target, result := range responses {
		if result.Status != nil {
			diff.StatusCodes[target] = *result.Status

			if firstStatus == nil {
				firstStatus = result.Status
			} else if *firstStatus != *result.Status {
				diff.StatusMismatch = true
			}
		}

		if result.Body != nil {
			if firstBody == nil {
				firstBody = result.Body
			} else if *firstBody != *result.Body {
				diff.BodyMismatch = true

				bodyPreview := *result.Body
				if len(bodyPreview) > 100 {
					bodyPreview = bodyPreview[:100] + "..."
				}

				diff.BodyDiffs[target] = bodyPreview
			}
		}

		diff.LatencyDiff[target] = result.LatencyMs
	}

	if !diff.StatusMismatch && !diff.BodyMismatch {
		return nil
	}

	return diff
}
