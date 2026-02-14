package cli

import (
	"flag"
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
	AuthHeader   string
	Headers      []string
	HTMLReport   string
	ParseNginx   string
	NginxFormat  string

	IgnoreVolatile    bool
	IgnoreFields      []string
	IgnorePatterns    []string
	ShowVolatileDiffs bool

	ListenAddr    string
	Upstream      string
	CaptureOut    string
	CaptureMode   bool
	CaptureStream bool

	TLSCert string
	TLSKey  string

	RulesFile    string
	BaselineFile string

	CloudUpload bool
	CloudURL    string
	CloudAPIKey string
	CloudEnv    string
	CloudLabels map[string]string
}

func ParseArgs() (*CliArgs, ExitCode) {
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

	flag.StringVar(&args.AuthHeader, "auth", "", "Authorization header value (e.g., 'Bearer token123')")

	var headerFlags stringSlice
	flag.Var(&headerFlags, "header", "Custom header in format 'Key: Value' (can be used multiple times)")

	flag.StringVar(&args.HTMLReport, "html-report", "", "Generate HTML report at specified path")

	flag.StringVar(&args.ParseNginx, "parse-nginx", "", "Convert nginx log to json format (output path)")
	flag.StringVar(&args.NginxFormat, "nginx-format", "combined", "Nginx log format (combined or common)")

	flag.BoolVar(&args.IgnoreVolatile, "ignore-volatile", true, "Ignore common volatile fields (timestamps, IDs)")
	flag.BoolVar(&args.ShowVolatileDiffs, "show-volatile-diffs", false, "Show diffs even if only volatile fields differ")

	var ignoreFieldsFlag stringSlice
	var ignorePatternsFlag stringSlice
	flag.Var(&ignoreFieldsFlag, "ignore-field", "JSON field to ignore in comparison (can be repeated)")
	flag.Var(&ignorePatternsFlag, "ignore-pattern", "Regex pattern for fields to ignore (can be repeated)")

	flag.BoolVar(&args.CaptureMode, "capture", false, "Enable reverse proxy capture mode")
	flag.StringVar(&args.ListenAddr, "listen", ":8080", "Reverse proxy listen address")
	flag.StringVar(&args.Upstream, "upstream", "", "Upstream server to proxy to (e.g. production.api.com)")
	flag.StringVar(&args.CaptureOut, "output", "captured.json", "Output JSON file path")
	flag.BoolVar(&args.CaptureStream, "stream", false, "Also stream capture records to stdout")

	flag.StringVar(&args.TLSCert, "tls-cert", "", "TLS certification")
	flag.StringVar(&args.TLSKey, "tls-key", "", "TLS key")

	flag.StringVar(&args.RulesFile, "rules", "", "Path to rules.yaml file for regression testing")
	flag.StringVar(&args.BaselineFile, "baseline", "", "Path to baseline results JSON for comparison")

	flag.BoolVar(&args.CloudUpload, "cloud", false, "Upload results to Replayer Cloud")
	flag.StringVar(&args.CloudURL, "cloud-url", getEnvOrDefault("REPLAYER_CLOUD_URL", "http://localhost:8090"), "Replayer Cloud URL")
	flag.StringVar(&args.CloudAPIKey, "cloud-api-key", os.Getenv("REPLAYER_API_KEY"), "Replayer Cloud API key")
	flag.StringVar(&args.CloudEnv, "cloud-env", "default", "Environment name for cloud upload")

	var cloudLabelsFlag stringSlice
	flag.Var(&cloudLabelsFlag, "cloud-label", "Label for cloud upload in format 'key=value' (can be repeated)")

	flag.Parse()

	args.CloudLabels = make(map[string]string)
	for _, label := range cloudLabelsFlag {
		if idx := indexOf(label, '='); idx > 0 {
			args.CloudLabels[label[:idx]] = label[idx+1:]
		}
	}

	args.Headers = headerFlags
	args.IgnoreFields = ignoreFieldsFlag
	args.IgnorePatterns = ignorePatternsFlag
	args.Targets = flag.Args()

	if args.ParseNginx != "" {
		if args.InputFile == "" {
			fmt.Fprintln(os.Stderr, "Error: --input-file is required")
			flag.Usage()
			return nil, ExitInvalid
		}

		return args, ExitOK
	}

	if args.CaptureMode {
		if args.Upstream == "" {
			fmt.Fprintln(os.Stderr, "Error: --upstream is required in capture mode")
			flag.Usage()
			return nil, ExitInvalid
		}

		return args, ExitOK
	}

	if args.InputFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --input-file is required")
		flag.Usage()
		return nil, ExitInvalid
	}

	if len(args.Targets) == 0 && !args.DryRun {
		fmt.Fprintln(os.Stderr, "Error: at least one target is required")
		flag.Usage()
		return nil, ExitInvalid
	}

	return args, ExitOK
}

type stringSlice []string

func (s *stringSlice) String() string {
	return fmt.Sprintf("%v", *s)
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return defaultVal
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}

	return -1
}
