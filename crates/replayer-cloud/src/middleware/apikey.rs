use axum::http::StatusCode;
use axum::{
    body::Body,
    extract::State,
    http::Request,
    middleware::Next,
    response::{IntoResponse, Response},
};
use uuid::Uuid;

use crate::auth::hash_api_key;
use crate::handler::AppState;

#[derive(Clone, Debug)]
pub struct AuthUserId(pub Uuid);

pub async fn apikey_auth(
    State(state): State<AppState>,
    mut req: Request<Body>,
    next: Next,
) -> Response {
    let provided = req
        .headers()
        .get("X-API-Key")
        .and_then(|v| v.to_str().ok())
        .map(|s| s.to_string());

    let provided = match provided {
        Some(k) if !k.is_empty() => k,
        _ => return (StatusCode::UNAUTHORIZED, r#"{"error":"unauthorized"}"#).into_response(),
    };

    let key_hash = hash_api_key(&provided);
    let api_key_record = match state.store.get_api_key_by_hash(&key_hash).await {
        Ok(Some(k)) => k,
        _ => return (StatusCode::UNAUTHORIZED, r#"{"error":"unauthorized"}"#).into_response(),
    };

    if let Some(expires) = api_key_record.expires_at {
        if chrono::Utc::now() > expires {
            return (StatusCode::UNAUTHORIZED, r#"{"error":"api key expired"}"#).into_response();
        }
    }

    let store = state.store.clone();
    let key_id = api_key_record.id;
    tokio::spawn(async move {
        let _ = store.update_api_key_last_used(key_id).await;
    });

    req.extensions_mut()
        .insert(AuthUserId(api_key_record.user_id));
    next.run(req).await
}
