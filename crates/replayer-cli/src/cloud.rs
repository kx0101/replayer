use std::collections::HashMap;

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use replayer_core::models::{MultiEnvResult, Summary};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CloudClient {
    #[serde(skip)]
    base_url: String,

    #[serde(skip)]
    api_key: String,

    #[serde(skip)]
    client: reqwest::Client,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UploadRequest {
    pub environment: String,
    pub targets: Vec<String>,
    pub summary: Summary,
    pub results: Vec<MultiEnvResult>,

    #[serde(skip_serializing_if = "Option::is_none")]
    pub labels: Option<HashMap<String, String>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UploadResponse {
    pub id: String,
    pub environment: String,
    pub created_at: DateTime<Utc>,
}

impl CloudClient {
    pub fn new(base_url: &str, api_key: &str) -> anyhow::Result<Self> {
        let parsed: url::Url = base_url
            .parse()
            .map_err(|e| anyhow::anyhow!("invalid baseURL: {}", e))?;

        let scheme = parsed.scheme();
        if scheme != "https" && scheme != "http" {
            anyhow::bail!("invalid scheme in baseURL");
        }

        let host = parsed.host_str().unwrap_or("");
        if let Ok(ip) = host.parse::<std::net::IpAddr>() {
            if is_private_ip(&ip) {
                anyhow::bail!("baseURL cannot be private IP");
            }
        }

        let client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(30))
            .build()?;

        Ok(Self {
            base_url: parsed.to_string().trim_end_matches('/').to_string(),
            api_key: api_key.to_string(),
            client,
        })
    }

    pub async fn upload(&self, req: &UploadRequest) -> anyhow::Result<UploadResponse> {
        let body = serde_json::to_vec(req)?;

        let response = self
            .client
            .post(format!("{}/api/v1/runs", self.base_url))
            .header("Content-Type", "application/json")
            .header("X-API-Key", &self.api_key)
            .body(body)
            .send()
            .await
            .map_err(|e| anyhow::anyhow!("sending request: {}", e))?;

        let status = response.status();
        let resp_body = response
            .text()
            .await
            .map_err(|e| anyhow::anyhow!("reading response: {}", e))?;

        if status.as_u16() != 201 {
            anyhow::bail!("upload failed: {} - {}", status, resp_body);
        }

        let upload_resp: UploadResponse = serde_json::from_str(&resp_body)
            .map_err(|e| anyhow::anyhow!("parsing response: {}", e))?;

        Ok(upload_resp)
    }
}

fn is_private_ip(ip: &std::net::IpAddr) -> bool {
    match ip {
        std::net::IpAddr::V4(v4) => v4.is_private() || v4.is_loopback() || v4.is_link_local(),
        std::net::IpAddr::V6(v6) => v6.is_loopback(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use replayer_core::models::LatencyStats;

    fn make_upload_request() -> UploadRequest {
        UploadRequest {
            environment: "production".to_string(),
            targets: vec!["staging.api".to_string(), "prod.api".to_string()],
            summary: Summary {
                total_requests: 100,
                succeeded: 95,
                failed: 5,
                latency: LatencyStats::default(),
                by_target: None,
            },
            results: vec![],
            labels: Some(HashMap::from([
                ("version".to_string(), "v1.0.0".to_string()),
                ("branch".to_string(), "main".to_string()),
            ])),
        }
    }

    #[test]
    fn test_new_client() {
        let client = CloudClient::new("http://example.com:8090", "test-api-key").unwrap();
        assert!(client.base_url.contains("example.com"));
        assert_eq!(client.api_key, "test-api-key");
    }

    #[tokio::test]
    async fn test_upload_success() {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        let url = format!("http://{}", addr);

        tokio::spawn(async move {
            let app = axum::Router::new().route(
                "/api/v1/runs",
                axum::routing::post(|req: axum::extract::Request| async move {
                    let (parts, body) = req.into_parts();
                    assert_eq!(
                        parts.headers.get("Content-Type").unwrap(),
                        "application/json"
                    );
                    assert_eq!(parts.headers.get("X-API-Key").unwrap(), "test-api-key");

                    let body_bytes = axum::body::to_bytes(body, usize::MAX).await.unwrap();
                    let upload_req: UploadRequest = serde_json::from_slice(&body_bytes).unwrap();
                    assert_eq!(upload_req.environment, "production");

                    let resp = serde_json::json!({
                        "id": "test-run-id",
                        "environment": "production",
                        "created_at": "2024-01-01T00:00:00Z"
                    });
                    (axum::http::StatusCode::CREATED, axum::Json(resp))
                }),
            );
            axum::serve(listener, app).await.unwrap();
        });
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;

        let client = CloudClient {
            base_url: url,
            api_key: "test-api-key".to_string(),
            client: reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(5))
                .build()
                .unwrap(),
        };

        let req = make_upload_request();
        let resp = client.upload(&req).await.unwrap();
        assert_eq!(resp.id, "test-run-id");
        assert_eq!(resp.environment, "production");
    }

    #[tokio::test]
    async fn test_upload_unauthorized() {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        let url = format!("http://{}", addr);

        tokio::spawn(async move {
            let app = axum::Router::new().route(
                "/api/v1/runs",
                axum::routing::post(|| async {
                    (axum::http::StatusCode::UNAUTHORIZED, "invalid api key")
                }),
            );
            axum::serve(listener, app).await.unwrap();
        });
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;

        let client = CloudClient {
            base_url: url,
            api_key: "invalid-key".to_string(),
            client: reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(5))
                .build()
                .unwrap(),
        };

        let req = UploadRequest {
            environment: "test".to_string(),
            targets: vec!["api.test".to_string()],
            summary: Summary {
                total_requests: 0,
                succeeded: 0,
                failed: 0,
                latency: LatencyStats::default(),
                by_target: None,
            },
            results: vec![],
            labels: None,
        };

        let result = client.upload(&req).await;
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_upload_server_error() {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        let url = format!("http://{}", addr);

        tokio::spawn(async move {
            let app = axum::Router::new().route(
                "/api/v1/runs",
                axum::routing::post(|| async {
                    (
                        axum::http::StatusCode::INTERNAL_SERVER_ERROR,
                        "internal server error",
                    )
                }),
            );
            axum::serve(listener, app).await.unwrap();
        });
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;

        let client = CloudClient {
            base_url: url,
            api_key: "test-api-key".to_string(),
            client: reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(5))
                .build()
                .unwrap(),
        };

        let req = UploadRequest {
            environment: "test".to_string(),
            targets: vec!["api.test".to_string()],
            summary: Summary {
                total_requests: 0,
                succeeded: 0,
                failed: 0,
                latency: LatencyStats::default(),
                by_target: None,
            },
            results: vec![],
            labels: None,
        };

        let result = client.upload(&req).await;
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_upload_invalid_response() {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        let url = format!("http://{}", addr);

        tokio::spawn(async move {
            let app = axum::Router::new().route(
                "/api/v1/runs",
                axum::routing::post(|| async {
                    (axum::http::StatusCode::CREATED, "not valid json")
                }),
            );
            axum::serve(listener, app).await.unwrap();
        });
        tokio::time::sleep(std::time::Duration::from_millis(50)).await;

        let client = CloudClient {
            base_url: url,
            api_key: "test-api-key".to_string(),
            client: reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(5))
                .build()
                .unwrap(),
        };

        let req = UploadRequest {
            environment: "test".to_string(),
            targets: vec!["api.test".to_string()],
            summary: Summary {
                total_requests: 0,
                succeeded: 0,
                failed: 0,
                latency: LatencyStats::default(),
                by_target: None,
            },
            results: vec![],
            labels: None,
        };

        let result = client.upload(&req).await;
        assert!(result.is_err());
    }

    #[test]
    fn test_upload_request_json_roundtrip() {
        let req = make_upload_request();
        let data = serde_json::to_string(&req).unwrap();
        let decoded: UploadRequest = serde_json::from_str(&data).unwrap();

        assert_eq!(decoded.environment, req.environment);
        assert_eq!(decoded.targets.len(), req.targets.len());
        assert_eq!(decoded.labels.as_ref().unwrap()["version"], "v1.0.0");
    }

    #[test]
    fn test_upload_request_empty_labels_omitted() {
        let req = UploadRequest {
            environment: "test".to_string(),
            targets: vec!["api.test".to_string()],
            summary: Summary {
                total_requests: 0,
                succeeded: 0,
                failed: 0,
                latency: LatencyStats::default(),
                by_target: None,
            },
            results: vec![],
            labels: None,
        };

        let data = serde_json::to_string(&req).unwrap();
        let raw: serde_json::Value = serde_json::from_str(&data).unwrap();
        assert!(
            raw.get("labels").is_none(),
            "labels should be omitted when None"
        );
    }
}
