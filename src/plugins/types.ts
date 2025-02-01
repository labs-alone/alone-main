import { EventEmitter } from 'events';

/**
 * Plugin metadata interface
 */
export interface PluginMetadata {
    id: string;
    name: string;
    version: string;
    description?: string;
    author?: string;
    path: string;
    dependencies?: string[];
    compatibilityVersion?: string;
    tags?: string[];
    license?: string;
    repository?: string;
    homepage?: string;
}

/**
 * Plugin configuration interface
 */
export interface PluginConfig {
    enabled?: boolean;
    priority?: number;
    timeout?: number;
    retryAttempts?: number;
    settings?: Record<string, any>;
    permissions?: string[];
    environment?: 'development' | 'production' | 'testing';
}

/**
 * Plugin state enum
 */
export enum PluginState {
    UNLOADED = 'unloaded',
    LOADED = 'loaded',
    RUNNING = 'running',
    STOPPED = 'stopped',
    ERROR = 'error',
    DISABLED = 'disabled'
}

/**
 * Plugin event enum
 */
export enum PluginEvent {
    LOADED = 'plugin:loaded',
    STARTED = 'plugin:started',
    STOPPED = 'plugin:stopped',
    ERROR = 'plugin:error',
    RELOADED = 'plugin:reloaded',
    CLEANUP_COMPLETE = 'plugin:cleanup_complete',
    STATE_CHANGED = 'plugin:state_changed',
    HOOK_EXECUTED = 'plugin:hook_executed'
}

/**
 * Plugin validation result interface
 */
export interface PluginValidationResult {
    valid: boolean;
    errors: string[];
    warnings?: string[];
}

/**
 * Plugin hook types
 */
export interface PluginHooks {
    onInit?: () => Promise<void>;
    onDestroy?: () => Promise<void>;
    beforeStart?: () => Promise<void>;
    afterStart?: () => Promise<void>;
    beforeStop?: () => Promise<void>;
    afterStop?: () => Promise<void>;
    onError?: (error: Error) => Promise<void>;
    onConfigChange?: (newConfig: PluginConfig) => Promise<void>;
    [key: string]: ((...args: any[]) => Promise<any>) | undefined;
}

/**
 * Plugin interface
 */
export interface Plugin extends EventEmitter {
    readonly id: string;
    config: PluginConfig;
    hooks?: PluginHooks;
    state: PluginState;

    onStart(): Promise<void>;
    onStop(): Promise<void>;
    getMetadata(): Promise<PluginMetadata>;
    validateConfig?(config: PluginConfig): Promise<boolean>;
    handleError?(error: Error): Promise<void>;
}

/**
 * Plugin error class
 */
export class PluginError extends Error {
    constructor(
        public code: string,
        message: string,
        public details?: any
    ) {
        super(message);
        this.name = 'PluginError';
    }
}

/**
 * Plugin dependency interface
 */
export interface PluginDependency {
    id: string;
    version: string;
    optional?: boolean;
}

/**
 * Plugin resource usage interface
 */
export interface PluginResourceUsage {
    memoryUsage: number;
    cpuUsage: number;
    timestamp: Date;
}

/**
 * Plugin communication interface
 */
export interface PluginMessage {
    type: string;
    payload: any;
    source: string;
    target?: string;
    timestamp: Date;
}

/**
 * Plugin security interface
 */
export interface PluginSecurity {
    permissions: string[];
    signature?: string;
    certificateHash?: string;
    trustedSources?: string[];
}

/**
 * Plugin update info interface
 */
export interface PluginUpdateInfo {
    currentVersion: string;
    newVersion: string;
    releaseNotes?: string;
    updateUrl?: string;
    mandatory?: boolean;
}

/**
 * Plugin storage interface
 */
export interface PluginStorage {
    get(key: string): Promise<any>;
    set(key: string, value: any): Promise<void>;
    delete(key: string): Promise<void>;
    clear(): Promise<void>;
}

/**
 * Plugin API interface
 */
export interface PluginAPI {
    register(plugin: Plugin): Promise<void>;
    unregister(pluginId: string): Promise<void>;
    getPlugin(pluginId: string): Plugin | undefined;
    listPlugins(): Plugin[];
    executeHook(hookName: string, ...args: any[]): Promise<any[]>;
}

/**
 * Plugin logger interface
 */
export interface PluginLogger {
    debug(message: string, ...args: any[]): void;
    info(message: string, ...args: any[]): void;
    warn(message: string, ...args: any[]): void;
    error(message: string, ...args: any[]): void;
    setLevel(level: string): void;
}

/**
 * Plugin metrics interface
 */
export interface PluginMetrics {
    recordMetric(name: string, value: number, tags?: Record<string, string>): void;
    getMetric(name: string): Promise<number>;
    listMetrics(): Promise<string[]>;
}

/**
 * Plugin health check interface
 */
export interface PluginHealthCheck {
    name: string;
    status: 'healthy' | 'unhealthy' | 'warning';
    details?: Record<string, any>;
    timestamp: Date;
}

/**
 * Plugin capability interface
 */
export interface PluginCapabilities {
    features: string[];
    maxInstances?: number;
    supportedPlatforms?: string[];
    requiredResources?: string[];
}

/**
 * Plugin configuration validator
 */
export type PluginConfigValidator = (config: PluginConfig) => Promise<PluginValidationResult>;

/**
 * Plugin factory interface
 */
export interface PluginFactory {
    create(config: PluginConfig): Promise<Plugin>;
    validate(config: PluginConfig): Promise<PluginValidationResult>;
}