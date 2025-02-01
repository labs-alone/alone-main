import { EventEmitter } from 'events';
import { 
    Plugin, 
    PluginMetadata, 
    PluginConfig, 
    PluginState,
    PluginEvent,
    PluginError,
    PluginHooks,
    PluginValidationResult
} from './types';
import { Logger } from '../utils/logger';

interface PluginManagerConfig {
    maxPlugins?: number;
    autoStart?: boolean;
    validateOnLoad?: boolean;
    enableHotReload?: boolean;
}

class PluginManager extends EventEmitter {
    private plugins: Map<string, Plugin>;
    private pluginStates: Map<string, PluginState>;
    private hooks: Map<string, Set<PluginHooks>>;
    private config: PluginManagerConfig;
    private logger: Logger;

    constructor(config: PluginManagerConfig = {}) {
        super();
        this.plugins = new Map();
        this.pluginStates = new Map();
        this.hooks = new Map();
        this.config = {
            maxPlugins: 100,
            autoStart: true,
            validateOnLoad: true,
            enableHotReload: false,
            ...config
        };
        this.logger = new Logger('PluginManager');
    }

    async loadPlugin(
        pluginPath: string, 
        config?: PluginConfig
    ): Promise<PluginMetadata> {
        try {
            // Check plugin limit
            if (this.plugins.size >= this.config.maxPlugins!) {
                throw new PluginError('MAX_PLUGINS_EXCEEDED', 
                    `Maximum number of plugins (${this.config.maxPlugins}) exceeded`);
            }

            // Load plugin module
            const pluginModule = await import(pluginPath);
            const plugin: Plugin = new pluginModule.default(config);

            // Validate plugin
            if (this.config.validateOnLoad) {
                const validationResult = await this.validatePlugin(plugin);
                if (!validationResult.valid) {
                    throw new PluginError('VALIDATION_FAILED', 
                        `Plugin validation failed: ${validationResult.errors.join(', ')}`);
                }
            }

            // Register plugin
            const metadata = await plugin.getMetadata();
            this.plugins.set(metadata.id, plugin);
            this.pluginStates.set(metadata.id, PluginState.LOADED);

            // Register hooks
            this.registerPluginHooks(metadata.id, plugin.hooks);

            // Auto-start if configured
            if (this.config.autoStart) {
                await this.startPlugin(metadata.id);
            }

            this.emit(PluginEvent.LOADED, metadata);
            return metadata;

        } catch (error) {
            this.logger.error(`Failed to load plugin: ${error.message}`);
            throw new PluginError('LOAD_FAILED', 
                `Failed to load plugin: ${error.message}`);
        }
    }

    async startPlugin(pluginId: string): Promise<void> {
        const plugin = this.plugins.get(pluginId);
        if (!plugin) {
            throw new PluginError('PLUGIN_NOT_FOUND', 
                `Plugin ${pluginId} not found`);
        }

        try {
            await plugin.onStart();
            this.pluginStates.set(pluginId, PluginState.RUNNING);
            this.emit(PluginEvent.STARTED, pluginId);

        } catch (error) {
            this.logger.error(`Failed to start plugin ${pluginId}: ${error.message}`);
            this.pluginStates.set(pluginId, PluginState.ERROR);
            throw new PluginError('START_FAILED', 
                `Failed to start plugin ${pluginId}: ${error.message}`);
        }
    }

    async stopPlugin(pluginId: string): Promise<void> {
        const plugin = this.plugins.get(pluginId);
        if (!plugin) {
            throw new PluginError('PLUGIN_NOT_FOUND', 
                `Plugin ${pluginId} not found`);
        }

        try {
            await plugin.onStop();
            this.pluginStates.set(pluginId, PluginState.STOPPED);
            this.emit(PluginEvent.STOPPED, pluginId);

        } catch (error) {
            this.logger.error(`Failed to stop plugin ${pluginId}: ${error.message}`);
            throw new PluginError('STOP_FAILED', 
                `Failed to stop plugin ${pluginId}: ${error.message}`);
        }
    }

    async reloadPlugin(pluginId: string): Promise<PluginMetadata> {
        if (!this.config.enableHotReload) {
            throw new PluginError('HOT_RELOAD_DISABLED', 
                'Hot reload is disabled in configuration');
        }

        try {
            await this.stopPlugin(pluginId);
            const plugin = this.plugins.get(pluginId);
            if (!plugin) {
                throw new PluginError('PLUGIN_NOT_FOUND', 
                    `Plugin ${pluginId} not found`);
            }

            const metadata = await plugin.getMetadata();
            await this.loadPlugin(metadata.path, plugin.config);
            return metadata;

        } catch (error) {
            this.logger.error(`Failed to reload plugin ${pluginId}: ${error.message}`);
            throw new PluginError('RELOAD_FAILED', 
                `Failed to reload plugin ${pluginId}: ${error.message}`);
        }
    }

    private async validatePlugin(plugin: Plugin): Promise<PluginValidationResult> {
        const errors: string[] = [];

        // Check required methods
        const requiredMethods = ['onStart', 'onStop', 'getMetadata'];
        for (const method of requiredMethods) {
            if (typeof plugin[method] !== 'function') {
                errors.push(`Missing required method: ${method}`);
            }
        }

        // Validate metadata
        const metadata = await plugin.getMetadata();
        if (!metadata.id || !metadata.name || !metadata.version) {
            errors.push('Invalid plugin metadata');
        }

        // Validate hooks
        if (plugin.hooks) {
            for (const [hookName, hookFn] of Object.entries(plugin.hooks)) {
                if (typeof hookFn !== 'function') {
                    errors.push(`Invalid hook implementation: ${hookName}`);
                }
            }
        }

        return {
            valid: errors.length === 0,
            errors
        };
    }

    private registerPluginHooks(pluginId: string, hooks: PluginHooks): void {
        if (!hooks) return;

        for (const [hookName, hookFn] of Object.entries(hooks)) {
            if (!this.hooks.has(hookName)) {
                this.hooks.set(hookName, new Set());
            }
            this.hooks.get(hookName)!.add(hookFn);
        }
    }

    async executeHook(hookName: string, ...args: any[]): Promise<any[]> {
        const hookFns = this.hooks.get(hookName);
        if (!hookFns) return [];

        const results = [];
        for (const hookFn of hookFns) {
            try {
                const result = await hookFn(...args);
                results.push(result);
            } catch (error) {
                this.logger.error(`Hook execution failed: ${error.message}`);
            }
        }
        return results;
    }

    getPluginState(pluginId: string): PluginState | undefined {
        return this.pluginStates.get(pluginId);
    }

    getLoadedPlugins(): PluginMetadata[] {
        return Array.from(this.plugins.values())
            .map(plugin => plugin.getMetadata());
    }

    async cleanup(): Promise<void> {
        const pluginIds = Array.from(this.plugins.keys());
        for (const pluginId of pluginIds) {
            try {
                await this.stopPlugin(pluginId);
            } catch (error) {
                this.logger.error(`Failed to stop plugin ${pluginId} during cleanup: ${error.message}`);
            }
        }

        this.plugins.clear();
        this.pluginStates.clear();
        this.hooks.clear();
        this.emit(PluginEvent.CLEANUP_COMPLETE);
    }
}

export { 
    PluginManager,
    PluginManagerConfig,
    PluginError
};