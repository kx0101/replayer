package reader

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kx0101/replayer/internal/cli"
	"github.com/kx0101/replayer/internal/models"
)

func ReadEntries(args *cli.CliArgs) ([]models.LogEntry, error) {
	file, err := os.Open(args.InputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return parseEntries(file, args.Limit, false)
}

func DryRun(input string) error {
	file, err := os.Open(input)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	defer file.Close()

	_, err = parseEntries(file, 0, true)
	return err
}

func parseEntries(r io.Reader, limit int, dryRun bool) ([]models.LogEntry, error) {
	scanner := bufio.NewScanner(r)
	var entries []models.LogEntry
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

func parseLine(line string, lineNum int) (models.LogEntry, error) {
	var entry models.LogEntry
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		return models.LogEntry{}, fmt.Errorf("invalid line %d: %w", lineNum, err)
	}

	return entry, nil
}
