# DFCleaner - Agent & Contributor Guide

## Project Overview

DFCleaner is an AI-powered cross-platform disk cleanup desktop application. Unlike existing tools (BleachBit, CCleaner, ncdu) that clean by predefined rules, DFCleaner uses an AI Agent to understand file semantics and advise users on safe deletion with natural language explanations.

**Core identity**: DFCleaner is an **application**, not a chatbot. The main interface is disk space visualization (Treemap + file list). AI analysis runs in the background and marks files with risk levels directly in the UI. Natural language chat is a secondary entry point.

## Tech Stack

### Backend (Go)
- **Framework**: Wails v2 — Go functions bound directly to frontend via `wails:generate`, no HTTP layer
- **AI Agent**: CloudWeGo Eino — ReAct Agent with Tool/Function Calling
- **LLM Adapters**: eino-ext (`openai`, `claude`, `ollama`, `gemini`, `deepseek`, `qwen`, etc.)
- **Database**: SQLite + GORM (AutoMigrate)
- **Auto-update**: GitHub Releases API

### Frontend (React + TypeScript)
- **Build**: Vite
- **UI**: React + TypeScript + Tailwind CSS + shadcn/ui + Radix UI
- **State**: Zustand (client) + TanStack Query (server)
- **Routing**: TanStack Router — three views: Dashboard, Scanner, Settings
- **Forms**: React Hook Form + Zod
- **i18n**: react-i18next (hard requirement, all text externalized from day one)
- **Visualization**: d3-hierarchy + custom SVG (Treemap)
- **Icons**: Lucide React
- **Virtual lists**: @tanstack/virtual

## Go Package Structure

```
internal/
├── scanner/       # File system scanner + Eino Tools
├── analyzer/      # AI Agent (Eino ReAct) + System Prompt
├── cleaner/       # Cleanup engine (delete/move/trash)
├── llm/           # LLM adapter layer (eino-ext config management)
├── store/         # SQLite storage (config, history) + GORM models
├── platform/      # Platform-specific logic (trash paths, etc.)
└── app.go         # Wails App struct, binds all services
```

## Critical Rules

### Security
- **AI Agent is strictly read-only**. Agent Tools can only scan/analyze files. All write operations (delete/move) are triggered by users through the UI. This is an architectural constraint, not a policy.
- Deleted files go to system trash by default, not permanent delete.
- API keys are encrypted in SQLite, never stored in plaintext.

### Type Contract
- Go structs generate TypeScript types via `wails:generate`. **Never hand-write TS type definitions** for data that comes from Go. This ensures frontend and backend types are always in sync.

### i18n
- **All UI text must be externalized** via react-i18next keys (e.g., `t('settings.llm.title')`). No hardcoded user-facing strings anywhere.
- Default language packs: English and Chinese.
- AI System Prompt language follows user locale setting.

### No Degradation
- If the LLM is unavailable, the app is blocked at the configuration/setup screen. DFCleaner without AI has no value.

### Iterate, Don't Over-Design
- AI analysis strategy (trigger timing, batch sizes, depth levels) should be validated with real end-to-end testing, not over-specified upfront. Get the basic flow working first, then optimize.

## Development Workflow

1. Backend changes: modify Go code in `internal/`, run `wails generate` to update bindings
2. Frontend changes: modify React components in `frontend/src/`
3. Run dev: `wails dev`
4. Build: `wails build`

## Key Wails Binding Methods

| Method | Purpose |
|--------|---------|
| `Scan(path, opts)` | Scan directory, returns results in chunks via Wails Events |
| `Chat(message)` | Send message to AI Agent, streams response via Wails Events |
| `Cleanup(items)` | Execute cleanup operations, returns per-item results |
| `TestLLMConnection(config)` | Test LLM config including function calling support |
| `GetSettings()` / `SaveSettings()` | Read/write app settings |
| `GetLLMConfigs()` / `SaveLLMConfig()` | Manage LLM provider configurations |
| `CheckUpdate()` | Check GitHub Releases for new version |

## LLM Provider Types

Supported eino-ext adapters: `openai`, `claude`, `gemini`, `deepseek`, `ollama`, `qwen`, `ark`, `openrouter`, `qianfan`. Users select provider type, the app uses the corresponding adapter.
