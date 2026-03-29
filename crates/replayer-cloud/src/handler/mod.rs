pub mod auth;
pub mod baseline;
pub mod compare;
pub mod response;
pub mod runs;
pub mod settings;
pub mod templates;
pub mod web;

use std::sync::Arc;

use crate::auth::{EmailSender, SessionManager};
use crate::store::Store;

#[derive(Clone)]
pub struct AppState {
    pub store: Arc<dyn Store>,
    pub session_manager: Arc<SessionManager>,
    pub email_sender: Option<Arc<EmailSender>>,
    pub templates: Arc<tera::Tera>,
}
