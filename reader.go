package main

import (
	"bufio"
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
	scanner := bufio.NewScanner(r)
	var entries []LogEntry
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		entry, err := parseLine(line, lineNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			continue
		}

		if dryRun {
			fmt.Printf("[DRY RUN] - %d: %+v\n", lineNum, entry)
			continue
		}

		entries = append(entries, entry)

		if limit > 0 && len(entries) >= limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return entries, nil
}

func parseLine(line string, lineNum int) (LogEntry, error) {
	var entry LogEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return LogEntry{}, fmt.Errorf("invalid line %d: %w", lineNum, err)
	}

	return entry, nil
}
