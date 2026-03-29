use axum::{body::Body, http::Request, middleware::Next, response::Response};
use std::time::Instant;

pub async fn logging(req: Request<Body>, next: Next) -> Response {
    let method = req.method().clone();
    let path = req.uri().path().to_string();
    let start = Instant::now();

    let response = next.run(req).await;

    let status = response.status().as_u16();
    let elapsed = start.elapsed();

    tracing::info!("{} {} {} {:?}", method, path, status, elapsed);

    response
}
