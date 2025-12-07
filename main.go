package main

import (
	"fmt"
	"os"

	"github.com/kx0101/replayer/internal/cli"
	"github.com/kx0101/replayer/internal/filters"
	"github.com/kx0101/replayer/internal/parser"
	"github.com/kx0101/replayer/internal/reader"
	"github.com/kx0101/replayer/internal/replay"
	"github.com/kx0101/replayer/internal/report"
	"github.com/kx0101/replayer/internal/summary"
)

func main() {
	args := cli.ParseArgs()

	if args.ParseNginx != "" {
		fmt.Printf("Converting nginx logs from %s to %s...\n", args.InputFile, args.ParseNginx)
		if err := parser.ConvertNginxLogs(args.InputFile, args.ParseNginx, args.NginxFormat); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse nginx logs: %v\n", err)
			os.Exit(1)
		}

		return
	}

	if args.DryRun {
		reader.DryRun(args.InputFile)
		return
	}

	entries, err := reader.ReadEntries(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input file: %v\n", err)
		os.Exit(1)
	}

	filtered := filters.Apply(entries, args)

	results := replay.Run(filtered, args)

	if args.HTMLReport != "" {
		if err := report.GenerateHTML(results, args, args.HTMLReport); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate HTML report: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nHTML report generated: %s\n", args.HTMLReport)
	}

	if args.OutputJSON {
		summary.PrintJSONOutput(results)
		os.Exit(0)
	}

	summary.PrintSummary(results, args.Compare)
}
