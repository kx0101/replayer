package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/kx0101/replayer/internal/models"
)

var (
	// Combined log format: 127.0.0.1 - - [07/Dec/2024:10:15:30 +0000] "GET /users/123 HTTP/1.1" 200 1234 "http://example.com" "Mozilla/5.0"
	combinedLogRegex = regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) (\S+) \S+" (\d+) (\d+) "([^"]*)" "([^"]*)"`)

	// Common log format: 127.0.0.1 - - [07/Dec/2024:10:15:30 +0000] "GET /users/123 HTTP/1.1" 200 1234
	commonLogRegex = regexp.MustCompile(`^(\S+) \S+ \S+ \[([^\]]+)\] "(\S+) (\S+) \S+" (\d+) (\d+)`)
)

type NginxParser struct {
	format string
}

func NewNginxParser(format string) *NginxParser {
	if format == "" {
		format = "combined"
	}

	return &NginxParser{
		format: format,
	}
}

func (p *NginxParser) ParseFile(inputPath, outputPath string) error {
	inFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inFile.Close()

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	scanner := bufio.NewScanner(inFile)
	lineNum := 0
	parsed := 0
	skipped := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if strings.TrimSpace(line) == "" {
			continue
		}

		entry, err := p.parseLine(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping line %d: %v\n", lineNum, err)
			skipped++
			continue
		}

		data, err := json.Marshal(entry)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal line %d: %v\n", lineNum, err)
			skipped++
			continue
		}

		outFile.Write(data)
		outFile.Write([]byte("\n"))
		parsed++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	fmt.Printf("Parsed %d requests, skipped %d invalid lines\n", parsed, skipped)
	return nil
}

func (p *NginxParser) parseLine(line string) (*models.LogEntry, error) {
	var matches []string

	matches = combinedLogRegex.FindStringSubmatch(line)
	if matches == nil {
		matches = commonLogRegex.FindStringSubmatch(line)

		if matches == nil {
			return nil, fmt.Errorf("line does not match nginx log format")
		}
	}

	// matches[0] = full match
	// matches[1] = IP
	// matches[2] = timestamp
	// matches[3] = method
	// matches[4] = path
	// matches[5] = status
	// matches[6] = bytes
	// matches[7] = referer (combined only)
	// matches[8] = user agent (combined only)

	method := matches[3]
	path := matches[4]

	headers := make(map[string]string)

	if len(matches) > 8 {
		if matches[8] != "-" {
			headers["User-Agent"] = matches[8]
		}

		if matches[7] != "-" && matches[7] == "" {
			headers["Referrer"] = matches[7]
		}
	}

	pathParts := strings.SplitN(path, "?", 2)
	cleanPath := pathParts[0]

	entry := &models.LogEntry{
		Method:  strings.ToUpper(method),
		Path:    cleanPath,
		Headers: headers,
		Body:    nil,
	}

	return entry, nil
}

func ConvertNginxLogs(inputPath, outputPath, format string) error {
	parser := NewNginxParser(format)
	return parser.ParseFile(inputPath, outputPath)
}
