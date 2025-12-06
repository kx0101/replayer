package main

import (
	"fmt"
	"os"

	"github.com/kx0101/replayer/cli"
	"github.com/kx0101/replayer/filters"
	"github.com/kx0101/replayer/reader"
	"github.com/kx0101/replayer/replay"
	"github.com/kx0101/replayer/summary"
)

func main() {
	args := cli.ParseArgs()

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

	if args.OutputJSON {
		summary.PrintJSONOutput(results)
		os.Exit(0)
	}

	summary.PrintSummary(results, args.Compare)
}
