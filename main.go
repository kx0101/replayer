package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	args := ParseArgs()

	if args.ParseNginx != "" {
		fmt.Printf("Converting nginx logs from %s to %s...\n", args.InputFile, args.ParseNginx)
		if err := ConvertNginxLogs(args.InputFile, args.ParseNginx, args.NginxFormat); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse nginx logs: %v\n", err)
			os.Exit(1)
		}

		return
	}

	if args.DryRun {
		err := DryRun(args.InputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Dry run failed: %v\n", err)
			os.Exit(1)
		}

		return
	}

	if args.CaptureMode {
		fmt.Printf("Starting reverse proxy on %s, forwarding to %s...\n", args.ListenAddr, args.Upstream)
		config := &CaptureConfig{
			ListenAddr: args.ListenAddr,
			Upstream:   args.Upstream,
			OutputFile: args.CaptureOut,
			Stream:     args.CaptureStream,
			TLSCert:    args.TLSCert,
			TLSKey:     args.TLSKey,
		}

		if err := StartReverseProxy(config); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start reverse proxy: %v\n", err)
			os.Exit(1)
		}

		return
	}

	entries, err := ReadEntries(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read input file: %v\n", err)
		os.Exit(1)
	}

	filtered := Apply(entries, args)

	results := Run(filtered, args)

	summary := AggregateResults(results)

	if args.HTMLReport != "" {
		if err := GenerateHTML(results, args, args.HTMLReport); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate HTML report: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("\nHTML report generated: %s\n", args.HTMLReport)
	}

	if args.RulesFile != "" {
		rulesConfig, err := ParseRulesFile(args.RulesFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load rules: %v\n", err)
			os.Exit(1)
		}

		current := &ReplayRunData{
			Results: results,
			Summary: ConvertToSummary(summary),
		}

		var baseline *ReplayRunData
		if args.BaselineFile != "" {
			baseline, err = LoadBaselineFile(args.BaselineFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to load baseline: %v\n", err)
				fmt.Fprintf(os.Stderr, "Latency rules will be skipped\n")
			}
		}

		evalResult := EvaluateRules(rulesConfig, current, baseline)

		if args.OutputJSON {
			output := map[string]any{
				"results":         results,
				"summary":         summary,
				"rule_evaluation": evalResult,
			}

			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(output); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			}

			os.Exit(GetExitCode(evalResult))
		}

		fmt.Fprint(os.Stderr, FormatRuleResult(evalResult))
		os.Exit(GetExitCode(evalResult))
	}

	if args.OutputJSON {
		PrintJSONOutput(results)
		os.Exit(0)
	}

	PrintSummary(results, args.Compare)
}
