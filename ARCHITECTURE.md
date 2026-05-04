# DFCleaner Architecture

## System Overview

```
┌─────────────────────────────────────────────────────────┐
│                    Wails Desktop App                     │
│                                                         │
│  ┌──────────────┐  Wails Bindings  ┌────────────────┐  │
│  │   Frontend   │ ◄──────────────► │  Go Backend    │  │
│  │  (React/TS)  │  wails:generate  │  (internal/)   │  │
│  │              │  Wails Events    │                │  │
│  └──────────────┘                  └───────┬────────┘  │
│                                            │            │
│                              ┌─────────────┼──────┐    │
│                              │             │      │    │
│                         ┌────▼───┐   ┌─────▼──┐  │    │
│                         │Scanner │   │Analyzer│  │    │
│                         │+Tools  │   │(Eino   │  │    │
│                         └────────┘   │ReAct)  │  │    │
│                              ▲       └────┬───┘  │    │
│                              │            │      │    │
│                         ┌────┴─────┐  ┌───▼────┐│    │
│                         │Cleaner   │  │  LLM   ││    │
│                         │(Trash/   │  │(eino-  ││    │
│                         │Delete)   │  │ ext)   ││    │
│                         └──────────┘  └────────┘│    │
│                                           │      │    │
│                                      ┌────▼───┐  │    │
│                                      │ Store  │  │    │
│                                      │(SQLite)│  │    │
│                                      └────────┘  │    │
│                                           ▲      │    │
│                                      ┌────┴───┐  │    │
│                                      │Platform│  │    │
│                                      │(OS     │  │    │
│                                      │specific│  │    │
│                                      └────────┘  │    │
└─────────────────────────────────────────────────────────┘
```

## Data Flow: Typical User Session

```
1. App Start
   └─► Check LLM config in SQLite
       └─► Test connection (incl. function calling validation)
           └─► OK → Dashboard | Fail → Setup wizard

2. Scan Directory
   └─► Frontend calls Scan(path, opts)
       └─► Go scanner walks directory tree (goroutine pool)
           └─► Every 500 files: push chunk via Wails Events
               └─► Frontend appends to Zustand store
                   └─► Virtual list + Treemap render incrementally

3. AI Analysis (automatic after scan)
   └─► Scanner results → batch to LLM
       └─► LLM returns structured analysis per file
           └─► Push AnalysisResult via Wails Events
               └─► Frontend updates file list with risk level badges

4. User Cleanup
   └─► User selects files → clicks "Clean"
       └─► Frontend calls Cleanup(items)
           └─► Cleaner executes per-file (trash/delete/move)
               └─► Returns per-item result
                   └─► Log to SQLite cleanup_logs

5. Chat (secondary entry)
   └─► User types message → Chat(msg)
       └─► Eino ReAct Agent processes
           ├─► Calls Tools (scan_directory, get_file_info, etc.)
           ├─► Streams thoughts via Wails Events
           └─► Analysis results reflected in main UI file list
```

## Three Frontend Views

```
```
┌─────────────────────────────────────────┐
│ Custom Title Bar (Frameless)            │
├────────┬────────────────────────────────┤
│Sidebar │                                │
├─────────────────────────────────────────┤
│                                         │
│  Dashboard:                             │
│  - Space overview (total/used/free)     │
│  - Quick scan entry                     │
│  - Recent cleanup history               │
│  - LLM connection status                │
│                                         │
│  Scanner (main view, 90% of time):      │
│  - Treemap (d3-hierarchy + SVG)         │
│  - File list (virtual, sortable)        │
│  - AI risk level column (green/yellow/  │
│    red) + reason tooltip                │
│  - Cleanup action bar                   │
│  - Chat sidebar (collapsible)           │
│                                         │
│  Settings:                              │
│  - LLM config (provider/endpoint/key/   │
│    model)                               │
│  - Scan preferences                     │
│  - Privacy (content preview toggle)     │
│  - Theme (dark/light/system)            │
│  - Language (i18n)                      │
│                                         │
└─────────────────────────────────────────┘
```

## AI Agent Architecture (Eino)

```
┌──────────────────────────────────┐
│       Eino ReAct Agent           │
│                                  │
│  System Prompt:                  │
│  ├─ Role: disk cleanup assistant │
│  ├─ Tool descriptions            │
│  ├─ Risk level definitions       │
│  ├─ Output format (structured)   │
│  ├─ Platform info (runtime)      │
│  └─ User custom instructions     │
│                                  │
│  Tools (read-only):              │
│  ├─ scan_directory(path) → overview│
│  ├─ get_file_info(path) → metadata│
│  ├─ get_large_files(threshold)   │
│  ├─ get_old_files(age)           │
│  └─ find_duplicates(path)        │
│                                  │
│  Output per file:                │
│  {                               │
│    risk_level: safe|caution|     │
│                 dangerous,        │
│    reason: string,               │
│    category: cache|temp|log|     │
│             config|document|     │
│             media,               │
│    confidence: 0-1               │
│  }                               │
└──────────────────────────────────┘
```

## Database Schema (SQLite + GORM)

Core tables:
- `llm_configs` — provider, endpoint, api_key (encrypted), model_name, is_active
- `scan_history` — path, scanned_at, file_count, total_size
- `cleanup_logs` — scan_id, file_path, operation, result, created_at
- `user_settings` — key, value (KV store for theme, language, preferences)

GORM AutoMigrate handles runtime schema evolution. SQL files maintained in `internal/store/schema/` as documentation.

## Cross-Platform Strategy

Platform-specific code isolated in `internal/platform/`:
- **Trash paths**: Linux (`~/.local/share/Trash/files`), macOS (`~/.Trash`), Windows (system API)
- **File system quirks**: macOS `.DS_Store`, Windows `Thumbs.db`, Linux `.directory`
- **System directories**: per-OS cache/config/temp directory detection

Build tags (`//go:build linux`, `//go:build darwin`, `//go:build windows`) for platform-specific implementations.
