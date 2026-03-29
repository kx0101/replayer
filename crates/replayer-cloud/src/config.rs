use anyhow::{bail, Result};

#[derive(Clone)]
pub struct Config {
    pub database_url: String,
    pub listen_addr: String,
    pub session_secret: String,
    pub secure_cookies: bool,
    pub base_url: String,

    pub smtp_host: String,
    pub smtp_port: u16,
    pub smtp_user: String,
    pub smtp_password: String,
    pub smtp_from: String,
}

impl Config {
    pub fn load() -> Result<Self> {
        let database_url = std::env::var("DATABASE_URL")
            .map_err(|_| anyhow::anyhow!("DATABASE_URL is required"))?;

        let session_secret = std::env::var("SESSION_SECRET")
            .map_err(|_| anyhow::anyhow!("SESSION_SECRET is required (minimum 32 characters)"))?;

        if session_secret.len() < 32 {
            bail!("SESSION_SECRET must be at least 32 characters");
        }

        let base_url =
            std::env::var("BASE_URL").unwrap_or_else(|_| "http://localhost:8090".to_string());

        let listen_addr =
            std::env::var("LISTEN_ADDR").unwrap_or_else(|_| "0.0.0.0:8090".to_string());

        let secure_cookies = std::env::var("SECURE_COOKIES")
            .map(|v| v == "true")
            .unwrap_or(false);

        let smtp_port: u16 = std::env::var("SMTP_PORT")
            .ok()
            .and_then(|v| v.parse().ok())
            .unwrap_or(587);

        Ok(Config {
            database_url,
            listen_addr,
            session_secret,
            secure_cookies,
            base_url,
            smtp_host: std::env::var("SMTP_HOST").unwrap_or_default(),
            smtp_port,
            smtp_user: std::env::var("SMTP_USER").unwrap_or_default(),
            smtp_password: std::env::var("SMTP_PASSWORD").unwrap_or_default(),
            smtp_from: std::env::var("SMTP_FROM").unwrap_or_default(),
        })
    }
}
