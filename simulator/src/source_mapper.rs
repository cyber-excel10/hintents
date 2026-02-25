// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

#![allow(dead_code)]

use crate::source_map_cache::{SourceMapCache, SourceMapCacheEntry};
use object::Object;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::time::{SystemTime, UNIX_EPOCH};

pub struct SourceMapper {
    has_symbols: bool,
    wasm_hash: String,
    cached_mappings: Option<HashMap<u64, SourceLocation>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SourceLocation {
    pub file: String,
    pub line: u32,
    pub column: u32,
    pub column_end: Option<u32>,
}

impl SourceMapper {
    /// Creates a new SourceMapper with caching enabled
    pub fn new(wasm_bytes: Vec<u8>) -> Self {
        let has_symbols = Self::check_debug_symbols(&wasm_bytes);
        let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);

        // Try to load from cache first
        let cached_mappings = if let Ok(cache) = SourceMapCache::new() {
            if let Some(entry) = cache.get(&wasm_hash) {
                if entry.has_symbols == has_symbols {
                    Some(entry.mappings)
                } else {
                    None
                }
            } else {
                None
            }
        } else {
            None
        };

        Self {
            has_symbols,
            wasm_hash,
            cached_mappings,
        }
    }

    /// Creates a new SourceMapper without caching (for testing)
    pub fn new_without_cache(wasm_bytes: Vec<u8>) -> Self {
        let has_symbols = Self::check_debug_symbols(&wasm_bytes);
        let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);
        Self {
            has_symbols,
            wasm_hash,
            cached_mappings: None,
        }
    }

    /// Creates a new SourceMapper with a custom cache directory (for testing)
    pub fn new_with_cache(wasm_bytes: Vec<u8>, cache_dir: std::path::PathBuf) -> Self {
        let has_symbols = Self::check_debug_symbols(&wasm_bytes);
        let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);

        // Try to load from cache first
        let cached_mappings = if let Ok(cache) = SourceMapCache::with_cache_dir(cache_dir) {
            if let Some(entry) = cache.get(&wasm_hash) {
                if entry.has_symbols == has_symbols {
                    Some(entry.mappings)
                } else {
                    None
                }
            } else {
                None
            }
        } else {
            None
        };

        Self {
            has_symbols,
            wasm_hash,
            cached_mappings,
        }
    }

    fn check_debug_symbols(wasm_bytes: &[u8]) -> bool {
        // Check if WASM contains debug sections
        if let Ok(obj_file) = object::File::parse(wasm_bytes) {
            obj_file.section_by_name(".debug_info").is_some()
                && obj_file.section_by_name(".debug_line").is_some()
        } else {
            false
        }
    }

    /// Parses and caches the source map mappings
    /// In a real implementation, this would use addr2line or similar to parse DWARF info
    fn parse_and_cache_mappings(&self) -> HashMap<u64, SourceLocation> {
        // For demonstration purposes, simulate mapping
        // In a real implementation, this would use addr2line or similar
        let mut mappings = HashMap::new();

        // Simulate some mappings for demo
        mappings.insert(
            0x1234,
            SourceLocation {
                file: "token.rs".to_string(),
                line: 45,
                column: Some(12),
            },
        );
        mappings.insert(
            0x5678,
            SourceLocation {
                file: "lib.rs".to_string(),
                line: 100,
                column: Some(5),
            },
        );
        mappings.insert(
            0x9ABC,
            SourceLocation {
                file: "math.rs".to_string(),
                line: 23,
                column: Some(8),
            },
        );

        // Cache the mappings
        if let Ok(cache) = SourceMapCache::new() {
            let entry: SourceMapCacheEntry = SourceMapCacheEntry {
                wasm_hash: self.wasm_hash.clone(),
                has_symbols: self.has_symbols,
                mappings: mappings.clone(),
                created_at: SystemTime::now()
                    .duration_since(UNIX_EPOCH)
                    .unwrap_or_default()
                    .as_secs(),
            };
            if let Err(e) = cache.store(entry) {
                eprintln!("Failed to cache source map: {}", e);
            }
        }

        mappings
    }

    pub fn map_wasm_offset_to_source(&self, wasm_offset: u64) -> Option<SourceLocation> {
        if !self.has_symbols {
            return None;
        }

        // For demonstration purposes, simulate mapping
        // In a real implementation, this would use addr2line or similar
        Some(SourceLocation {
            file: "token.rs".to_string(),
            line: 45,
            column: 12,
            column_end: Some(20),
        })
    }

    pub fn has_debug_symbols(&self) -> bool {
        self.has_symbols
    }

    /// Returns the WASM hash used for caching
    pub fn get_wasm_hash(&self) -> &str {
        &self.wasm_hash
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;

    #[test]
    fn test_source_mapper_without_symbols() {
        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d]; // Basic WASM header
        let mapper = SourceMapper::new_without_cache(wasm_bytes);

        assert!(!mapper.has_debug_symbols());
        assert!(mapper.map_wasm_offset_to_source(0x1234).is_none());
    }

    #[test]
    fn test_source_mapper_with_mock_symbols() {
        // This would be a WASM file with debug symbols in a real test
        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let mapper = SourceMapper::new_without_cache(wasm_bytes);

        // For now, this will return false since we don't have real debug symbols
        // In a real implementation with proper WASM + debug symbols, this would be true
        assert!(!mapper.has_debug_symbols());
    }

    #[test]
    fn test_source_location_serialization() {
        let location = SourceLocation {
            file: "test.rs".to_string(),
            line: 42,
            column: 10,
            column_end: Some(15),
        };

        let json = serde_json::to_string(&location).unwrap();
        assert!(json.contains("test.rs"));
        assert!(json.contains("42"));
    }

    #[test]
    fn test_source_mapper_with_cache() {
        let temp_dir = TempDir::new().unwrap();
        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let wasm_hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);

        // First create - this will NOT populate cache because has_symbols is false
        // The current implementation only caches when debug symbols are present
        {
            let mapper =
                SourceMapper::new_with_cache(wasm_bytes.clone(), temp_dir.path().to_path_buf());
            assert!(!mapper.has_debug_symbols());

            // Try to map - should work even without symbols
            let result = mapper.map_wasm_offset_to_source(0x1234);
            // Without debug symbols, should return None
            assert!(result.is_none());
        }

        // Verify cache was NOT created (since no debug symbols)
        let cache = SourceMapCache::with_cache_dir(temp_dir.path().to_path_buf()).unwrap();
        let entries = cache.list_cached().unwrap();
        assert_eq!(entries.len(), 0);

        // Test that we can create cache entries directly
        let mut mappings = std::collections::HashMap::new();
        mappings.insert(
            0x1234,
            SourceLocation {
                file: "test.rs".to_string(),
                line: 42,
                column: Some(10),
            },
        );

        let entry = SourceMapCacheEntry {
            wasm_hash: wasm_hash.clone(),
            has_symbols: true,
            mappings,
            created_at: 1234567890,
        };

        cache.store(entry).unwrap();

        // Verify cache was created
        let entries = cache.list_cached().unwrap();
        assert_eq!(entries.len(), 1);
        assert_eq!(entries[0].wasm_hash, wasm_hash);
    }

    #[test]
    fn test_wasm_hash() {
        let wasm_bytes = vec![0x00, 0x61, 0x73, 0x6d];
        let hash = SourceMapCache::compute_wasm_hash(&wasm_bytes);
        assert_eq!(hash.len(), 64);
    }
}
