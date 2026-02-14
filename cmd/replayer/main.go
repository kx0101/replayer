package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kx0101/replayer/internal/cli"
	"github.com/kx0101/replayer/internal/cloud"
	"github.com/kx0101/replayer/internal/input"
	"github.com/kx0101/replayer/internal/models"
	"github.com/kx0101/replayer/internal/output"
	"github.com/kx0101/replayer/internal/proxy"
	"github.com/kx0101/replayer/internal/replay"
	"github.com/kx0101/replayer/internal/rules"
)

var (
	dryRunFn            = input.DryRun
	convertNginxLogsFn  = input.ConvertNginxLogs
	startReverseProxyFn = proxy.StartReverseProxy
	readEntriesFn       = input.ReadEntries
	runReplayFn         = replay.Run
	generateHTMLFn      = output.GenerateHTML
	printSummaryFn      = output.PrintSummary
	printJSONOutputFn   = output.PrintJSONOutput
	applyFn             = input.Apply
	aggregateResultsFn  = output.AggregateResults
	convertToSummaryFn  = output.ConvertToSummary
)

func main() {
	os.Exit(int(run()))
}

func run() cli.ExitCode {
	args, code := cli.ParseArgs()
	if code != cli.ExitOK {
		return code
	}

	return execute(args)
}

func execute(args *cli.CliArgs) cli.ExitCode {
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

func runParseNginx(args *cli.CliArgs) cli.ExitCode {
	fmt.Printf("Converting nginx logs from %s to %s...\n", args.InputFile, args.ParseNginx)
	if err := convertNginxLogsFn(args.InputFile, args.ParseNginx, args.NginxFormat); err != nil {
		return handleError("Failed to parse nginx logs", err)
	}

	return cli.ExitOK
}

func runDryRun(args *cli.CliArgs) cli.ExitCode {
	if err := dryRunFn(args.InputFile); err != nil {
		return handleError("Dry run failed", err)
	}

	return cli.ExitOK
}

func runCapture(args *cli.CliArgs) cli.ExitCode {
	fmt.Printf("Starting reverse proxy on %s, forwarding to %s...\n", args.ListenAddr, args.Upstream)
	config := &proxy.CaptureConfig{
		ListenAddr: args.ListenAddr,
		Upstream:   args.Upstream,
		OutputFile: args.CaptureOut,
		Stream:     args.CaptureStream,
		TLSCert:    args.TLSCert,
		TLSKey:     args.TLSKey,
	}

	if err := startReverseProxyFn(config); err != nil {
		return handleError("Failed to start reverse proxy", err)
	}
	return cli.ExitOK
}

func runReplayMode(args *cli.CliArgs) cli.ExitCode {
	entries, err := readEntriesFn(args)
	if err != nil {
		return handleError("failed to read input file", err)
	}

	filtered := applyFn(entries, args)
	results := runReplayFn(filtered, args)

	out := &rules.ReplayRunData{
		Results: results,
		Summary: convertToSummaryFn(aggregateResultsFn(results)),
	}

	if args.HTMLReport != "" {
		if err := generateHTMLFn(out.Results, args, args.HTMLReport); err != nil {
			return handleError("Failed to generate HTML report", err)
		}
	}

	if args.CloudUpload {
		if err := uploadToCloud(args, out); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cloud upload failed: %v\n", err)
		}
	}

	if args.RulesFile != "" {
		return runRules(args, out)
	}

	return outputResults(args, out)
}

func uploadToCloud(args *cli.CliArgs, data *rules.ReplayRunData) error {
	if args.CloudAPIKey == "" {
		return fmt.Errorf("REPLAYER_API_KEY not set (use --cloud-api-key or set env var)")
	}

	client, err := cloud.NewClient(args.CloudURL, args.CloudAPIKey)
	if err != nil {
		return fmt.Errorf("creating cloud client: %w", err)
	}

	req := &cloud.UploadRequest{
		Environment: args.CloudEnv,
		Targets:     args.Targets,
		Summary:     data.Summary,
		Results:     data.Results,
		Labels:      args.CloudLabels,
	}

	resp, err := client.Upload(req)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Uploaded to cloud: %s/runs/%s\n", args.CloudURL, resp.ID)
	return nil
}

func runRules(args *cli.CliArgs, current *rules.ReplayRunData) cli.ExitCode {
	rulesConfig, err := rules.ParseRulesFile(args.RulesFile)
	if err != nil {
		return handleError("Failed to load rules", err)
	}

	baseline := loadBaseline(args.BaselineFile)
	evalResult := rules.EvaluateRules(rulesConfig, current, baseline)

	if args.OutputJSON {
		return outputRulesJSON(current, evalResult)
	}

	fmt.Fprint(os.Stderr, rules.FormatRuleResult(evalResult))
	return rules.GetExitCode(evalResult)
}

func loadBaseline(baselineFile string) *rules.ReplayRunData {
	if baselineFile == "" {
		return nil
	}

	baseline, err := rules.LoadBaselineFile(baselineFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load baseline: %v\n", err)
		fmt.Fprintf(os.Stderr, "Latency rules will be skipped\n")
		return nil
	}

	return baseline
}

func outputRulesJSON(current *rules.ReplayRunData, evalResult *rules.RuleEvaluationResult) cli.ExitCode {
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

	return rules.GetExitCode(evalResult)
}

func outputResults(args *cli.CliArgs, out *rules.ReplayRunData) cli.ExitCode {
	if args.OutputJSON {
		printJSONOutputFn(out.Results)
	} else {
		printSummaryFn(out.Results, args.Compare)
	}

	return exitForResults(args, out.Results)
}

func exitForResults(args *cli.CliArgs, results []models.MultiEnvResult) cli.ExitCode {
	if args.Compare && replay.HasDiffs(results) {
		return cli.ExitDiffs
	}

	return cli.ExitOK
}

func handleError(msg string, err error) cli.ExitCode {
	fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	return cli.ExitRuntime
}
