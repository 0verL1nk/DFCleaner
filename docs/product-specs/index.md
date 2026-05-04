# Product Specifications Index

## Feature Specs

| Spec | Description |
|------|-------------|
| [disk-scanner](./disk-scanner.md) | File system scanning engine — recursive scan, metadata collection, concurrent scanning, AI tool exposure |
| [ai-analyzer](./ai-analyzer.md) | AI analysis engine — LLM-powered file safety analysis, batch optimization, risk classification, Agent orchestration |
| [cleanup-engine](./cleanup-engine.md) | Cleanup execution — user confirmation, trash/delete/move, partial failure handling, operation logging |
| [llm-provider](./llm-provider.md) | Multi-LLM support — eino-ext adapters, connection testing with function calling validation, API key encryption |
| [disk-visualizer](./disk-visualizer.md) | Disk visualization — Treemap (d3-hierarchy), file list, AI highlighting, drill-down with cache + mtime check |
| [app-shell](./app-shell.md) | Application shell — Wails desktop app, three views (Dashboard/Scanner/Settings), i18n, dark/light theme, auto-update |

## Source

Detailed specs live in `openspec/changes/dfcleaner-ai-disk-cleaner/specs/`. This index summarizes the key requirements for quick reference.

## User Flow

```
Launch → LLM Check → Dashboard
                        │
                        ├─► Quick Scan → Scanner View
                        │                  │
                        │                  ├─► Treemap / File List
                        │                  ├─► AI auto-analyzes → risk badges
                        │                  ├─► User selects → Cleanup → Confirm
                        │                  └─► Chat sidebar (secondary)
                        │
                        └─► Settings
                             ├─► LLM Config (provider/key/model)
                             ├─► Scan preferences
                             ├─► Privacy (content preview)
                             ├─► Theme (dark/light/system)
                             └─► Language (EN/ZH)
```
