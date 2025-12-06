package replay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/kx0101/replayer/cli"
	"github.com/kx0101/replayer/models"
)

func Run(entries []models.LogEntry, args *cli.CliArgs) []models.MultiEnvResult {
	semaphore := make(chan struct{}, args.Concurrency)
	client := &http.Client{
		Timeout: time.Duration(args.Timeout) * time.Millisecond,
	}

	results := make([]models.MultiEnvResult, 0, len(entries))

	for i, entry := range entries {
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

		results = append(results, models.MultiEnvResult{
			Index:     i,
			Request:   entry,
			Responses: responses,
		})
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
	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	return models.ReplayResult{
		Index:     index,
		Status:    &status,
		LatencyMs: latencyMs,
		Error:     nil,
		Body:      &bodyStr,
	}
}
