use std::collections::HashMap;

use clap::Parser;
use replayer_core::ExitCode;

#[derive(Parser, Debug)]
#[command(
    name = "replayer",
    about = "HTTP traffic replay tool",
    trailing_var_arg = true
)]
pub struct CliArgs {
    #[arg(long = "input-file", default_value = "")]
    pub input_file: String,

    #[arg(long, default_value_t = 1)]
    pub concurrency: usize,

    #[arg(long, default_value_t = 5000)]
    pub timeout: i64,

    #[arg(long, default_value_t = 0)]
    pub delay: i64,

    #[arg(long, default_value_t = 0)]
    pub limit: usize,

    #[arg(long = "filter-method", default_value = "")]
    pub filter_method: String,

    #[arg(long = "filter-path", default_value = "")]
    pub filter_path: String,

    #[arg(long = "dry-run")]
    pub dry_run: bool,

    #[arg(long = "summary-only")]
    pub summary_only: bool,

    #[arg(long = "output-json")]
    pub output_json: bool,

    #[arg(long)]
    pub compare: bool,

    #[arg(long = "rate-limit", default_value_t = 0)]
    pub rate_limit: usize,

    #[arg(
        long = "progress",
        default_value_t = true,
        num_args = 0..=1,
        default_missing_value = "true",
        action = clap::ArgAction::Set,
    )]
    pub progress_bar: bool,

    #[arg(long = "auth", default_value = "")]
    pub auth_header: String,

    #[arg(long = "header")]
    pub headers: Vec<String>,

    #[arg(long = "html-report", default_value = "")]
    pub html_report: String,

    #[arg(long = "parse-nginx", default_value = "")]
    pub parse_nginx: String,

    #[arg(long = "nginx-format", default_value = "combined")]
    pub nginx_format: String,

    #[arg(
        long = "ignore-volatile",
        default_value_t = true,
        num_args = 0..=1,
        default_missing_value = "true",
        action = clap::ArgAction::Set,
    )]
    pub ignore_volatile: bool,

    #[arg(long = "ignore-field")]
    pub ignore_fields: Vec<String>,

    #[arg(long = "ignore-pattern")]
    pub ignore_patterns: Vec<String>,

    #[arg(long = "show-volatile-diffs")]
    pub show_volatile_diffs: bool,

    #[arg(long = "listen", default_value = ":8080")]
    pub listen_addr: String,

    #[arg(long = "upstream", default_value = "")]
    pub upstream: String,

    #[arg(long = "output", default_value = "captured.json")]
    pub capture_out: String,

    #[arg(long = "capture")]
    pub capture_mode: bool,

    #[arg(long = "stream")]
    pub capture_stream: bool,

    #[arg(long = "tls-cert", default_value = "")]
    pub tls_cert: String,

    #[arg(long = "tls-key", default_value = "")]
    pub tls_key: String,

    #[arg(long = "rules", default_value = "")]
    pub rules_file: String,

    #[arg(long = "baseline", default_value = "")]
    pub baseline_file: String,

    #[arg(long = "cloud")]
    pub cloud_upload: bool,

    #[arg(
        long = "cloud-url",
        env = "REPLAYER_CLOUD_URL",
        default_value = "http://localhost:8090"
    )]
    pub cloud_url: String,

    #[arg(long = "cloud-api-key", env = "REPLAYER_API_KEY", default_value = "")]
    pub cloud_api_key: String,

    #[arg(long = "cloud-env", default_value = "default")]
    pub cloud_env: String,

    #[arg(long = "cloud-label")]
    pub(crate) cloud_label_args: Vec<String>,

    #[arg(skip)]
    pub cloud_labels: HashMap<String, String>,

    #[arg(allow_hyphen_values = true)]
    pub targets: Vec<String>,
}

impl CliArgs {
    pub fn parse_and_validate() -> Result<Self, ExitCode> {
        let mut args = Self::parse();

        args.cloud_labels = HashMap::new();
        for label in &args.cloud_label_args {
            if let Some(idx) = label.find('=') {
                if idx > 0 {
                    args.cloud_labels
                        .insert(label[..idx].to_string(), label[idx + 1..].to_string());
                }
            }
        }

        if !args.parse_nginx.is_empty() {
            if args.input_file.is_empty() {
                eprintln!("Error: --input-file is required");
                return Err(ExitCode::Invalid);
            }
            return Ok(args);
        }

        if args.capture_mode {
            if args.upstream.is_empty() {
                eprintln!("Error: --upstream is required in capture mode");
                return Err(ExitCode::Invalid);
            }
            return Ok(args);
        }

        if args.input_file.is_empty() {
            eprintln!("Error: --input-file is required");
            return Err(ExitCode::Invalid);
        }

        if args.targets.is_empty() && !args.dry_run {
            eprintln!("Error: at least one target is required");
            return Err(ExitCode::Invalid);
        }

        Ok(args)
    }
}
