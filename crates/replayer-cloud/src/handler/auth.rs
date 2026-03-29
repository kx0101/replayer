use axum::{
    extract::State,
    http::StatusCode,
    response::{IntoResponse, Redirect, Response},
    Form,
};
use serde::Deserialize;

use crate::auth;
use crate::models::User;

use super::response::render_template;
use super::AppState;

#[derive(Deserialize)]
pub struct LoginForm {
    pub email: String,
    pub password: String,
}

#[derive(Deserialize)]
pub struct RegisterForm {
    pub email: String,
    pub password: String,
    pub password_confirm: String,
}

#[derive(Deserialize)]
pub struct VerifyQuery {
    pub token: Option<String>,
}

pub async fn login_page(State(state): State<AppState>) -> Response {
    let ctx = tera::Context::new();
    render_template(&state.templates, "pages/login.html", &ctx)
}

pub async fn login(State(state): State<AppState>, Form(form): Form<LoginForm>) -> Response {
    let email = form.email.trim().to_lowercase();
    let password = form.password;

    if email.is_empty() || password.is_empty() {
        return render_login_error(&state, "Email and password are required", &email);
    }

    let user = match state.store.get_user_by_email(&email).await {
        Ok(Some(u)) => u,
        Ok(None) => {
            return render_login_error(&state, "Invalid email or password", &email);
        }
        Err(e) => {
            tracing::error!("error getting user: {}", e);
            return render_login_error(&state, "Invalid email or password", &email);
        }
    };

    if !auth::check_password(&password, &user.password_hash) {
        return render_login_error(&state, "Invalid email or password", &email);
    }

    let cookie = match state
        .session_manager
        .create_session_cookie(user.id, &user.email)
    {
        Ok(c) => c,
        Err(e) => {
            tracing::error!("error creating session: {}", e);
            return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
        }
    };

    let redirect_to = if user.verified_at.is_none() {
        "/verify-pending"
    } else {
        "/"
    };

    let mut resp = Redirect::to(redirect_to).into_response();
    resp.headers_mut()
        .insert(axum::http::header::SET_COOKIE, cookie);
    resp
}

fn render_login_error(state: &AppState, error: &str, email: &str) -> Response {
    let mut ctx = tera::Context::new();
    ctx.insert("Error", error);
    ctx.insert("Email", email);
    render_template(&state.templates, "pages/login.html", &ctx)
}

pub async fn register_page(State(state): State<AppState>) -> Response {
    let ctx = tera::Context::new();
    render_template(&state.templates, "pages/register.html", &ctx)
}

pub async fn register(State(state): State<AppState>, Form(form): Form<RegisterForm>) -> Response {
    let email = form.email.trim().to_lowercase();
    let password = form.password;
    let password_confirm = form.password_confirm;

    if email.is_empty() || password.is_empty() {
        return render_register_error(&state, "Email and password are required", &email);
    }

    if password.len() < 8 {
        return render_register_error(&state, "Password must be at least 8 characters", &email);
    }

    if password != password_confirm {
        return render_register_error(&state, "Passwords do not match", &email);
    }

    match state.store.get_user_by_email(&email).await {
        Ok(Some(_)) => {
            return render_register_error(
                &state,
                "An account with this email already exists",
                &email,
            );
        }
        Err(e) => {
            tracing::error!("error checking existing user: {}", e);
            return render_register_error(&state, "An error occurred. Please try again.", &email);
        }
        Ok(None) => {}
    }

    let password_hash = match auth::hash_password(&password) {
        Ok(h) => h,
        Err(e) => {
            tracing::error!("error hashing password: {}", e);
            return render_register_error(&state, "An error occurred. Please try again.", &email);
        }
    };

    let verify_token: String = match auth::generate_verify_token() {
        Ok(t) => t,
        Err(e) => {
            tracing::error!("error generating verify token: {}", e);
            return render_register_error(&state, "An error occurred. Please try again.", &email);
        }
    };

    let mut user = User {
        id: uuid::Uuid::nil(),
        email: email.clone(),
        password_hash,
        verify_token: Some(verify_token.clone()),
        verified_at: None,
        created_at: chrono::Utc::now(),
        updated_at: chrono::Utc::now(),
    };

    if let Err(e) = state.store.create_user(&mut user).await {
        tracing::error!("error creating user: {}", e);
        return render_register_error(&state, "An error occurred. Please try again.", &email);
    }

    if let Some(ref sender) = state.email_sender {
        if sender.is_configured() {
            if let Err(e) = sender.send_verification_email(&email, &verify_token).await {
                tracing::error!("error sending verification email: {}", e);
            }
        } else {
            tracing::info!("verification token generated for new user (email sending disabled)");
        }
    } else {
        tracing::info!("verification token generated for new user (email sending disabled)");
    }

    let cookie = match state
        .session_manager
        .create_session_cookie(user.id, &user.email)
    {
        Ok(c) => c,
        Err(e) => {
            tracing::error!("error creating session: {}", e);
            return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
        }
    };

    let mut resp = Redirect::to("/verify-pending").into_response();
    resp.headers_mut()
        .insert(axum::http::header::SET_COOKIE, cookie);
    resp
}

fn render_register_error(state: &AppState, error: &str, email: &str) -> Response {
    let mut ctx = tera::Context::new();
    ctx.insert("Error", error);
    ctx.insert("Email", email);
    render_template(&state.templates, "pages/register.html", &ctx)
}

pub async fn verify_email(
    State(state): State<AppState>,
    axum::extract::Query(query): axum::extract::Query<VerifyQuery>,
) -> Response {
    let token = match query.token {
        Some(t) if !t.is_empty() => t,
        _ => return Redirect::to("/login").into_response(),
    };

    let user = match state.store.get_user_by_verify_token(&token).await {
        Ok(Some(u)) => u,
        _ => return Redirect::to("/login").into_response(),
    };

    if let Err(e) = state.store.verify_user(user.id).await {
        tracing::error!("error verifying user: {}", e);
        return Redirect::to("/login").into_response();
    }

    let mut resp = Redirect::to("/").into_response();
    if let Ok(cookie) = state
        .session_manager
        .create_session_cookie(user.id, &user.email)
    {
        resp.headers_mut()
            .insert(axum::http::header::SET_COOKIE, cookie);
    }
    resp
}

pub async fn verify_pending_page(State(state): State<AppState>) -> Response {
    let ctx = tera::Context::new();
    render_template(&state.templates, "pages/verify_pending.html", &ctx)
}

pub async fn logout(State(state): State<AppState>) -> Response {
    let mut resp = Redirect::to("/login").into_response();
    if let Ok(cookie) = state.session_manager.clear_session_cookie() {
        resp.headers_mut()
            .insert(axum::http::header::SET_COOKIE, cookie);
    }
    resp
}
