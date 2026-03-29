use axum::{extract::State, http::StatusCode, response::Response, Extension};
use uuid::Uuid;

use crate::middleware::AuthUserId;

use super::response::{respond_error, respond_json};
use super::AppState;

pub async fn set_baseline(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    axum::extract::Path(id): axum::extract::Path<Uuid>,
) -> Response {
    if let Err(e) = state.store.set_baseline_for_user(user_id, id).await {
        let msg = e.to_string();
        if msg.contains("run not found") {
            return respond_error(StatusCode::NOT_FOUND, "run not found");
        }
        tracing::error!("error setting baseline: {}", e);
        return respond_error(StatusCode::INTERNAL_SERVER_ERROR, "failed to set baseline");
    }

    respond_json(StatusCode::OK, &serde_json::json!({"status": "ok"}))
}
