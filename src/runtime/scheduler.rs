use std::sync::Arc;
use tokio::sync::{mpsc, RwLock};
use tokio::time::{Duration, Instant};
use std::collections::{HashMap, BinaryHeap};
use serde::{Serialize, Deserialize};
use thiserror::Error;
use std::cmp::Ordering;

#[derive(Error, Debug)]
pub enum SchedulerError {
    #[error("Task not found: {0}")]
    TaskNotFound(String),
    #[error("Queue full: {0}")]
    QueueFull(String),
    #[error("Invalid schedule: {0}")]
    InvalidSchedule(String),
    #[error("Task execution failed: {0}")]
    ExecutionError(String),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TaskConfig {
    pub id: String,
    pub priority: u8,
    pub max_retries: u32,
    pub timeout: Duration,
    pub dependencies: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TaskStats {
    pub total_executed: u64,
    pub total_failed: u64,
    pub average_duration: Duration,
    pub last_execution: Option<Instant>,
}

#[derive(Debug, Clone, Eq, PartialEq)]
struct Task {
    id: String,
    priority: u8,
    scheduled_time: Instant,
    config: TaskConfig,
    retries: u32,
}

impl Ord for Task {
    fn cmp(&self, other: &Self) -> Ordering {
        self.priority.cmp(&other.priority)
            .then_with(|| other.scheduled_time.cmp(&self.scheduled_time))
    }
}

impl PartialOrd for Task {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

pub struct Scheduler {
    task_queue: Arc<RwLock<BinaryHeap<Task>>>,
    task_stats: Arc<RwLock<HashMap<String, TaskStats>>>,
    max_concurrent: usize,
    tx: mpsc::Sender<Task>,
    rx: mpsc::Receiver<Task>,
}

impl Scheduler {
    pub async fn new(max_concurrent: usize) -> Self {
        let (tx, rx) = mpsc::channel(max_concurrent);
        
        Self {
            task_queue: Arc::new(RwLock::new(BinaryHeap::new())),
            task_stats: Arc::new(RwLock::new(HashMap::new())),
            max_concurrent,
            tx,
            rx,
        }
    }

    pub async fn schedule_task(&self, config: TaskConfig) -> Result<(), SchedulerError> {
        let task = Task {
            id: config.id.clone(),
            priority: config.priority,
            scheduled_time: Instant::now(),
            config,
            retries: 0,
        };

        let mut queue = self.task_queue.write().await;
        if queue.len() >= self.max_concurrent {
            return Err(SchedulerError::QueueFull(
                "Maximum concurrent tasks reached".to_string()
            ));
        }

        queue.push(task);
        Ok(())
    }

    pub async fn start(&mut self) {
        let queue = Arc::clone(&self.task_queue);
        let stats = Arc::clone(&self.task_stats);
        let tx = self.tx.clone();

        tokio::spawn(async move {
            loop {
                let mut queue = queue.write().await;
                if let Some(task) = queue.pop() {
                    if let Err(e) = tx.send(task).await {
                        eprintln!("Failed to send task: {}", e);
                    }
                }
                tokio::time::sleep(Duration::from_millis(100)).await;
            }
        });

        while let Some(task) = self.rx.recv().await {
            let stats = Arc::clone(&stats);
            
            tokio::spawn(async move {
                let execution_result = Self::execute_task(&task).await;
                let mut task_stats = stats.write().await;
                
                let stats_entry = task_stats.entry(task.id.clone())
                    .or_insert(TaskStats {
                        total_executed: 0,
                        total_failed: 0,
                        average_duration: Duration::from_secs(0),
                        last_execution: None,
                    });

                stats_entry.total_executed += 1;
                stats_entry.last_execution = Some(Instant::now());

                if execution_result.is_err() {
                    stats_entry.total_failed += 1;
                }
            });
        }
    }

    async fn execute_task(task: &Task) -> Result<(), SchedulerError> {
        // Simulate task execution
        tokio::time::sleep(Duration::from_millis(100)).await;
        
        // For demonstration purposes, fail some tasks randomly
        if rand::random::<f32>() < 0.1 {
            return Err(SchedulerError::ExecutionError(
                format!("Task {} failed randomly", task.id)
            ));
        }

        Ok(())
    }

    pub async fn cancel_task(&self, task_id: &str) -> Result<(), SchedulerError> {
        let mut queue = self.task_queue.write().await;
        let before_len = queue.len();
        
        let mut new_queue: BinaryHeap<Task> = queue.drain()
            .filter(|task| task.id != task_id)
            .collect();
        
        *queue = new_queue;

        if queue.len() == before_len {
            return Err(SchedulerError::TaskNotFound(task_id.to_string()));
        }

        Ok(())
    }

    pub async fn get_task_stats(&self, task_id: &str) -> Result<TaskStats, SchedulerError> {
        let stats = self.task_stats.read().await;
        stats.get(task_id)
            .cloned()
            .ok_or_else(|| SchedulerError::TaskNotFound(task_id.to_string()))
    }

    pub async fn get_queue_size(&self) -> usize {
        self.task_queue.read().await.len()
    }

    pub async fn clear_queue(&self) {
        let mut queue = self.task_queue.write().await;
        queue.clear();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_task_scheduling() {
        let scheduler = Scheduler::new(10).await;
        
        let config = TaskConfig {
            id: "test-task".to_string(),
            priority: 1,
            max_retries: 3,
            timeout: Duration::from_secs(1),
            dependencies: vec![],
        };

        assert!(scheduler.schedule_task(config).await.is_ok());
        assert_eq!(scheduler.get_queue_size().await, 1);
    }

    #[tokio::test]
    async fn test_queue_limit() {
        let scheduler = Scheduler::new(1).await;
        
        let config1 = TaskConfig {
            id: "task1".to_string(),
            priority: 1,
            max_retries: 3,
            timeout: Duration::from_secs(1),
            dependencies: vec![],
        };

        let config2 = TaskConfig {
            id: "task2".to_string(),
            priority: 1,
            max_retries: 3,
            timeout: Duration::from_secs(1),
            dependencies: vec![],
        };

        assert!(scheduler.schedule_task(config1).await.is_ok());
        assert!(scheduler.schedule_task(config2).await.is_err());
    }

    #[tokio::test]
    async fn test_task_cancellation() {
        let scheduler = Scheduler::new(10).await;
        
        let config = TaskConfig {
            id: "test-task".to_string(),
            priority: 1,
            max_retries: 3,
            timeout: Duration::from_secs(1),
            dependencies: vec![],
        };

        scheduler.schedule_task(config).await.unwrap();
        assert!(scheduler.cancel_task("test-task").await.is_ok());
        assert_eq!(scheduler.get_queue_size().await, 0);
    }
}