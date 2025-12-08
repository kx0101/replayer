package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func ReadEntries(args *CliArgs) ([]LogEntry, error) {
	file, err := os.Open(args.InputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		err = file.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close file: %v\n", err)
		}
	}()

	return parseEntries(file, args.Limit, false)
}

func DryRun(input string) error {
	if strings.Contains(input, "..") {
		return fmt.Errorf("invalid input path: %s", input)
	}

	file, err := os.Open(input) // #nosec G304
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		err = file.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close file: %v\n", err)
		}
	}()

	_, err = parseEntries(file, 0, true)
	return err
}

func parseEntries(r io.Reader, limit int, dryRun bool) ([]LogEntry, error) {
	decoder := json.NewDecoder(r)
	var entries []LogEntry
	lineNum := 0

	for limit <= 0 || len(entries) < limit {

		var entry LogEntry
		if err := decoder.Decode(&entry); err != nil {
			if err == io.EOF {
				break
			}

			lineNum++
			fmt.Fprintf(os.Stderr, "invalid JSON object %d: %v\n", lineNum, err)
			continue
		}

		lineNum++

		if dryRun {
			fmt.Printf("[DRY RUN] - %d: %+v\n", lineNum, entry)
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}
