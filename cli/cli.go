package cli

import (
	"flag"
	"fmt"
	"os"
)

type CliArgs struct {
	InputFile    string
	Targets      []string
	Concurrency  int
	Timeout      int64
	Delay        int64
	Limit        int
	FilterMethod string
	FilterPath   string
	DryRun       bool
	SummaryOnly  bool
	OutputJSON   bool
	Compare      bool
	RateLimit    int
	ProgressBar  bool
}

func ParseArgs() *CliArgs {
	args := &CliArgs{}

	flag.StringVar(&args.InputFile, "input-file", "", "Path to the input log file")
	flag.IntVar(&args.Concurrency, "concurrency", 1, "Number of concurrent requests")
	flag.Int64Var(&args.Timeout, "timeout", 5000, "Timeout for request (ms)")
	flag.Int64Var(&args.Delay, "delay", 0, "Delay per request (ms)")
	flag.IntVar(&args.Limit, "limit", 0, "Limit the number of requests to replay")
	flag.StringVar(&args.FilterMethod, "filter-method", "", "Filter method (e.g., GET, POST)")
	flag.StringVar(&args.FilterPath, "filter-path", "", "Filter path (e.g., /api/resource)")
	flag.BoolVar(&args.DryRun, "dry-run", false, "Enable dry run mode")
	flag.BoolVar(&args.SummaryOnly, "summary-only", false, "Output summary only")
	flag.BoolVar(&args.OutputJSON, "output-json", false, "Output results as JSON")
	flag.BoolVar(&args.Compare, "compare", false, "Compare responses between targets")
	flag.IntVar(&args.RateLimit, "rate-limit", 0, "Maximum requests per second (0 = unlimited)")
	flag.BoolVar(&args.ProgressBar, "progress", true, "Show progress bar")

	flag.Parse()

	args.Targets = flag.Args()

	if args.InputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --input-file is required")
		flag.Usage()
		os.Exit(1)
	}

	if len(args.Targets) == 0 && !args.DryRun {
		fmt.Fprintln(os.Stderr, "Error: at least one target is required")
		flag.Usage()
		os.Exit(1)
	}

	return args
}
