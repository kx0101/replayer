pub mod cli;
pub mod cloud;
pub mod input;
pub mod output;
pub mod proxy;
pub mod replay;
pub mod rules;

use replayer_core::ExitCode;

fn main() {
    let code = run();
    std::process::exit(i32::from(code));
}

fn run() -> ExitCode {
    let args = match cli::CliArgs::parse_and_validate() {
        Ok(args) => args,
        Err(code) => return code,
    };

    let rt = tokio::runtime::Runtime::new().expect("Failed to create tokio runtime");
    rt.block_on(execute(args))
}

async fn execute(args: cli::CliArgs) -> ExitCode {
    if !args.parse_nginx.is_empty() {
        return run_parse_nginx(&args);
    }

    if args.dry_run {
        return run_dry_run(&args);
    }

    if args.capture_mode {
        return run_capture(&args).await;
    }

    run_replay_mode(&args).await
}

fn run_parse_nginx(args: &cli::CliArgs) -> ExitCode {
    println!(
        "Converting nginx logs from {} to {}...",
        args.input_file, args.parse_nginx
    );
    if let Err(e) =
        input::nginx::convert_nginx_logs(&args.input_file, &args.parse_nginx, &args.nginx_format)
    {
        return handle_error("Failed to parse nginx logs", &e);
    }
    ExitCode::Ok
}

fn run_dry_run(args: &cli::CliArgs) -> ExitCode {
    if let Err(e) = input::reader::dry_run(&args.input_file) {
        return handle_error("Dry run failed", &e);
    }
    ExitCode::Ok
}

async fn run_capture(args: &cli::CliArgs) -> ExitCode {
    println!(
        "Starting reverse proxy on {}, forwarding to {}...",
        args.listen_addr, args.upstream
    );
    let config = proxy::CaptureConfig {
        listen_addr: args.listen_addr.clone(),
        upstream: args.upstream.clone(),
        output_file: args.capture_out.clone(),
        stream: args.capture_stream,
        tls_cert: args.tls_cert.clone(),
        tls_key: args.tls_key.clone(),
    };
    if let Err(e) = proxy::start_reverse_proxy(&config).await {
        return handle_error("Failed to start reverse proxy", &e);
    }
    ExitCode::Ok
}

async fn run_replay_mode(args: &cli::CliArgs) -> ExitCode {
    let entries = match input::reader::read_entries(&args.input_file, args.limit) {
        Ok(e) => e,
        Err(e) => return handle_error("failed to read input file", &e),
    };

    let filtered = input::filter::apply(entries, &args.filter_method, &args.filter_path);
    let results = replay::engine::run(&filtered, args).await;

    let agg = output::summary::aggregate_results(&results);
    let summary = output::summary::convert_to_summary(&agg);

    let run_data = rules::types::ReplayRunData { results, summary };

    if !args.html_report.is_empty() {
        if let Err(e) = output::report::generate_html(
            &run_data.results,
            &args.input_file,
            &args.targets,
            args.compare,
            &args.html_report,
        ) {
            return handle_error("Failed to generate HTML report", &e);
        }
    }

    if args.cloud_upload {
        if let Err(e) = upload_to_cloud(args, &run_data).await {
            eprintln!("Warning: cloud upload failed: {}", e);
        }
    }

    if !args.rules_file.is_empty() {
        return run_rules(args, &run_data);
    }

    output_results(args, &run_data)
}

async fn upload_to_cloud(
    args: &cli::CliArgs,
    data: &rules::types::ReplayRunData,
) -> anyhow::Result<()> {
    if args.cloud_api_key.is_empty() {
        anyhow::bail!("REPLAYER_API_KEY not set (use --cloud-api-key or set env var)");
    }

    let client = cloud::CloudClient::new(&args.cloud_url, &args.cloud_api_key)?;
    let req = cloud::UploadRequest {
        environment: args.cloud_env.clone(),
        targets: args.targets.clone(),
        summary: data.summary.clone(),
        results: data.results.clone(),
        labels: Some(args.cloud_labels.clone()),
    };

    let resp = client.upload(&req).await?;
    eprintln!("Uploaded to cloud: {}/runs/{}", args.cloud_url, resp.id);
    Ok(())
}

fn run_rules(args: &cli::CliArgs, current: &rules::types::ReplayRunData) -> ExitCode {
    let rules_config = match rules::parser::parse_rules_file(&args.rules_file) {
        Ok(c) => c,
        Err(e) => return handle_error("Failed to load rules", &e),
    };

    let baseline = load_baseline(&args.baseline_file);
    let eval_result = rules::engine::evaluate_rules(&rules_config, current, baseline.as_ref());

    if args.output_json {
        return output_rules_json(current, &eval_result);
    }

    eprint!("{}", rules::formatter::format_rule_result(&eval_result));
    rules::formatter::get_exit_code(&eval_result)
}

fn load_baseline(baseline_file: &str) -> Option<rules::types::ReplayRunData> {
    if baseline_file.is_empty() {
        return None;
    }

    match rules::parser::load_baseline_file(baseline_file) {
        Ok(b) => Some(b),
        Err(e) => {
            eprintln!("Warning: failed to load baseline: {}", e);
            eprintln!("Latency rules will be skipped");
            None
        }
    }
}

fn output_rules_json(
    current: &rules::types::ReplayRunData,
    eval_result: &rules::types::RuleEvaluationResult,
) -> ExitCode {
    let output = serde_json::json!({
        "results": current.results,
        "summary": current.summary,
        "rule_evaluation": eval_result,
    });

    if let Err(e) = serde_json::to_writer_pretty(std::io::stdout(), &output) {
        eprintln!("Error encoding JSON: {}", e);
    }
    println!();

    rules::formatter::get_exit_code(eval_result)
}

fn output_results(args: &cli::CliArgs, out: &rules::types::ReplayRunData) -> ExitCode {
    if args.output_json {
        output::summary::print_json_output(&out.results);
    } else {
        output::summary::print_summary(&out.results, args.compare);
    }

    exit_for_results(args, &out.results)
}

fn exit_for_results(
    args: &cli::CliArgs,
    results: &[replayer_core::models::MultiEnvResult],
) -> ExitCode {
    if args.compare && replay::engine::has_diffs(results) {
        return ExitCode::Diffs;
    }
    ExitCode::Ok
}

fn handle_error(msg: &str, err: &dyn std::fmt::Display) -> ExitCode {
    eprintln!("{}: {}", msg, err);
    ExitCode::Runtime
}
