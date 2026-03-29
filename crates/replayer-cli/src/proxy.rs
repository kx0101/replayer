use std::collections::HashMap;
use std::sync::Arc;

use base64::{engine::general_purpose, Engine as _};
use chrono::Utc;
use http_body_util::{BodyExt, Full};
use hyper::body::Bytes;
use hyper::server::conn::http1;
use hyper::service::service_fn;
use hyper::Request;
use serde::Serialize;
use tokio::io::AsyncWriteExt;
use tokio::net::TcpListener;
use tokio::sync::Mutex;

pub struct CaptureConfig {
    pub listen_addr: String,
    pub upstream: String,
    pub output_file: String,
    pub stream: bool,
    pub tls_cert: String,
    pub tls_key: String,
}

#[derive(Serialize)]
struct CapturedEntry {
    timestamp: String,
    method: String,
    path: String,
    headers: HashMap<String, Vec<String>>,
    body: String,
    status: u16,
    response_headers: HashMap<String, Vec<String>>,
    response_body: String,
    latency_ms: i64,
}

pub async fn start_reverse_proxy(config: &CaptureConfig) -> anyhow::Result<()> {
    let raw_upstream = config.upstream.trim().to_string();
    if raw_upstream.is_empty() {
        anyhow::bail!("upstream is empty");
    }

    let upstream_url: url::Url = raw_upstream
        .parse()
        .map_err(|e| anyhow::anyhow!("invalid upstream URL: {}", e))?;

    let file = tokio::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(&config.output_file)
        .await?;
    let writer = Arc::new(Mutex::new(tokio::io::BufWriter::new(file)));

    let upstream_base = format!(
        "{}://{}",
        upstream_url.scheme(),
        upstream_url.host_str().unwrap_or("localhost"),
    );
    let upstream_base = if let Some(port) = upstream_url.port() {
        format!("{}:{}", upstream_base, port)
    } else {
        upstream_base
    };
    let upstream_base = Arc::new(upstream_base);

    let fwd_client = reqwest::Client::builder().build()?;

    let addr = config.listen_addr.clone();
    let bind_addr = if addr.starts_with(':') {
        format!("0.0.0.0{}", addr)
    } else {
        addr
    };

    let listener = TcpListener::bind(&bind_addr).await?;
    eprintln!(
        "Capture mode ON -- listening on {} --> {}",
        config.listen_addr,
        upstream_url.host_str().unwrap_or("")
    );

    let use_tls = !config.tls_cert.is_empty() && !config.tls_key.is_empty();

    if use_tls {
        let certs = load_certs(&config.tls_cert)?;
        let key = load_key(&config.tls_key)?;

        let tls_config = tokio_rustls::rustls::ServerConfig::builder()
            .with_no_client_auth()
            .with_single_cert(certs, key)?;

        let tls_acceptor = tokio_rustls::TlsAcceptor::from(Arc::new(tls_config));

        loop {
            let (stream, _) = listener.accept().await?;
            let tls_acceptor = tls_acceptor.clone();
            let upstream_base = upstream_base.clone();
            let fwd_client = fwd_client.clone();
            let writer = writer.clone();

            tokio::spawn(async move {
                match tls_acceptor.accept(stream).await {
                    Ok(tls_stream) => {
                        let io = hyper_util::rt::TokioIo::new(tls_stream);
                        let service = service_fn(move |req| {
                            let upstream_base = upstream_base.clone();
                            let fwd_client = fwd_client.clone();
                            let writer = writer.clone();
                            async move { handle_proxy(req, &upstream_base, &fwd_client, &writer).await }
                        });
                        if let Err(e) = http1::Builder::new().serve_connection(io, service).await {
                            eprintln!("Error serving TLS connection: {}", e);
                        }
                    }
                    Err(e) => eprintln!("TLS accept error: {}", e),
                }
            });
        }
    } else {
        loop {
            let (stream, _) = listener.accept().await?;
            let upstream_base = upstream_base.clone();
            let fwd_client = fwd_client.clone();
            let writer = writer.clone();

            tokio::spawn(async move {
                let io = hyper_util::rt::TokioIo::new(stream);
                let service = service_fn(move |req| {
                    let upstream_base = upstream_base.clone();
                    let fwd_client = fwd_client.clone();
                    let writer = writer.clone();
                    async move { handle_proxy(req, &upstream_base, &fwd_client, &writer).await }
                });
                if let Err(e) = http1::Builder::new().serve_connection(io, service).await {
                    eprintln!("Error serving connection: {}", e);
                }
            });
        }
    }
}

async fn handle_proxy(
    req: Request<hyper::body::Incoming>,
    upstream_base: &str,
    client: &reqwest::Client,
    writer: &Arc<Mutex<tokio::io::BufWriter<tokio::fs::File>>>,
) -> Result<hyper::Response<Full<Bytes>>, std::convert::Infallible> {
    let start = std::time::Instant::now();
    let timestamp = Utc::now();

    let (parts, body) = req.into_parts();
    let req_body_bytes = body
        .collect()
        .await
        .map(|b| b.to_bytes())
        .unwrap_or_default();

    let method_str = parts.method.to_string();
    let path = parts
        .uri
        .path_and_query()
        .map(|pq| pq.as_str().to_string())
        .unwrap_or_else(|| "/".to_string());

    let req_headers = header_map_to_hashmap(&parts.headers);

    let url = format!("{}{}", upstream_base, path);

    let method: reqwest::Method = parts.method;
    let mut fwd = client.request(method, &url);
    for (name, value) in parts.headers.iter() {
        fwd = fwd.header(name, value);
    }
    if !req_body_bytes.is_empty() {
        fwd = fwd.body(req_body_bytes.to_vec());
    }

    match fwd.send().await {
        Ok(upstream_resp) => {
            let status = upstream_resp.status().as_u16();
            let resp_headers = {
                let mut map: HashMap<String, Vec<String>> = HashMap::new();
                for (name, value) in upstream_resp.headers().iter() {
                    let v = value.to_str().unwrap_or("").to_string();
                    map.entry(name.as_str().to_string()).or_default().push(v);
                }
                map
            };
            let resp_body_bytes = upstream_resp.bytes().await.unwrap_or_default();
            let latency_ms = start.elapsed().as_millis() as i64;

            let entry = CapturedEntry {
                timestamp: timestamp.to_rfc3339(),
                method: method_str,
                path,
                headers: req_headers,
                body: general_purpose::STANDARD.encode(&req_body_bytes),
                status,
                response_headers: resp_headers,
                response_body: general_purpose::STANDARD.encode(&resp_body_bytes),
                latency_ms,
            };

            if let Ok(data) = serde_json::to_string(&entry) {
                eprintln!("{}", data);
                let mut w = writer.lock().await;
                let _ = w.write_all(data.as_bytes()).await;
                let _ = w.write_all(b"\n").await;
                let _ = w.flush().await;
            }

            let mut response = hyper::Response::new(Full::new(resp_body_bytes));
            *response.status_mut() =
                hyper::StatusCode::from_u16(status).unwrap_or(hyper::StatusCode::BAD_GATEWAY);
            Ok(response)
        }
        Err(e) => {
            let msg = format!("proxy error: {}", e);
            eprintln!("{}", msg);
            Ok(hyper::Response::builder()
                .status(502)
                .body(Full::new(Bytes::from(msg)))
                .unwrap())
        }
    }
}

fn header_map_to_hashmap(headers: &hyper::HeaderMap) -> HashMap<String, Vec<String>> {
    let mut map: HashMap<String, Vec<String>> = HashMap::new();
    for (name, value) in headers.iter() {
        let v = value.to_str().unwrap_or("").to_string();
        map.entry(name.as_str().to_string()).or_default().push(v);
    }
    map
}

fn load_certs(
    path: &str,
) -> anyhow::Result<Vec<tokio_rustls::rustls::pki_types::CertificateDer<'static>>> {
    let data = std::fs::read(path)?;
    let certs: Vec<_> = rustls_pemfile::certs(&mut &data[..]).collect::<Result<Vec<_>, _>>()?;
    Ok(certs)
}

fn load_key(path: &str) -> anyhow::Result<tokio_rustls::rustls::pki_types::PrivateKeyDer<'static>> {
    let data = std::fs::read(path)?;
    let key = rustls_pemfile::private_key(&mut &data[..])?
        .ok_or_else(|| anyhow::anyhow!("no private key found in {}", path))?;
    Ok(key)
}
