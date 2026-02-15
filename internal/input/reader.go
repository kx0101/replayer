package input

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

func parseEntries(r io.Reader, limit int, dryRun bool) ([]models.LogEntry, error) {
	scanner := bufio.NewScanner(r)
	var entries []models.LogEntry
	lineNum := 0

	for scanner.Scan() {
		if limit > 0 && len(entries) >= limit {
			break
		}

		line := scanner.Text()
		lineNum++

		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry models.LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			fmt.Fprintf(os.Stderr, "invalid JSON object %d: %v\n", lineNum, err)
			continue
		}

		if dryRun {
			fmt.Printf("[DRY RUN] - %d: %+v\n", lineNum, entry)
			continue
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}
