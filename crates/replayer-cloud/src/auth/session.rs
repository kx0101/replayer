use aes_gcm::{
    aead::{Aead, OsRng},
    AeadCore, Aes256Gcm, KeyInit,
};
use anyhow::{anyhow, Result};
use axum::http::HeaderValue;
use base64::{engine::general_purpose::URL_SAFE, Engine};
use chrono::{Duration, Utc};

use crate::models::SessionData;

const SESSION_COOKIE_NAME: &str = "session";
const SESSION_DURATION_DAYS: i64 = 7;

pub struct SessionManager {
    key: Vec<u8>,
    secure_cookie: bool,
}

impl SessionManager {
    pub fn new(secret: &str, secure_cookie: bool) -> Result<Self> {
        if secret.len() < 32 {
            return Err(anyhow!("session secret must be at least 32 characters"));
        }
        Ok(SessionManager {
            key: secret.as_bytes()[..32].to_vec(),
            secure_cookie,
        })
    }

    pub fn create_session_cookie(&self, user_id: uuid::Uuid, email: &str) -> Result<HeaderValue> {
        let session = SessionData {
            user_id,
            email: email.to_string(),
            expires_at: Utc::now() + Duration::days(SESSION_DURATION_DAYS),
        };

        let data = serde_json::to_vec(&session)?;
        let encrypted = self.encrypt(&data)?;

        let max_age = SESSION_DURATION_DAYS * 24 * 60 * 60;
        let secure = if self.secure_cookie { "; Secure" } else { "" };
        let cookie = format!(
            "{}={}; Path=/; HttpOnly; SameSite=Lax; Max-Age={}{}",
            SESSION_COOKIE_NAME, encrypted, max_age, secure
        );

        Ok(HeaderValue::from_str(&cookie)?)
    }

    pub fn get_session(&self, cookie_header: Option<&str>) -> Result<SessionData> {
        let cookie_str = cookie_header.ok_or_else(|| anyhow!("no cookie header"))?;

        let value = cookie_str
            .split(';')
            .find_map(|part| {
                let part = part.trim();
                if part.starts_with(&format!("{}=", SESSION_COOKIE_NAME)) {
                    Some(part[SESSION_COOKIE_NAME.len() + 1..].to_string())
                } else {
                    None
                }
            })
            .ok_or_else(|| anyhow!("session cookie not found"))?;

        let data = self.decrypt(&value)?;
        let session: SessionData = serde_json::from_slice(&data)?;

        if Utc::now() > session.expires_at {
            return Err(anyhow!("session expired"));
        }

        Ok(session)
    }

    pub fn clear_session_cookie(&self) -> Result<HeaderValue> {
        let secure = if self.secure_cookie { "; Secure" } else { "" };
        let cookie = format!(
            "{}=; Path=/; HttpOnly; SameSite=Lax; Max-Age=-1{}",
            SESSION_COOKIE_NAME, secure
        );
        Ok(HeaderValue::from_str(&cookie)?)
    }

    fn encrypt(&self, plaintext: &[u8]) -> Result<String> {
        let key = aes_gcm::Key::<Aes256Gcm>::from_slice(&self.key);
        let cipher = Aes256Gcm::new(key);
        let nonce = Aes256Gcm::generate_nonce(&mut OsRng);

        let ciphertext = cipher
            .encrypt(&nonce, plaintext)
            .map_err(|e| anyhow!("encryption failed: {}", e))?;

        let mut combined = nonce.to_vec();
        combined.extend_from_slice(&ciphertext);

        Ok(URL_SAFE.encode(combined))
    }

    fn decrypt(&self, encoded: &str) -> Result<Vec<u8>> {
        let combined = URL_SAFE.decode(encoded)?;

        let key = aes_gcm::Key::<Aes256Gcm>::from_slice(&self.key);
        let cipher = Aes256Gcm::new(key);

        if combined.len() < 12 {
            return Err(anyhow!("ciphertext too short"));
        }

        let (nonce_bytes, ciphertext) = combined.split_at(12);
        let nonce = aes_gcm::Nonce::from_slice(nonce_bytes);

        cipher
            .decrypt(nonce, ciphertext)
            .map_err(|e| anyhow!("decryption failed: {}", e))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_new_session_manager_valid_secret() {
        let sm = SessionManager::new("12345678901234567890123456789012", false);
        assert!(sm.is_ok(), "valid 32-char secret should succeed");
    }

    #[test]
    fn test_new_session_manager_secret_too_short() {
        let sm = SessionManager::new("tooshort", false);
        assert!(sm.is_err(), "short secret should fail");
    }

    #[test]
    fn test_new_session_manager_long_secret_truncated() {
        let sm = SessionManager::new("12345678901234567890123456789012extra", true);
        assert!(sm.is_ok(), "long secret should work (truncated to 32)");
    }

    #[test]
    fn test_encrypt_decrypt() {
        let sm = SessionManager::new("12345678901234567890123456789012", false).unwrap();

        let plaintext = b"test data";
        let encrypted = sm.encrypt(plaintext).unwrap();
        let decrypted = sm.decrypt(&encrypted).unwrap();

        assert_eq!(decrypted, plaintext, "decrypted should match plaintext");
    }

    #[test]
    fn test_encrypt_produces_different_ciphertexts() {
        let sm = SessionManager::new("12345678901234567890123456789012", false).unwrap();
        let plaintext = b"test data";
        let enc1 = sm.encrypt(plaintext).unwrap();
        let enc2 = sm.encrypt(plaintext).unwrap();
        assert_ne!(enc1, enc2, "each encryption should use a unique nonce");
    }

    #[test]
    fn test_create_and_get_session() {
        let sm = SessionManager::new("12345678901234567890123456789012", false).unwrap();
        let user_id = uuid::Uuid::new_v4();
        let email = "test@example.com";

        let cookie = sm.create_session_cookie(user_id, email).unwrap();
        let cookie_str = cookie.to_str().unwrap();

        assert!(
            cookie_str.contains("HttpOnly"),
            "session cookie should be HttpOnly"
        );

        let session = sm.get_session(Some(cookie_str)).unwrap();
        assert_eq!(session.user_id, user_id);
        assert_eq!(session.email, email);
    }

    #[test]
    fn test_get_session_no_cookie() {
        let sm = SessionManager::new("12345678901234567890123456789012", false).unwrap();
        let result = sm.get_session(None);
        assert!(result.is_err(), "should fail with no cookie");
    }

    #[test]
    fn test_get_session_invalid_cookie() {
        let sm = SessionManager::new("12345678901234567890123456789012", false).unwrap();
        let result = sm.get_session(Some("session=invalid-session-value"));
        assert!(result.is_err(), "should fail with invalid cookie");
    }

    #[test]
    fn test_get_session_different_key() {
        let sm1 = SessionManager::new("12345678901234567890123456789012", false).unwrap();
        let sm2 = SessionManager::new("differentkey90123456789012345678", false).unwrap();

        let user_id = uuid::Uuid::new_v4();
        let cookie = sm1
            .create_session_cookie(user_id, "test@example.com")
            .unwrap();
        let cookie_str = cookie.to_str().unwrap();

        let result = sm2.get_session(Some(cookie_str));
        assert!(result.is_err(), "should fail with different key");
    }

    #[test]
    fn test_clear_session() {
        let sm = SessionManager::new("12345678901234567890123456789012", false).unwrap();
        let cookie = sm.clear_session_cookie().unwrap();
        let cookie_str = cookie.to_str().unwrap();

        assert!(
            cookie_str.contains("Max-Age=-1"),
            "expected Max-Age=-1 in cleared cookie"
        );
    }

    #[test]
    fn test_secure_cookie() {
        let sm = SessionManager::new("12345678901234567890123456789012", true).unwrap();
        let user_id = uuid::Uuid::new_v4();
        let cookie = sm
            .create_session_cookie(user_id, "test@example.com")
            .unwrap();
        let cookie_str = cookie.to_str().unwrap();

        assert!(cookie_str.contains("Secure"), "expected Secure flag");
    }

    #[test]
    fn test_session_expiry() {
        let sm = SessionManager::new("12345678901234567890123456789012", false).unwrap();
        let user_id = uuid::Uuid::new_v4();
        let cookie = sm
            .create_session_cookie(user_id, "test@example.com")
            .unwrap();
        let cookie_str = cookie.to_str().unwrap();

        let session = sm.get_session(Some(cookie_str)).unwrap();

        let expected_expiry = Utc::now() + Duration::days(SESSION_DURATION_DAYS);
        let diff = (session.expires_at - expected_expiry).num_seconds().abs();
        assert!(
            diff < 60,
            "session expiry should be within 60 seconds of expected, diff={}s",
            diff
        );
    }
}
