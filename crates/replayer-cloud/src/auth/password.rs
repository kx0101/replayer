use anyhow::Result;

const BCRYPT_COST: u32 = 12;

pub fn hash_password(password: &str) -> Result<String> {
    let hash = bcrypt::hash(password, BCRYPT_COST)?;
    Ok(hash)
}

pub fn check_password(password: &str, hash: &str) -> bool {
    bcrypt::verify(password, hash).unwrap_or(false)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_hash_password() {
        let password = "testpassword123";
        let hash = hash_password(password).unwrap();

        assert!(!hash.is_empty(), "hash should not be empty");
        assert_ne!(hash, password, "hash should not equal plaintext password");

        let hash2 = hash_password(password).unwrap();
        assert_ne!(
            hash, hash2,
            "same password should produce different hashes (bcrypt salt)"
        );
    }

    #[test]
    fn test_check_password() {
        let password = "testpassword123";
        let hash = hash_password(password).unwrap();

        let tests = vec![
            ("correct password", password, hash.as_str(), true),
            ("wrong password", "wrongpassword", hash.as_str(), false),
            ("empty password", "", hash.as_str(), false),
            ("invalid hash", password, "invalid", false),
        ];

        for (name, pwd, h, want) in tests {
            assert_eq!(check_password(pwd, h), want, "test case: {}", name);
        }
    }

    #[test]
    fn test_hash_password_with_empty_string() {
        let hash = hash_password("").unwrap();
        assert!(!hash.is_empty(), "hash of empty string should not be empty");
        assert!(
            check_password("", &hash),
            "check_password should return true for empty string with its hash"
        );
    }
}
