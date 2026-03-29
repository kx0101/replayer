use anyhow::Result;
use lettre::{
    message::header::ContentType, transport::smtp::authentication::Credentials, AsyncSmtpTransport,
    AsyncTransport, Message, Tokio1Executor,
};
use rand::RngCore;

pub struct EmailSender {
    host: String,
    port: u16,
    user: String,
    password: String,
    from: String,
    base_url: String,
}

impl EmailSender {
    pub fn new(
        host: String,
        port: u16,
        user: String,
        password: String,
        from: String,
        base_url: String,
    ) -> Self {
        EmailSender {
            host,
            port,
            user,
            password,
            from,
            base_url,
        }
    }

    pub async fn send_verification_email(&self, to: &str, token: &str) -> Result<()> {
        let verify_url = format!("{}/verify?token={}", self.base_url, token);

        let body = format!(
            "Hello,\n\n\
             Please verify your email address by clicking the link below:\n\n\
             {}\n\n\
             If you didn't create an account, you can safely ignore this email.\n\n\
             Thanks,\nReplayer Cloud",
            verify_url
        );

        let email = Message::builder()
            .from(self.from.parse()?)
            .to(to.parse()?)
            .subject("Verify your Replayer Cloud account")
            .header(ContentType::TEXT_PLAIN)
            .body(body)?;

        let creds = Credentials::new(self.user.clone(), self.password.clone());

        let mailer = AsyncSmtpTransport::<Tokio1Executor>::relay(&self.host)?
            .port(self.port)
            .credentials(creds)
            .build();

        mailer.send(email).await?;
        Ok(())
    }

    pub fn is_configured(&self) -> bool {
        !self.host.is_empty() && !self.from.is_empty()
    }
}

pub fn generate_verify_token() -> Result<String> {
    let mut bytes = [0u8; 32];
    rand::thread_rng().fill_bytes(&mut bytes);
    Ok(hex::encode(bytes))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashSet;

    #[test]
    fn test_new_email_sender() {
        let sender = EmailSender::new(
            "smtp.example.com".to_string(),
            587,
            "user@example.com".to_string(),
            "password".to_string(),
            "noreply@example.com".to_string(),
            "https://replayer.example.com".to_string(),
        );
        assert_eq!(sender.host, "smtp.example.com");
        assert_eq!(sender.port, 587);
    }

    #[test]
    fn test_email_sender_is_configured() {
        let tests = vec![
            (
                "fully configured",
                "smtp.example.com",
                "noreply@example.com",
                true,
            ),
            ("missing host", "", "noreply@example.com", false),
            ("missing from", "smtp.example.com", "", false),
            ("both missing", "", "", false),
        ];

        for (name, host, from, want) in tests {
            let sender = EmailSender::new(
                host.to_string(),
                587,
                String::new(),
                String::new(),
                from.to_string(),
                String::new(),
            );
            assert_eq!(sender.is_configured(), want, "test case: {}", name);
        }
    }

    #[test]
    fn test_generate_verify_token() {
        let token1 = generate_verify_token().unwrap();
        assert_eq!(token1.len(), 64, "token length should be 64");

        for c in token1.chars() {
            assert!(
                c.is_ascii_hexdigit() && !c.is_ascii_uppercase(),
                "token contains non-hex character: {}",
                c
            );
        }

        let token2 = generate_verify_token().unwrap();
        assert_ne!(token1, token2, "tokens should be unique");
    }

    #[test]
    fn test_generate_verify_token_unique() {
        let mut tokens = HashSet::new();
        for _ in 0..100 {
            let token = generate_verify_token().unwrap();
            assert!(
                tokens.insert(token.clone()),
                "duplicate token generated: {}",
                token
            );
        }
    }

    #[test]
    fn test_email_message_format() {
        let sender = EmailSender::new(
            "smtp.example.com".to_string(),
            587,
            "user@example.com".to_string(),
            "password".to_string(),
            "noreply@example.com".to_string(),
            "https://replayer.example.com".to_string(),
        );

        let token = "abc123";
        let expected_url = "https://replayer.example.com/verify?token=abc123";
        let url = format!("{}/verify?token={}", sender.base_url, token);
        assert!(
            url.contains(expected_url),
            "verify URL not constructed correctly"
        );
    }
}
