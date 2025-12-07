package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"
)

func main() {
	output := flag.String("output", "test_logs.json", "Output file path")
	count := flag.Int("count", 100, "Number of log entries to generate")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	file, err := os.Create(*output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	requestTypes := []struct {
		weight int
		gen    func() map[string]any
	}{
		{40, generateGetUser},
		{20, generateCheckout},
		{30, generateStatus},
		{10, generateSlow},
	}

	totalWeight := 0
	for _, rt := range requestTypes {
		totalWeight += rt.weight
	}

	for i := 0; i < *count; i++ {
		roll := rand.Intn(totalWeight)
		cumulative := 0
		var entry map[string]any

		for _, rt := range requestTypes {
			cumulative += rt.weight
			if roll < cumulative {
				entry = rt.gen()
				break
			}
		}

		data, _ := json.Marshal(entry)
		file.Write(data)
		file.Write([]byte("\n"))
	}

	fmt.Printf("Generated %d log entries to %s\n", *count, *output)
}

func generateGetUser() map[string]any {
	userID := rand.Intn(1000) + 1
	return map[string]any{
		"method": "GET",
		"path":   fmt.Sprintf("/users/%d", userID),
		"headers": map[string]string{
			"Accept": "application/json",
		},
		"body": nil,
	}
}

func generateCheckout() map[string]any {
	numItems := rand.Intn(5) + 1
	items := make([]int, numItems)
	for i := 0; i < numItems; i++ {
		items[i] = rand.Intn(1000) + 1
	}

	return map[string]any{
		"method": "POST",
		"path":   "/checkout",
		"headers": map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		"body": map[string]any{
			"user_id": rand.Intn(1000) + 1,
			"items":   items,
		},
	}
}

func generateStatus() map[string]any {
	return map[string]any{
		"method": "GET",
		"path":   "/status",
		"headers": map[string]string{
			"Accept": "application/json",
		},
		"body": nil,
	}
}

func generateSlow() map[string]any {
	return map[string]any{
		"method": "GET",
		"path":   "/slow",
		"headers": map[string]string{
			"Accept": "application/json",
		},
		"body": nil,
	}
}
