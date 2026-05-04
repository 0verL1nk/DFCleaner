# Frontend Guide

## Tech Stack

| Category | Choice | Notes |
|----------|--------|-------|
| Build | Vite | Fast HMR, tree-shaking |
| Framework | React 18+ | Functional components only |
| Language | TypeScript | Strict mode |
| Styling | Tailwind CSS | Utility-first, dark mode via `class` strategy |
| Components | shadcn/ui + Radix UI | Copy-paste components, not a dependency |
| State (server) | TanStack Query | Caching, background refetch for Go bindings |
| State (client) | Zustand | Scan state, UI state, settings |
| Routing | TanStack Router | Type-safe, three routes: `/`, `/scanner`, `/settings` |
| Forms | React Hook Form + Zod | LLM config, settings |
| i18n | react-i18next | All text externalized, EN/ZH packs |
| Visualization | d3-hierarchy + SVG | Treemap layout calculation, custom React rendering |
| Virtual lists | @tanstack/virtual | File lists with 10k+ items |
| Icons | Lucide React | Consistent icon set |

## Custom Title Bar

Wails runs in `Frameless: true` mode. The system title bar is hidden; a custom React component handles:
- Drag to move (CSS `--wails-draggable: drag`)
- Double-click to maximize/restore
- Minimize, maximize, close buttons via Wails runtime API

This gives full control over the title bar appearance and allows the sidebar to extend to the top edge.

## Directory Structure

```
frontend/src/
├── components/
│   ├── ui/              # shadcn/ui components
│   ├── treemap/         # Treemap visualization
│   ├── file-list/       # Virtual file list
│   ├── chat/            # Chat sidebar
│   ├── dashboard/       # Dashboard widgets
│   └── settings/        # Settings forms
├── hooks/               # TanStack Query hooks for Wails bindings
├── stores/              # Zustand stores
├── i18n/                # Language files (en.json, zh.json)
├── routes/              # TanStack Router route definitions
├── lib/                 # Utilities
└── types/               # ONLY auto-generated types from wails:generate
```

## Key Patterns

### Wails Binding Calls

```typescript
// hooks/useScan.ts — TanStack Query wraps Wails binding
import { Scan } from '../wailsjs/go/internal/App'

export function useScan() {
  return useMutation({
    mutationFn: (params: ScanParams) => Scan(params.path, params.opts),
  })
}
```

### Wails Event Listening

```typescript
// hooks/useScanEvents.ts
import { EventsOn } from '../wailsjs/runtime/runtime'

useEffect(() => {
  EventsOn('scan:chunk', (chunk: FileEntry[]) => {
    // Append to Zustand store
    addFiles(chunk)
  })
  EventsOn('scan:complete', () => {
    // Trigger AI analysis
  })
}, [])
```

### i18n Usage

```tsx
// Always use t() for user-facing text
<h1>{t('dashboard.title')}</h1>
<p>{t('scanner.noFiles')}</p>
```

### Dark Mode

Tailwind `class` strategy. Theme toggle updates `<html>` class:

```tsx
<html className={theme === 'dark' ? 'dark' : ''}>
```

## Design Tokens

Risk level colors:
- `safe` → green (`text-green-600`, `bg-green-50`)
- `caution` → yellow (`text-yellow-600`, `bg-yellow-50`)
- `dangerous` → red (`text-red-600`, `bg-red-50`)
- `analysis_error` → gray (`text-gray-500`, `bg-gray-50`)
