# Design Decisions Index

All architectural decisions for DFCleaner, organized by category.

## Architecture

| Decision | Choice | Why |
|----------|--------|-----|
| Desktop framework | Wails v2 | Go-native, no sidecar/HTTP, small binary |
| AI Agent framework | CloudWeGo Eino | Go-native ReAct Agent, Tool calling, streaming |
| Database | SQLite + GORM | Embedded, zero-config, GORM AutoMigrate |
| Frontend | React + TypeScript + Tailwind + shadcn/ui | User-specified, mature ecosystem |
| Visualization | d3-hierarchy + custom SVG | Full control over Treemap interaction/styling |
| Auto-update | GitHub Releases API | No custom update server needed |

## AI Agent

| Decision | Choice | Why |
|----------|--------|-----|
| Agent model | Eino ReAct | Agent decides scan strategy via tool calls |
| Tool scope | Read-only only | Architecture-level safety: Agent cannot modify files |
| Step/token limit | No limit | User pays for own API, tool returns are summarized |
| LLM adapters | eino-ext native (openai, claude, ollama, etc.) | 10+ adapters covering major providers |
| System Prompt | Hardcoded base + platform info + user custom | Separation of concerns |
| Analysis output | Structured: risk_level + reason + category + confidence | Machine-readable for UI rendering |

## Product

| Decision | Choice | Why |
|----------|--------|-----|
| Core identity | Application, not chat | Visual-first, AI embedded, chat secondary |
| Views | 3: Dashboard, Scanner, Settings | Simple > complex |
| LLM unavailable | Block app, no degradation | DFCleaner without AI has no value |
| Default deletion | System trash | Recoverable by default |
| i18n | Hard requirement from day one | Retrofitting i18n is expensive |
| Theme | Dark/light, follow system | shadcn/ui supports this for free |

## Data

| Decision | Choice | Why |
|----------|--------|-----|
| Type contract | wails:generate auto-generates TS types | Frontend/backend always in sync |
| Scan data flow | Chunked push (500 items) + virtual list | Handle 10k+ files without lag |
| Drill-down | Cache + mtime check | Fast when unchanged, fresh when modified |
| Error handling | Per-item report, no rollback | Simple, predictable |
| SQLite schema | GORM AutoMigrate + SQL doc files | Runtime migration + human-readable docs |

## Detailed Documents

- [core-beliefs.md](./core-beliefs.md) — Foundational design beliefs
