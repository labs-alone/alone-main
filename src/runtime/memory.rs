use std::sync::Arc;
use tokio::sync::RwLock;
use std::collections::HashMap;
use serde::{Serialize, Deserialize};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum MemoryError {
    #[error("Memory limit exceeded: {0}")]
    LimitExceeded(String),
    #[error("Key not found: {0}")]
    KeyNotFound(String),
    #[error("Invalid operation: {0}")]
    InvalidOperation(String),
    #[error("Serialization error: {0}")]
    SerializationError(String),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MemoryStats {
    total_allocated: usize,
    total_freed: usize,
    current_usage: usize,
    peak_usage: usize,
    allocation_count: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct MemoryBlock {
    data: Vec<u8>,
    timestamp: std::time::SystemTime,
    ttl: Option<std::time::Duration>,
    metadata: HashMap<String, String>,
}

pub struct MemoryManager {
    memory_limit: usize,
    storage: Arc<RwLock<HashMap<String, MemoryBlock>>>,
    stats: Arc<RwLock<MemoryStats>>,
}

impl MemoryManager {
    pub async fn new(memory_limit: usize) -> Self {
        Self {
            memory_limit,
            storage: Arc::new(RwLock::new(HashMap::new())),
            stats: Arc::new(RwLock::new(MemoryStats {
                total_allocated: 0,
                total_freed: 0,
                current_usage: 0,
                peak_usage: 0,
                allocation_count: 0,
            })),
        }
    }

    pub async fn allocate(
        &self,
        key: String,
        data: Vec<u8>,
        ttl: Option<std::time::Duration>,
        metadata: HashMap<String, String>,
    ) -> Result<(), MemoryError> {
        let mut storage = self.storage.write().await;
        let mut stats = self.stats.write().await;

        let block_size = data.len();

        // Check memory limit
        if stats.current_usage + block_size > self.memory_limit {
            return Err(MemoryError::LimitExceeded(format!(
                "Memory limit of {} bytes exceeded",
                self.memory_limit
            )));
        }

        // Create memory block
        let block = MemoryBlock {
            data,
            timestamp: std::time::SystemTime::now(),
            ttl,
            metadata,
        };

        // Update stats
        stats.total_allocated += block_size;
        stats.current_usage += block_size;
        stats.allocation_count += 1;
        stats.peak_usage = stats.peak_usage.max(stats.current_usage);

        // Store the block
        storage.insert(key, block);

        Ok(())
    }

    pub async fn free(&self, key: &str) -> Result<(), MemoryError> {
        let mut storage = self.storage.write().await;
        let mut stats = self.stats.write().await;

        if let Some(block) = storage.remove(key) {
            stats.total_freed += block.data.len();
            stats.current_usage -= block.data.len();
            Ok(())
        } else {
            Err(MemoryError::KeyNotFound(key.to_string()))
        }
    }

    pub async fn get(&self, key: &str) -> Result<Vec<u8>, MemoryError> {
        let storage = self.storage.read().await;

        if let Some(block) = storage.get(key) {
            // Check TTL if set
            if let Some(ttl) = block.ttl {
                let age = block.timestamp
                    .elapsed()
                    .map_err(|e| MemoryError::InvalidOperation(e.to_string()))?;
                
                if age > ttl {
                    return Err(MemoryError::KeyNotFound(
                        "Key expired".to_string()
                    ));
                }
            }
            Ok(block.data.clone())
        } else {
            Err(MemoryError::KeyNotFound(key.to_string()))
        }
    }

    pub async fn get_stats(&self) -> MemoryStats {
        self.stats.read().await.clone()
    }

    pub async fn cleanup_expired(&self) -> Result<usize, MemoryError> {
        let mut storage = self.storage.write().await;
        let mut stats = self.stats.write().await;
        let mut cleaned = 0;

        storage.retain(|_, block| {
            if let Some(ttl) = block.ttl {
                if let Ok(age) = block.timestamp.elapsed() {
                    if age <= ttl {
                        return true;
                    }
                    stats.current_usage -= block.data.len();
                    stats.total_freed += block.data.len();
                    cleaned += 1;
                    return false;
                }
            }
            true
        });

        Ok(cleaned)
    }

    pub async fn add_metadata(
        &self,
        key: &str,
        metadata_key: String,
        metadata_value: String,
    ) -> Result<(), MemoryError> {
        let mut storage = self.storage.write().await;

        if let Some(block) = storage.get_mut(key) {
            block.metadata.insert(metadata_key, metadata_value);
            Ok(())
        } else {
            Err(MemoryError::KeyNotFound(key.to_string()))
        }
    }

    pub async fn get_metadata(&self, key: &str) -> Result<HashMap<String, String>, MemoryError> {
        let storage = self.storage.read().await;

        if let Some(block) = storage.get(key) {
            Ok(block.metadata.clone())
        } else {
            Err(MemoryError::KeyNotFound(key.to_string()))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::time::{sleep, Duration};

    #[tokio::test]
    async fn test_memory_allocation() {
        let manager = MemoryManager::new(1024).await;
        let data = vec![0u8; 100];
        let metadata = HashMap::new();

        assert!(manager.allocate(
            "test".to_string(),
            data,
            None,
            metadata
        ).await.is_ok());

        let stats = manager.get_stats().await;
        assert_eq!(stats.current_usage, 100);
    }

    #[tokio::test]
    async fn test_memory_limit() {
        let manager = MemoryManager::new(50).await;
        let data = vec![0u8; 100];
        let metadata = HashMap::new();

        assert!(manager.allocate(
            "test".to_string(),
            data,
            None,
            metadata
        ).await.is_err());
    }

    #[tokio::test]
    async fn test_ttl() {
        let manager = MemoryManager::new(1024).await;
        let data = vec![0u8; 100];
        let metadata = HashMap::new();

        assert!(manager.allocate(
            "test".to_string(),
            data,
            Some(Duration::from_millis(100)),
            metadata
        ).await.is_ok());

        sleep(Duration::from_millis(200)).await;
        assert!(manager.get("test").await.is_err());
    }
}