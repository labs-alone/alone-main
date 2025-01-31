# VAE - An Advanced AI Agent Framework

<div align="center">
  <img src="https://media.discordapp.net/attachments/1199317611058577408/1334752994079805503/Add_a_heading.png?ex=679dad18&is=679c5b98&hm=c4213df2760135471de98003d0f1c74b4b1ec85b853c8165b407d623e3750894&=&format=webp&quality=lossless&width=1439&height=479" alt="Alone Labs Banner" width="100%" />
</div>

## Table of Contents
- [Overview](#Overview)
- [Core Features](#core-features)
- [Extension Points](#extension-points)
- [Quick Start](#quick-start)
- [Using VAE as a Module](#using-vae-as-a-module)

## Overview
VAE is a highly modular AI agent engine built with TypeScript, Rust and Go that emphasizes pluggable architecture and platform independence. It provides a flexible foundation for building AI systems through:

- Plugin-based architecture with hot-swappable components
- Multi-provider LLM support (GPT-4, Claude, custom providers)
- Cross-platform agent management
- Extensible manager system for custom behaviors
- Vector-based semantic storage with pgvector

## Core Features

### Plugin Architecture
- **Manager System**: Extend functionality through custom managers
  - Memory Manager: Handles agent memory and context
  - Personality Manager: Controls agent behavior and responses
  - Custom Managers: Add your own specialized behaviors

### State Management
- **Shared State System**: Centralized state management across components
  - Manager-specific data storage
  - Custom data injection
  - Cross-manager communication

### LLM Integration
- **Provider Abstraction**: Support for multiple LLM providers
  - Built-in GPT-4 and Claude support
  - Extensible provider interface for custom LLMs
  - Configurable model selection per operation
  - Automatic fallback and retry handling

### Platform Support
- **Platform Agnostic Core**: 
  - Abstract agent engine independent of platforms
  - Built-in support for CLI and API interfaces
  - Extensible platform manager interface
  - Example implementations for new platform integration

### Storage Layer
- **Flexible Data Storage**:
  - PostgreSQL with pgvector for semantic search
  - GORM-based data models
  - Customizable memory storage
  - Vector embedding support

### Toolkit/Function System
- **Pluggable Tool/Function Integration**:
  - Support for custom tool implementations
  - Built-in toolkit management
  - Function calling capabilities
  - Automatic tool response handling
  - State-aware tool execution

## Extension Points
1. **LLM Providers**: Add new AI providers by implementing the LLM interface
```go
type Provider interface {
    GenerateCompletion(context.Context, CompletionRequest) (string, error)
    GenerateJSON(context.Context, JSONRequest, interface{}) error
    EmbedText(context.Context, string) ([]float32, error)
}
```

2. **Managers**: Create new behaviors by implementing the Manager interface
```go
type Manager interface {
    GetID() ManagerID
    GetDependencies() []ManagerID
    Process(state *state.State) error
    PostProcess(state *state.State) error
    Context(state *state.State) ([]state.StateData, error)
    Store(fragment *db.Fragment) error
    StartBackgroundProcesses()
    StopBackgroundProcesses()
    RegisterEventHandler(callback EventCallbackFunc)
    triggerEvent(eventData EventData)
}
```

## Quick Start
1. Clone the repository
```bash 
git clone https://github.com/alonelabs/vae
```   
2. Copy `.env.example` to `.env` and configure your environment variables
3. Install dependencies:
```bash
npm install
cargo build
go mod download
```
4. Run the development environment:
```bash
npm run dev
```

## Environment Variables
```env
DB_URL=postgresql://user:password@localhost:5432/vae
OPENAI_API_KEY=your_openai_api_key
ANTHROPIC_API_KEY=your_anthropic_api_key

Platform-specific credentials as needed
```

## Architecture
The project follows a clean, multi-language architecture:

- `src/agents`: TypeScript agent implementations
- `src/runtime`: Rust-based runtime engine
- `src/plugins`: Plugin system
- `pkg/network`: Go networking layer
- `examples/`: Reference implementations

## Using VAE as a Module

1. Add VAE to your project:
```bash
npm install @alonelabs/vae
```

2. Import VAE in your code:
```typescript
import {
  Engine,
  LLMClient,
  BaseAgent,
  MemoryManager,
  PersonalityManager
} from '@alonelabs/vae';
```

3. Basic usage example:
```typescript
// Initialize LLM client
const llmClient = new LLMClient({
  provider: 'gpt4',
  apiKey: process.env.OPENAI_API_KEY,
  modelConfig: {
    default: 'gpt-4-turbo'
  }
});

// Create engine instance
const engine = new Engine({
  llm: llmClient,
  db: database,
  logger: logger
});

// Process input
const state = await engine.newState({
  actorId,
  sessionId,
  input: "Your input text here"
});

const response = await engine.process(state);
```

4. Available packages:
- `@alonelabs/vae`: Core framework
- `@alonelabs/vae-llm`: LLM provider interfaces
- `@alonelabs/vae-runtime`: Rust runtime
- `@alonelabs/vae-network`: Go networking
- `@alonelabs/vae-plugins`: Plugin system

For detailed examples, see the `examples/` directory in the repository.

## Contact

- Website: [alonelabs.net](https://alonelabs.net)
- Email: contact@alonelabs.net
- Twitter: [@alone_labs](https://x.com/alone_labs)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
