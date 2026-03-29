use std::collections::HashMap;

use axum::{extract::State, http::StatusCode, response::Response, Extension, Json};
use serde::Deserialize;

use replayer_core::models::{MultiEnvResult, Summary};

use crate::middleware::AuthUserId;
use crate::models::Run;
use crate::store::ListFilter;

use super::response::{respond_error, respond_json};
use super::AppState;

#[derive(Deserialize)]
pub struct CreateRunRequest {
    pub environment: String,
    pub targets: Vec<String>,
    pub summary: Summary,
    pub results: Vec<MultiEnvResult>,
    #[serde(default)]
    pub labels: Option<HashMap<String, String>>,
}

#[derive(Deserialize)]
pub struct ListRunsQuery {
    pub environment: Option<String>,
    pub limit: Option<i64>,
    pub offset: Option<i64>,
    pub after: Option<String>,
    pub before: Option<String>,
}

pub async fn create_run(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    Json(req): Json<CreateRunRequest>,
) -> Response {
    if req.environment.is_empty() {
        return respond_error(StatusCode::BAD_REQUEST, "environment is required");
    }
    if req.targets.is_empty() {
        return respond_error(StatusCode::BAD_REQUEST, "targets is required");
    }

    let mut run = Run {
        id: uuid::Uuid::nil(),
        user_id: None,
        environment: req.environment,
        targets: req.targets,
        created_at: chrono::Utc::now(),
        total_requests: req.summary.total_requests,
        succeeded: req.summary.succeeded,
        failed: req.summary.failed,
        latency_stats: req.summary.latency,
        by_target: req.summary.by_target.unwrap_or_default(),
        results: req.results,
        is_baseline: false,
        baseline_id: None,
        labels: Some(req.labels.unwrap_or_default()),
    };

    if let Err(e) = state.store.create_run_for_user(user_id, &mut run).await {
        tracing::error!("error creating run: {}", e);
        return respond_error(StatusCode::INTERNAL_SERVER_ERROR, "failed to create run");
    }

    respond_json(StatusCode::CREATED, &run)
}

pub async fn get_run(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    axum::extract::Path(id): axum::extract::Path<uuid::Uuid>,
) -> Response {
    match state.store.get_run_for_user(user_id, id).await {
        Ok(Some(run)) => respond_json(StatusCode::OK, &run),
        Ok(None) => respond_error(StatusCode::NOT_FOUND, "run not found"),
        Err(e) => {
            tracing::error!("error getting run: {}", e);
            respond_error(StatusCode::INTERNAL_SERVER_ERROR, "failed to get run")
        }
    }
}

pub async fn list_runs(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    axum::extract::Query(params): axum::extract::Query<ListRunsQuery>,
) -> Response {
    let filter = ListFilter {
        environment: params.environment,
        limit: params.limit.unwrap_or(0),
        offset: params.offset.unwrap_or(0),
        after: params
            .after
            .and_then(|s| chrono::DateTime::parse_from_rfc3339(&s).ok())
            .map(|dt| dt.with_timezone(&chrono::Utc)),
        before: params
            .before
            .and_then(|s| chrono::DateTime::parse_from_rfc3339(&s).ok())
            .map(|dt| dt.with_timezone(&chrono::Utc)),
    };

    match state.store.list_runs_for_user(user_id, filter).await {
        Ok((items, total)) => {
            let resp = serde_json::json!({
                "items": items,
                "total": total,
            });
            respond_json(StatusCode::OK, &resp)
        }
        Err(e) => {
            tracing::error!("error listing runs: {}", e);
            respond_error(StatusCode::INTERNAL_SERVER_ERROR, "failed to list runs")
        }
    }
}
