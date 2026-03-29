mod auth;
mod config;
mod handler;
mod middleware;
mod models;
mod store;

use std::sync::Arc;

use axum::middleware as axum_middleware;
use axum::{
    routing::{get, post},
    Router,
};
use sqlx::postgres::PgPoolOptions;
use tracing_subscriber::EnvFilter;

use crate::handler::AppState;
use crate::store::PostgresStore;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("info")),
        )
        .init();

    let cfg = config::Config::load()?;

    let pool = PgPoolOptions::new()
        .max_connections(10)
        .connect(&cfg.database_url)
        .await?;

    sqlx::query("SELECT 1").execute(&pool).await?;

    let pg_store = PostgresStore::new(pool);
    pg_store.migrate().await?;

    let session_manager = auth::SessionManager::new(&cfg.session_secret, cfg.secure_cookies)?;

    let email_sender = if !cfg.smtp_host.is_empty() {
        Some(Arc::new(auth::EmailSender::new(
            cfg.smtp_host.clone(),
            cfg.smtp_port,
            cfg.smtp_user.clone(),
            cfg.smtp_password.clone(),
            cfg.smtp_from.clone(),
            cfg.base_url.clone(),
        )))
    } else {
        None
    };

    let templates = handler::templates::load_templates()?;

    let state = AppState {
        store: Arc::new(pg_store),
        session_manager: Arc::new(session_manager),
        email_sender,
        templates: Arc::new(templates),
    };

    let api_routes = Router::new()
        .route(
            "/runs",
            post(handler::runs::create_run).get(handler::runs::list_runs),
        )
        .route("/runs/{id}", get(handler::runs::get_run))
        .route("/runs/{id}/baseline", post(handler::baseline::set_baseline))
        .route("/compare/{id}", get(handler::compare::compare_run))
        .layer(axum_middleware::from_fn_with_state(
            state.clone(),
            middleware::apikey::apikey_auth,
        ));

    let public_routes = Router::new()
        .route(
            "/login",
            get(handler::auth::login_page).post(handler::auth::login),
        )
        .route(
            "/register",
            get(handler::auth::register_page).post(handler::auth::register),
        )
        .route("/verify", get(handler::auth::verify_email))
        .route("/verify-pending", get(handler::auth::verify_pending_page))
        .layer(axum_middleware::from_fn_with_state(
            state.clone(),
            middleware::session::optional_session,
        ));

    let protected_routes = Router::new()
        .route("/", get(handler::web::dashboard))
        .route("/logout", post(handler::auth::logout))
        .route("/runs/{id}", get(handler::web::run_detail_page))
        .route("/runs/{id}/compare", get(handler::web::compare_view_page))
        .route("/settings", get(handler::settings::settings_page))
        .route(
            "/settings/api-keys",
            post(handler::settings::create_api_key),
        )
        .route(
            "/settings/api-keys/{id}",
            post(handler::settings::delete_api_key_form).delete(handler::settings::delete_api_key),
        )
        .route("/htmx/runs", get(handler::web::runs_list_partial))
        .route(
            "/htmx/runs/{id}/baseline",
            post(handler::web::set_baseline_htmx),
        )
        .layer(axum_middleware::from_fn_with_state(
            state.clone(),
            middleware::session::require_verified,
        ));

    let health = Router::new().route(
        "/health",
        get(|| async { axum::Json(serde_json::json!({"status": "ok"})) }),
    );

    let app = Router::new()
        .merge(health)
        .nest("/api/v1", api_routes)
        .merge(public_routes)
        .merge(protected_routes)
        .layer(axum_middleware::from_fn(middleware::logging::logging))
        .with_state(state);

    let listener = tokio::net::TcpListener::bind(&cfg.listen_addr).await?;
    tracing::info!("server listening on {}", cfg.listen_addr);

    axum::serve(listener, app)
        .with_graceful_shutdown(shutdown_signal())
        .await?;

    tracing::info!("server stopped");
    Ok(())
}

async fn shutdown_signal() {
    let ctrl_c = async {
        tokio::signal::ctrl_c()
            .await
            .expect("failed to install Ctrl+C handler");
    };

    #[cfg(unix)]
    let terminate = async {
        tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
            .expect("failed to install signal handler")
            .recv()
            .await;
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c => { tracing::info!("received Ctrl+C, shutting down..."); },
        _ = terminate => { tracing::info!("received SIGTERM, shutting down..."); },
    }
}
