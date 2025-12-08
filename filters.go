package main

import (
	"strings"
)

func Apply(entries []LogEntry, args *CliArgs) []LogEntry {
	if args.FilterMethod == "" && args.FilterPath == "" {
		return entries
	}

	filtered := make([]LogEntry, 0)

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
