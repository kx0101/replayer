use axum::{
    extract::State,
    http::StatusCode,
    response::{Html, IntoResponse, Response},
    Extension,
};
use serde::Deserialize;
use uuid::Uuid;

use crate::middleware::{AuthUser, AuthUserId};
use crate::store::ListFilter;

use super::compare::build_comparison;
use super::response::render_template;
use super::AppState;

#[derive(Deserialize)]
pub struct DashboardQuery {
    pub environment: Option<String>,
    pub page: Option<i64>,
}

pub async fn dashboard(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    Extension(AuthUser(user)): Extension<AuthUser>,
    axum::extract::Query(params): axum::extract::Query<DashboardQuery>,
) -> Response {
    let limit: i64 = 20;
    let page = params.page.unwrap_or(1).max(1);
    let offset = (page - 1) * limit;

    let filter = ListFilter {
        environment: params.environment.clone(),
        limit,
        offset,
        ..Default::default()
    };

    let (runs, total) = match state.store.list_runs_for_user(user_id, filter).await {
        Ok(r) => r,
        Err(e) => {
            tracing::error!("error listing runs: {}", e);
            return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
        }
    };

    let environments = get_distinct_environments(&state, user_id).await;
    let total_pages = (total + limit - 1) / limit;

    let mut ctx = tera::Context::new();
    ctx.insert("User", &user);
    ctx.insert("ActiveNav", "dashboard");
    ctx.insert("Runs", &runs);
    ctx.insert("Total", &total);
    ctx.insert("Page", &page);
    ctx.insert("TotalPages", &total_pages);
    ctx.insert("Limit", &limit);
    ctx.insert("Offset", &offset);
    ctx.insert(
        "Environment",
        &params.environment.clone().unwrap_or_default(),
    );
    ctx.insert("Environments", &environments);

    render_template(&state.templates, "pages/dashboard.html", &ctx)
}

async fn get_distinct_environments(state: &AppState, user_id: Uuid) -> Vec<String> {
    let filter = ListFilter {
        limit: 100,
        ..Default::default()
    };

    match state.store.list_runs_for_user(user_id, filter).await {
        Ok((runs, _)) => {
            let mut env_set = std::collections::HashSet::new();
            for r in &runs {
                env_set.insert(r.environment.clone());
            }
            env_set.into_iter().collect()
        }
        Err(_) => Vec::new(),
    }
}

pub async fn run_detail_page(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    Extension(AuthUser(user)): Extension<AuthUser>,
    axum::extract::Path(id): axum::extract::Path<Uuid>,
) -> Response {
    let run = match state.store.get_run_for_user(user_id, id).await {
        Ok(Some(r)) => r,
        Ok(None) => return (StatusCode::NOT_FOUND, "Run not found").into_response(),
        Err(e) => {
            tracing::error!("error getting run: {}", e);
            return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
        }
    };

    let mut ctx = tera::Context::new();
    ctx.insert("User", &user);
    ctx.insert("ActiveNav", "dashboard");
    ctx.insert("Run", &run);

    render_template(&state.templates, "pages/run_detail.html", &ctx)
}

pub async fn compare_view_page(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    Extension(AuthUser(user)): Extension<AuthUser>,
    axum::extract::Path(id): axum::extract::Path<Uuid>,
) -> Response {
    let run = match state.store.get_run_for_user(user_id, id).await {
        Ok(Some(r)) => r,
        Ok(None) => return (StatusCode::NOT_FOUND, "Run not found").into_response(),
        Err(e) => {
            tracing::error!("error getting run: {}", e);
            return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
        }
    };

    let baseline = match state
        .store
        .get_baseline_for_user(user_id, &run.environment)
        .await
    {
        Ok(b) => b,
        Err(e) => {
            tracing::error!("error getting baseline: {}", e);
            return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
        }
    };

    let mut ctx = tera::Context::new();
    ctx.insert("User", &user);
    ctx.insert("ActiveNav", "dashboard");
    ctx.insert("Run", &run);
    ctx.insert("NoBaseline", &baseline.is_none());

    if let Some(ref bl) = baseline {
        let comparison = build_comparison(&run, bl);
        ctx.insert("Baseline", bl);
        ctx.insert("Comparison", &comparison);
    }

    render_template(&state.templates, "pages/compare.html", &ctx)
}

#[derive(Deserialize)]
pub struct RunsListQuery {
    pub environment: Option<String>,
}

pub async fn runs_list_partial(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    axum::extract::Query(params): axum::extract::Query<RunsListQuery>,
) -> Response {
    let filter = ListFilter {
        environment: params.environment,
        limit: 20,
        ..Default::default()
    };

    let (runs, total) = match state.store.list_runs_for_user(user_id, filter).await {
        Ok(r) => r,
        Err(e) => {
            tracing::error!("error listing runs: {}", e);
            return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
        }
    };

    let mut ctx = tera::Context::new();
    ctx.insert("Runs", &runs);
    ctx.insert("Total", &total);

    match state.templates.render("partials/runs_list.html", &ctx) {
        Ok(html) => Html(html).into_response(),
        Err(e) => {
            tracing::error!("template error: {}", e);
            (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response()
        }
    }
}

pub async fn set_baseline_htmx(
    State(state): State<AppState>,
    Extension(AuthUserId(user_id)): Extension<AuthUserId>,
    axum::extract::Path(id): axum::extract::Path<Uuid>,
) -> Response {
    if let Err(e) = state.store.set_baseline_for_user(user_id, id).await {
        tracing::error!("error setting baseline: {}", e);
        return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
    }

    let run = match state.store.get_run_for_user(user_id, id).await {
        Ok(Some(r)) => r,
        Ok(None) | Err(_) => {
            return (StatusCode::INTERNAL_SERVER_ERROR, "Internal Server Error").into_response();
        }
    };

    let success_rate = if run.total_requests > 0 {
        (run.succeeded as f64) / (run.total_requests as f64) * 100.0
    } else {
        0.0
    };

    let created_str = run.created_at.format("%b %d, %H:%M").to_string();

    let success_html = if run.failed == 0 {
        r#"<span class="inline-flex items-center rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800">100%</span>"#.to_string()
    } else {
        format!(
            r#"<span class="inline-flex items-center rounded-full bg-yellow-100 px-2.5 py-0.5 text-xs font-medium text-yellow-800">{:.1}%</span>"#,
            success_rate
        )
    };

    let html = format!(
        r#"<tr>
            <td class="whitespace-nowrap py-4 pl-4 pr-3 text-sm font-medium text-gray-900 sm:pl-6">{env}</td>
            <td class="whitespace-nowrap px-3 py-4 text-sm text-gray-500">{created}</td>
            <td class="whitespace-nowrap px-3 py-4 text-sm text-gray-500">{total}</td>
            <td class="whitespace-nowrap px-3 py-4 text-sm">{success}</td>
            <td class="whitespace-nowrap px-3 py-4 text-sm text-gray-500">{p95}ms</td>
            <td class="whitespace-nowrap px-3 py-4 text-sm">
                <span class="inline-flex items-center rounded-full bg-indigo-100 px-2.5 py-0.5 text-xs font-medium text-indigo-800">Baseline</span>
            </td>
            <td class="relative whitespace-nowrap py-4 pl-3 pr-4 text-right text-sm font-medium sm:pr-6">
                <a href="/runs/{id}" class="text-indigo-600 hover:text-indigo-900">View</a>
            </td>
        </tr>"#,
        env = run.environment,
        created = created_str,
        total = run.total_requests,
        success = success_html,
        p95 = run.latency_stats.p95,
        id = run.id,
    );

    Html(html).into_response()
}
