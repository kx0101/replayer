mod postgres;

pub use postgres::PostgresStore;

use anyhow::Result;
use chrono::{DateTime, Utc};
use uuid::Uuid;

use crate::models::{APIKey, Run, RunListItem, User};

#[derive(Debug, Clone, Default)]
#[allow(dead_code)]
pub struct ListFilter {
    pub environment: Option<String>,
    pub after: Option<DateTime<Utc>>,
    pub before: Option<DateTime<Utc>>,
    pub limit: i64,
    pub offset: i64,
}

impl ListFilter {
    pub fn normalize(&mut self) {
        if self.limit <= 0 {
            self.limit = 20;
        }
        if self.limit > 100 {
            self.limit = 100;
        }
        if self.offset < 0 {
            self.offset = 0;
        }
    }
}

#[async_trait::async_trait]
#[allow(dead_code)]
pub trait Store: Send + Sync {
    async fn create_run(&self, run: &mut Run) -> Result<()>;
    async fn get_run(&self, id: Uuid) -> Result<Option<Run>>;
    async fn list_runs(&self, filter: ListFilter) -> Result<(Vec<RunListItem>, i64)>;
    async fn set_baseline(&self, id: Uuid) -> Result<()>;
    async fn get_baseline(&self, environment: &str) -> Result<Option<Run>>;

    async fn create_run_for_user(&self, user_id: Uuid, run: &mut Run) -> Result<()>;
    async fn get_run_for_user(&self, user_id: Uuid, run_id: Uuid) -> Result<Option<Run>>;
    async fn list_runs_for_user(
        &self,
        user_id: Uuid,
        filter: ListFilter,
    ) -> Result<(Vec<RunListItem>, i64)>;
    async fn set_baseline_for_user(&self, user_id: Uuid, run_id: Uuid) -> Result<()>;
    async fn get_baseline_for_user(&self, user_id: Uuid, env: &str) -> Result<Option<Run>>;

    async fn create_user(&self, user: &mut User) -> Result<()>;
    async fn get_user_by_email(&self, email: &str) -> Result<Option<User>>;
    async fn get_user_by_id(&self, id: Uuid) -> Result<Option<User>>;
    async fn get_user_by_verify_token(&self, token: &str) -> Result<Option<User>>;
    async fn verify_user(&self, user_id: Uuid) -> Result<()>;

    async fn create_api_key(&self, key: &mut APIKey) -> Result<()>;
    async fn get_api_key_by_hash(&self, hash: &str) -> Result<Option<APIKey>>;
    async fn list_api_keys_for_user(&self, user_id: Uuid) -> Result<Vec<APIKey>>;
    async fn delete_api_key(&self, user_id: Uuid, key_id: Uuid) -> Result<()>;
    async fn update_api_key_last_used(&self, key_id: Uuid) -> Result<()>;
}
