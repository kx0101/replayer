use axum::{
    body::Body,
    extract::State,
    http::Request,
    middleware::Next,
    response::{IntoResponse, Redirect, Response},
};

use crate::handler::AppState;
use crate::models::{SessionData, User};

use super::apikey::AuthUserId;

#[derive(Clone, Debug)]
pub struct AuthSession(#[allow(dead_code)] pub SessionData);

#[derive(Clone, Debug)]
pub struct AuthUser(pub User);

pub async fn optional_session(
    State(state): State<AppState>,
    mut req: Request<Body>,
    next: Next,
) -> Response {
    let cookie_header = req.headers().get("cookie").and_then(|v| v.to_str().ok());

    if let Ok(session) = state.session_manager.get_session(cookie_header) {
        req.extensions_mut().insert(AuthUserId(session.user_id));
        req.extensions_mut().insert(AuthSession(session));
    }

    next.run(req).await
}

pub async fn require_verified(
    State(state): State<AppState>,
    mut req: Request<Body>,
    next: Next,
) -> Response {
    let cookie_header = req.headers().get("cookie").and_then(|v| v.to_str().ok());

    let session = match state.session_manager.get_session(cookie_header) {
        Ok(s) => s,
        Err(_) => return Redirect::to("/login").into_response(),
    };

    let user = match state.store.get_user_by_id(session.user_id).await {
        Ok(Some(u)) => u,
        _ => {
            let mut resp = Redirect::to("/login").into_response();
            if let Ok(clear) = state.session_manager.clear_session_cookie() {
                resp.headers_mut()
                    .insert(axum::http::header::SET_COOKIE, clear);
            }
            return resp;
        }
    };

    if user.verified_at.is_none() {
        return Redirect::to("/verify-pending").into_response();
    }

    req.extensions_mut().insert(AuthUserId(session.user_id));
    req.extensions_mut().insert(AuthSession(session));
    req.extensions_mut().insert(AuthUser(user));

    next.run(req).await
}
