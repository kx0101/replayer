use axum::http::StatusCode;
use axum::response::{Html, IntoResponse, Response};
use serde::Serialize;

pub fn respond_json<T: Serialize>(status: StatusCode, data: &T) -> Response {
    let body = serde_json::to_string(data)
        .unwrap_or_else(|_| r#"{"error":"serialization error"}"#.to_string());
    (
        status,
        [(axum::http::header::CONTENT_TYPE, "application/json")],
        body,
    )
        .into_response()
}

pub fn respond_error(status: StatusCode, msg: &str) -> Response {
    let body = serde_json::json!({"error": msg});
    (
        status,
        [(axum::http::header::CONTENT_TYPE, "application/json")],
        body.to_string(),
    )
        .into_response()
}

pub fn render_template(templates: &tera::Tera, name: &str, ctx: &tera::Context) -> Response {
    match templates.render(name, ctx) {
        Ok(html) => Html(html).into_response(),
        Err(e) => {
            tracing::error!("template render error for {}: {}", name, e);
            (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response()
        }
    }
}
