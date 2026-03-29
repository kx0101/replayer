use anyhow::Result;
use rand::RngCore;
use sha2::{Digest, Sha256};

const API_KEY_LENGTH: usize = 32;

pub fn generate_api_key() -> Result<(String, String, String)> {
    let mut bytes = [0u8; API_KEY_LENGTH];
    rand::thread_rng().fill_bytes(&mut bytes);

    let full_key = format!("rp_{}", hex::encode(bytes));
    let key_hash = hash_api_key(&full_key);
    let key_prefix = full_key[..11].to_string(); // "rp_" (3) + 8 hex chars

    Ok((full_key, key_hash, key_prefix))
}

pub fn hash_api_key(key: &str) -> String {
    let mut hasher = Sha256::new();
    hasher.update(key.as_bytes());
    hex::encode(hasher.finalize())
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashSet;

    #[test]
    fn test_generate_api_key() {
        let (full_key, key_hash, key_prefix) = generate_api_key().unwrap();

        assert!(
            full_key.starts_with("rp_"),
            "fullKey should start with 'rp_'"
        );

        // "rp_" (3) + 8 hex chars = 11
        assert_eq!(key_prefix.len(), 11);
        assert!(
            full_key.starts_with(&key_prefix),
            "fullKey should start with keyPrefix"
        );
        assert!(!key_hash.is_empty(), "keyHash should not be empty");
        assert_ne!(key_hash, full_key, "keyHash should not equal fullKey");
        assert_eq!(
            hash_api_key(&full_key),
            key_hash,
            "hash_api_key(fullKey) should equal keyHash"
        );
    }

    #[test]
    fn test_generate_api_key_unique() {
        let mut keys = HashSet::new();
        for _ in 0..100 {
            let (full_key, _, _) = generate_api_key().unwrap();
            assert!(
                keys.insert(full_key.clone()),
                "duplicate key generated: {}",
                full_key
            );
        }
    }

    #[test]
    fn test_hash_api_key() {
        let key = "rp_testkey1234567890abcdef";
        let hash1 = hash_api_key(key);
        let hash2 = hash_api_key(key);
        assert_eq!(hash1, hash2, "same key should produce same hash");

        let other_key = "rp_otherkey1234567890abcdef";
        let other_hash = hash_api_key(other_key);
        assert_ne!(
            hash1, other_hash,
            "different keys should produce different hashes"
        );

        assert_eq!(hash1.len(), 64, "hash length should be 64");

        for c in hash1.chars() {
            assert!(
                c.is_ascii_hexdigit() && !c.is_ascii_uppercase(),
                "hash contains non-lowercase-hex character: {}",
                c
            );
        }
    }

    #[test]
    fn test_api_key_format() {
        let (full_key, _, _) = generate_api_key().unwrap();

        // "rp_" (3) + hex of 32 bytes (64) = 67
        let expected_len = 3 + (API_KEY_LENGTH * 2);
        assert_eq!(full_key.len(), expected_len);

        let hex_part = &full_key[3..];
        for c in hex_part.chars() {
            assert!(
                c.is_ascii_hexdigit() && !c.is_ascii_uppercase(),
                "key contains non-lowercase-hex character: {}",
                c
            );
        }
    }
}
