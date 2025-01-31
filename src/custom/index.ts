import { BaseAgent, AgentOptions } from '../base';

interface CustomAgentConfig extends AgentOptions {
  capabilities: string[];
  customEndpoint?: string;
  authToken?: string;
  webhooks?: {
    onMessage?: string;
    onError?: string;
    onStateChange?: string;
  };
}

interface CustomAgentState {
  activeCapabilities: string[];
  lastResponse: any;
  metrics: {
    requestCount: number;
    successRate: number;
    averageResponseTime: number;
  };
}

export class CustomAgent extends BaseAgent {
  private config: CustomAgentConfig;
  private customState: CustomAgentState;

  constructor(config: CustomAgentConfig) {
    super(config);
    this.config = config;
    this.customState = {
      activeCapabilities: [],
      lastResponse: null,
      metrics: {
        requestCount: 0,
        successRate: 100,
        averageResponseTime: 0
      }
    };
  }

  async initialize(): Promise<void> {
    // Validate capabilities
    if (!this.config.capabilities || this.config.capabilities.length === 0) {
      throw new Error('Custom agent must have at least one capability');
    }

    // Initialize capabilities
    this.customState.activeCapabilities = [...this.config.capabilities];
    
    // Setup webhooks if provided
    if (this.config.webhooks) {
      this.setupWebhooks();
    }

    await this.setState({ isRunning: true });
    this.emit('initialized', {
      capabilities: this.customState.activeCapabilities,
      timestamp: new Date()
    });
  }

  async process(input: any): Promise<any> {
    this.updateLastActive();
    const startTime = Date.now();

    try {
      // Process the input based on active capabilities
      const result = await this.processWithCapabilities(input);
      
      // Update metrics
      this.updateMetrics(startTime, true);
      
      this.customState.lastResponse = result;
      return result;
    } catch (error) {
      // Update metrics with failure
      this.updateMetrics(startTime, false);
      
      this.emit('error', error);
      throw error;
    }
  }

  async shutdown(): Promise<void> {
    // Clean up any active capabilities
    this.customState.activeCapabilities = [];
    
    // Clear metrics
    this.customState.metrics = {
      requestCount: 0,
      successRate: 100,
      averageResponseTime: 0
    };

    await this.setState({ isRunning: false });
    this.emit('shutdown', { timestamp: new Date() });
  }

  private async processWithCapabilities(input: any): Promise<any> {
    // Process input based on active capabilities
    const results = await Promise.all(
      this.customState.activeCapabilities.map(async (capability) => {
        // Simulate processing with different capabilities
        await new Promise(resolve => setTimeout(resolve, 100));
        return {
          capability,
          result: `Processed with ${capability}`
        };
      })
    );

    return {
      timestamp: new Date(),
      results
    };
  }

  private setupWebhooks(): void {
    if (this.config.webhooks.onMessage) {
      this.on('message', async (data) => {
        // Simulate webhook call
        console.log(`Webhook triggered: onMessage`, data);
      });
    }

    if (this.config.webhooks.onError) {
      this.on('error', async (error) => {
        console.log(`Webhook triggered: onError`, error);
      });
    }
  }

  private updateMetrics(startTime: number, success: boolean): void {
    const responseTime = Date.now() - startTime;
    const { metrics } = this.customState;
    
    metrics.requestCount++;
    metrics.averageResponseTime = (
      (metrics.averageResponseTime * (metrics.requestCount - 1) + responseTime) / 
      metrics.requestCount
    );

    if (!success) {
      metrics.successRate = (
        (metrics.successRate * (metrics.requestCount - 1) + 0) / 
        metrics.requestCount
      );
    }
  }

  // Public methods for managing capabilities
  public addCapability(capability: string): void {
    if (!this.customState.activeCapabilities.includes(capability)) {
      this.customState.activeCapabilities.push(capability);
      this.emit('capabilityAdded', { capability });
    }
  }

  public removeCapability(capability: string): void {
    this.customState.activeCapabilities = 
      this.customState.activeCapabilities.filter(cap => cap !== capability);
    this.emit('capabilityRemoved', { capability });
  }

  public getMetrics(): CustomAgentState['metrics'] {
    return { ...this.customState.metrics };
  }
}