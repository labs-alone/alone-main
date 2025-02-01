import { EventEmitter } from 'events';

/**
 * Agent Types
 */
export interface AgentConfig {
    id: string;
    name: string;
    model: string;
    temperature?: number;
    maxTokens?: number;
    stopSequences?: string[];
    contextWindow?: number;
    memory?: MemoryConfig;
    capabilities?: string[];
}

export interface AgentState {
    isActive: boolean;
    lastActive: Date;
    currentTask?: string;
    memoryUsage: number;
    conversationId?: string;
    metadata: Record<string, any>;
}

export interface AgentResponse {
    content: string;
    tokens: number;
    finished: boolean;
    metadata: ResponseMetadata;
}

export interface ResponseMetadata {
    timestamp: Date;
    model: string;
    promptTokens: number;
    completionTokens: number;
    totalTokens: number;
    latency: number;
    conversationId?: string;
}

/**
 * Memory Types
 */
export interface MemoryConfig {
    type: 'short_term' | 'long_term' | 'episodic' | 'semantic';
    capacity: number;
    ttl?: number;
    persistenceEnabled?: boolean;
    compressionEnabled?: boolean;
}

export interface MemoryEntry {
    id: string;
    content: string;
    type: string;
    timestamp: Date;
    metadata: Record<string, any>;
    associations?: string[];
    importance?: number;
}

export interface MemorySearchResult {
    entry: MemoryEntry;
    relevance: number;
    distance: number;
}

/**
 * Runtime Types
 */
export interface RuntimeConfig {
    maxAgents: number;
    maxMemoryUsage: number;
    maxConcurrentTasks: number;
    timeoutMs: number;
    debugMode: boolean;
}

export interface RuntimeMetrics {
    activeAgents: number;
    memoryUsage: number;
    taskCount: number;
    uptime: number;
    errorRate: number;
}

export interface TaskConfig {
    id: string;
    type: string;
    priority: number;
    timeout?: number;
    retries?: number;
    dependencies?: string[];
}

/**
 * Event Types
 */
export enum AgentEvent {
    CREATED = 'agent:created',
    STARTED = 'agent:started',
    STOPPED = 'agent:stopped',
    ERROR = 'agent:error',
    TASK_COMPLETE = 'agent:task_complete',
    MEMORY_UPDATED = 'agent:memory_updated'
}

export enum RuntimeEvent {
    STARTED = 'runtime:started',
    STOPPED = 'runtime:stopped',
    ERROR = 'runtime:error',
    AGENT_ADDED = 'runtime:agent_added',
    AGENT_REMOVED = 'runtime:agent_removed',
    MEMORY_FULL = 'runtime:memory_full'
}

/**
 * Error Types
 */
export class AgentError extends Error {
    constructor(
        public code: string,
        message: string,
        public details?: any
    ) {
        super(message);
        this.name = 'AgentError';
    }
}

export class RuntimeError extends Error {
    constructor(
        public code: string,
        message: string,
        public details?: any
    ) {
        super(message);
        this.name = 'RuntimeError';
    }
}

/**
 * Interface Definitions
 */
export interface Agent extends EventEmitter {
    readonly id: string;
    readonly config: AgentConfig;
    state: AgentState;

    initialize(): Promise<void>;
    start(): Promise<void>;
    stop(): Promise<void>;
    process(input: string): Promise<AgentResponse>;
    learn(data: any): Promise<void>;
}

export interface Memory {
    add(entry: MemoryEntry): Promise<void>;
    get(id: string): Promise<MemoryEntry | null>;
    search(query: string, limit?: number): Promise<MemorySearchResult[]>;
    forget(id: string): Promise<void>;
    clear(): Promise<void>;
}

export interface Runtime {
    addAgent(agent: Agent): Promise<void>;
    removeAgent(agentId: string): Promise<void>;
    getAgent(agentId: string): Agent | undefined;
    getMetrics(): RuntimeMetrics;
    shutdown(): Promise<void>;
}

/**
 * Utility Types
 */
export interface Logger {
    debug(message: string, ...args: any[]): void;
    info(message: string, ...args: any[]): void;
    warn(message: string, ...args: any[]): void;
    error(message: string, ...args: any[]): void;
}

export interface Metrics {
    increment(name: string, value?: number): void;
    gauge(name: string, value: number): void;
    timing(name: string, value: number): void;
    flush(): Promise<void>;
}

/**
 * Model Types
 */
export interface ModelConfig {
    provider: 'openai' | 'anthropic' | 'custom';
    model: string;
    apiKey: string;
    apiEndpoint?: string;
    options?: Record<string, any>;
}

export interface ModelResponse {
    content: string;
    usage: {
        promptTokens: number;
        completionTokens: number;
        totalTokens: number;
    };
    metadata: Record<string, any>;
}

/**
 * Security Types
 */
export interface SecurityConfig {
    apiKeys: string[];
    allowedOrigins: string[];
    rateLimits: {
        requests: number;
        window: number;
    };
    encryption?: {
        enabled: boolean;
        algorithm: string;
        keySize: number;
    };
}

/**
 * Validation Types
 */
export interface ValidationResult {
    valid: boolean;
    errors: string[];
    warnings?: string[];
}

export type Validator<T> = (data: T) => Promise<ValidationResult>;

/**
 * Export all types
 */
export * from '../plugins/types';