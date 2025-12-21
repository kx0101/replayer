package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type ExitCode int

const (
	ExitOK ExitCode = iota
	ExitDiffs
	ExitRules
	ExitInvalid
	ExitRuntime
)

var (
	dryRun            = DryRun
	convertNginxLogs  = ConvertNginxLogs
	startReverseProxy = StartReverseProxy
	readEntries       = ReadEntries
	runReplay         = Run
	generateHTML      = GenerateHTML
	printSummary      = PrintSummary
	printJSONOutput   = PrintJSONOutput
	apply             = Apply
	aggregateResults  = AggregateResults
	convertToSummary  = ConvertToSummary
)

func main() {
	os.Exit(int(run()))
}

func run() ExitCode {
	args, code := ParseArgs()
	if code != ExitOK {
		return code
	}

	return execute(args)
}

func execute(args *CliArgs) ExitCode {
	switch {
	case args.ParseNginx != "":
		return runParseNginx(args)
	case args.DryRun:
		return runDryRun(args)
	case args.CaptureMode:
		return runCapture(args)
	default:
		return runReplayMode(args)
	}
}

func runParseNginx(args *CliArgs) ExitCode {
	fmt.Printf("Converting nginx logs from %s to %s...\n", args.InputFile, args.ParseNginx)
	if err := convertNginxLogs(args.InputFile, args.ParseNginx, args.NginxFormat); err != nil {
		return handleError("Failed to parse nginx logs", err)
	}

	return ExitOK
}

func runDryRun(args *CliArgs) ExitCode {
	if err := dryRun(args.InputFile); err != nil {
		return handleError("Dry run failed", err)
	}

	return ExitOK
}

func runCapture(args *CliArgs) ExitCode {
	fmt.Printf("Starting reverse proxy on %s, forwarding to %s...\n", args.ListenAddr, args.Upstream)
	config := &CaptureConfig{
		ListenAddr: args.ListenAddr,
		Upstream:   args.Upstream,
		OutputFile: args.CaptureOut,
		Stream:     args.CaptureStream,
		TLSCert:    args.TLSCert,
		TLSKey:     args.TLSKey,
	}

	if err := startReverseProxy(config); err != nil {
		return handleError("Failed to start reverse proxy", err)
	}
	return ExitOK
}

func runReplayMode(args *CliArgs) ExitCode {
	entries, err := readEntries(args)
	if err != nil {
		return handleError("failed to read input file", err)
	}

	filtered := apply(entries, args)
	results := runReplay(filtered, args)

	out := &ReplayRunData{
		Results: results,
		Summary: convertToSummary(aggregateResults(results)),
	}

	if args.HTMLReport != "" {
		if err := generateHTML(out.Results, args, args.HTMLReport); err != nil {
			return handleError("Failed to generate HTML report", err)
		}
	}

	if args.RulesFile != "" {
		return runRules(args, out)
	}

	return outputResults(args, out)
}

func runRules(args *CliArgs, current *ReplayRunData) ExitCode {
	rulesConfig, err := ParseRulesFile(args.RulesFile)
	if err != nil {
		return handleError("Failed to load rules", err)
	}

	baseline := loadBaseline(args.BaselineFile)
	evalResult := EvaluateRules(rulesConfig, current, baseline)

	if args.OutputJSON {
		return outputRulesJSON(current, evalResult)
	}

	fmt.Fprint(os.Stderr, FormatRuleResult(evalResult))
	return GetExitCode(evalResult)
}

func loadBaseline(baselineFile string) *ReplayRunData {
	if baselineFile == "" {
		return nil
	}

	baseline, err := LoadBaselineFile(baselineFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load baseline: %v\n", err)
		fmt.Fprintf(os.Stderr, "Latency rules will be skipped\n")
		return nil
	}

	return baseline
}

func outputRulesJSON(current *ReplayRunData, evalResult *RuleEvaluationResult) ExitCode {
	output := map[string]any{
		"results":         current.Results,
		"summary":         current.Summary,
		"rule_evaluation": evalResult,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}

	return GetExitCode(evalResult)
}

func outputResults(args *CliArgs, out *ReplayRunData) ExitCode {
	if args.OutputJSON {
		printJSONOutput(out.Results)
	} else {
		printSummary(out.Results, args.Compare)
	}

	return exitForResults(args, out.Results)
}

func exitForResults(args *CliArgs, results []MultiEnvResult) ExitCode {
	if args.Compare && HasDiffs(results) {
		return ExitDiffs
	}

	return ExitOK
}

func handleError(msg string, err error) ExitCode {
	fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	return ExitRuntime
}
