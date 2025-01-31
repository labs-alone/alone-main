use std::sync::Arc;
use tokio::sync::{Mutex, RwLock};
use serde::{Serialize, Deserialize};
use thiserror::Error;
use std::collections::HashMap;

#[derive(Error, Debug)]
pub enum RuntimeError {
    #[error("Agent not found: {0}")]
    AgentNotFound(String),
    #[error("Memory allocation failed: {0}")]
    MemoryError(String),
    #[error("Task execution failed: {0}")]
    ExecutionError(String),
    #[error("Resource limit exceeded: {0}")]
    ResourceLimit(String),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RuntimeConfig {
    pub max_agents: usize,
    pub memory_limit: usize,
    pub timeout_ms: u64,
    pub enable_metrics: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AgentMetrics {
    pub memory_usage: usize,
    pub cpu_usage: f64,
    pub uptime_ms: u64,
    pub request_count: u64,
}

pub struct RuntimeEngine {
    config: RuntimeConfig,
    agents: Arc<RwLock<HashMap<String, AgentInstance>>>,
    metrics: Arc<Mutex<RuntimeMetrics>>,
}

#[derive(Debug)]
struct AgentInstance {
    id: String,
    state: AgentState,
    metrics: AgentMetrics,
}

#[derive(Debug, Clone, Copy)]
enum AgentState {
    Idle,
    Running,
    Paused,
    Error,
}

#[derive(Debug)]
struct RuntimeMetrics {
    total_memory_usage: usize,
    total_cpu_usage: f64,
    active_agents: usize,
    total_requests: u64,
}

impl RuntimeEngine {
    pub async fn new(config: RuntimeConfig) -> Self {
        Self {
            config,
            agents: Arc::new(RwLock::new(HashMap::new())),
            metrics: Arc::new(Mutex::new(RuntimeMetrics {
                total_memory_usage: 0,
                total_cpu_usage: 0.0,
                active_agents: 0,
                total_requests: 0,
            })),
        }
    }

    pub async fn register_agent(&self, id: String) -> Result<(), RuntimeError> {
        let mut agents = self.agents.write().await;
        
        if agents.len() >= self.config.max_agents {
            return Err(RuntimeError::ResourceLimit(
                "Maximum number of agents reached".to_string()
            ));
        }

        agents.insert(id.clone(), AgentInstance {
            id,
            state: AgentState::Idle,
            metrics: AgentMetrics {
                memory_usage: 0,
                cpu_usage: 0.0,
                uptime_ms: 0,
                request_count: 0,
            },
        });

        let mut metrics = self.metrics.lock().await;
        metrics.active_agents += 1;

        Ok(())
    }

    pub async fn execute_task(&self, agent_id: &str, task: Vec<u8>) -> Result<Vec<u8>, RuntimeError> {
        let agents = self.agents.read().await;
        let agent = agents.get(agent_id).ok_or_else(|| 
            RuntimeError::AgentNotFound(agent_id.to_string())
        )?;

        // Simulate task execution
        tokio::time::sleep(tokio::time::Duration::from_millis(100)).await;

        let mut metrics = self.metrics.lock().await;
        metrics.total_requests += 1;

        Ok(task) // Echo back the task for simulation
    }

    pub async fn get_agent_metrics(&self, agent_id: &str) -> Result<AgentMetrics, RuntimeError> {
        let agents = self.agents.read().await;
        let agent = agents.get(agent_id).ok_or_else(|| 
            RuntimeError::AgentNotFound(agent_id.to_string())
        )?;

        Ok(agent.metrics.clone())
    }

    pub async fn get_runtime_metrics(&self) -> RuntimeMetrics {
        self.metrics.lock().await.clone()
    }

    pub async fn shutdown_agent(&self, agent_id: &str) -> Result<(), RuntimeError> {
        let mut agents = self.agents.write().await;
        agents.remove(agent_id).ok_or_else(|| 
            RuntimeError::AgentNotFound(agent_id.to_string())
        )?;

        let mut metrics = self.metrics.lock().await;
        metrics.active_agents -= 1;

        Ok(())
    }

    pub async fn pause_agent(&self, agent_id: &str) -> Result<(), RuntimeError> {
        let mut agents = self.agents.write().await;
        let agent = agents.get_mut(agent_id).ok_or_else(|| 
            RuntimeError::AgentNotFound(agent_id.to_string())
        )?;

        agent.state = AgentState::Paused;
        Ok(())
    }

    pub async fn resume_agent(&self, agent_id: &str) -> Result<(), RuntimeError> {
        let mut agents = self.agents.write().await;
        let agent = agents.get_mut(agent_id).ok_or_else(|| 
            RuntimeError::AgentNotFound(agent_id.to_string())
        )?;

        agent.state = AgentState::Running;
        Ok(())
    }

    #[cfg(test)]
    pub async fn get_agent_count(&self) -> usize {
        self.agents.read().await.len()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_agent_registration() {
        let runtime = RuntimeEngine::new(RuntimeConfig {
            max_agents: 10,
            memory_limit: 1024 * 1024,
            timeout_ms: 1000,
            enable_metrics: true,
        }).await;

        assert!(runtime.register_agent("test-agent".to_string()).await.is_ok());
        assert_eq!(runtime.get_agent_count().await, 1);
    }

    #[tokio::test]
    async fn test_agent_limit() {
        let runtime = RuntimeEngine::new(RuntimeConfig {
            max_agents: 1,
            memory_limit: 1024 * 1024,
            timeout_ms: 1000,
            enable_metrics: true,
        }).await;

        assert!(runtime.register_agent("agent1".to_string()).await.is_ok());
        assert!(runtime.register_agent("agent2".to_string()).await.is_err());
    }
}