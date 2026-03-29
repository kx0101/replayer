use axum::{
    extract::State,
    http::StatusCode,
    response::{IntoResponse, Redirect, Response},
    Extension, Form,
};
use serde::Deserialize;
use uuid::Uuid;

use crate::auth;
use crate::middleware::{AuthUser, AuthUserId};
use crate::models::APIKey;

use super::response::render_template;
use super::AppState;

#[derive(Deserialize)]
pub struct CreateKeyForm {
    pub name: Option<String>,
}

pub async fn settings_page(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    Extension(AuthUser(user)): Extension<AuthUser>,
) -> Response {
    let api_keys = match state.store.list_api_keys_for_user(user_id).await {
        Ok(keys) => keys,
        Err(e) => {
            tracing::error!("error listing api keys: {}", e);
            return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
        }
    };

    let mut ctx = tera::Context::new();
    ctx.insert("User", &user);
    ctx.insert("ActiveNav", "settings");
    ctx.insert("APIKeys", &api_keys);

    render_template(&state.templates, "pages/settings.html", &ctx)
}

pub async fn create_api_key(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    Extension(AuthUser(user)): Extension<AuthUser>,
    Form(form): Form<CreateKeyForm>,
) -> Response {
    let name = form
        .name
        .as_deref()
        .map(|s| s.trim())
        .filter(|s| !s.is_empty())
        .unwrap_or("Default")
        .to_string();

    let (full_key, key_hash, key_prefix) = match auth::generate_api_key() {
        Ok(k) => k,
        Err(e) => {
            tracing::error!("error generating api key: {}", e);
            return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
        }
    };

    let mut api_key = APIKey {
        id: Uuid::nil(),
        user_id,
        key_hash,
        key_prefix,
        name,
        created_at: chrono::Utc::now(),
        last_used_at: None,
        expires_at: None,
    };

    if let Err(e) = state.store.create_api_key(&mut api_key).await {
        tracing::error!("error creating api key: {}", e);
        return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
    }

    let api_keys = state
        .store
        .list_api_keys_for_user(user_id)
        .await
        .unwrap_or_default();

    let mut ctx = tera::Context::new();
    ctx.insert("User", &user);
    ctx.insert("ActiveNav", "settings");
    ctx.insert("APIKeys", &api_keys);
    ctx.insert("NewKey", &full_key);

    render_template(&state.templates, "pages/settings.html", &ctx)
}

pub async fn delete_api_key(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    axum::extract::Path(id): axum::extract::Path<Uuid>,
) -> Response {
    if let Err(e) = state.store.delete_api_key(user_id, id).await {
        tracing::error!("error deleting api key: {}", e);
        return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
    }

    Redirect::to("/settings").into_response()
}

#[derive(Deserialize)]
pub struct DeleteMethodForm {
    pub _method: Option<String>,
}

pub async fn delete_api_key_form(
    State(state): State<AppState>,
    Extension(auth_user_id): Extension<AuthUserId>,
    Extension(_auth_user): Extension<AuthUser>,
    path: axum::extract::Path<Uuid>,
    Form(form): Form<DeleteMethodForm>,
) -> Response {
    if form._method.as_deref() == Some("DELETE") {
        return delete_api_key(State(state), Extension(auth_user_id), path).await;
    }
    (StatusCode::METHOD_NOT_ALLOWED, "Method not allowed").into_response()
}
