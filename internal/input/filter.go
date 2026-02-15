package input

import (
	"strings"

	"github.com/kx0101/replayer/internal/cli"
	"github.com/kx0101/replayer/internal/models"
)

func Apply(entries []models.LogEntry, args *cli.CliArgs) []models.LogEntry {
	if args.FilterMethod == "" && args.FilterPath == "" {
		return entries
	}

	filtered := make([]models.LogEntry, 0)

	for _, entry := range entries {
		if args.FilterMethod != "" {
			if !strings.EqualFold(entry.Method, args.FilterMethod) {
				continue
			}
		}

		if args.FilterPath != "" {
			if !strings.Contains(entry.Path, args.FilterPath) {
				continue
			}
		}

		filtered = append(filtered, entry)
	}

	return filtered
}
