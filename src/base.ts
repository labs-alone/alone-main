import { EventEmitter } from 'events';

export interface AgentOptions {
  name: string;
  version: string;
  maxMemory?: number;
  timeout?: number;
  metadata?: Record<string, any>;
}

export interface AgentState {
  isRunning: boolean;
  lastActive: Date;
  memory: Map<string, any>;
}

export abstract class BaseAgent extends EventEmitter {
  protected readonly name: string;
  protected readonly version: string;
  protected state: AgentState;
  private readonly options: AgentOptions;

  constructor(options: AgentOptions) {
    super();
    this.name = options.name;
    this.version = options.version;
    this.options = options;
    this.state = {
      isRunning: false,
      lastActive: new Date(),
      memory: new Map()
    };
  }

  abstract async initialize(): Promise<void>;
  abstract async process(input: any): Promise<any>;
  abstract async shutdown(): Promise<void>;

  protected async setState(newState: Partial<AgentState>): Promise<void> {
    this.state = { ...this.state, ...newState };
    this.emit('stateChange', this.state);
  }

  public async start(): Promise<void> {
    if (this.state.isRunning) {
      throw new Error('Agent is already running');
    }

    await this.initialize();
    await this.setState({ isRunning: true });
    this.emit('started', { timestamp: new Date() });
  }

  public async stop(): Promise<void> {
    if (!this.state.isRunning) {
      throw new Error('Agent is not running');
    }

    await this.shutdown();
    await this.setState({ isRunning: false });
    this.emit('stopped', { timestamp: new Date() });
  }

  public getStatus(): { name: string; state: AgentState } {
    return {
      name: this.name,
      state: this.state
    };
  }

  protected updateLastActive(): void {
    this.state.lastActive = new Date();
  }
}