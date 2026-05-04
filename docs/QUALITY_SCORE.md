# Quality Score

## Testing Standards

### Backend (Go)

| Layer | Minimum Coverage | Test Type |
|-------|-----------------|-----------|
| `internal/scanner` | 80% | Unit + integration (real filesystem) |
| `internal/cleaner` | 90% | Unit + integration (real filesystem with temp dirs) |
| `internal/analyzer` | 70% | Unit (mock LLM) + integration (real LLM with test key) |
| `internal/llm` | 80% | Unit (mock HTTP) |
| `internal/store` | 80% | Unit (in-memory SQLite) |
| `internal/platform` | 90% | Unit + per-OS integration |

### Frontend (React/TypeScript)

| Component | Minimum Coverage | Test Type |
|-----------|-----------------|-----------|
| Treemap | 80% | Component test (mock data) |
| File list | 80% | Component test (virtual list behavior) |
| Chat sidebar | 70% | Component test (mock events) |
| Settings forms | 90% | Component test (form validation) |
| Hooks | 80% | Hook tests (mock Wails bindings) |

### End-to-End

Required E2E scenarios:
1. **Happy path**: Configure LLM → Scan directory → AI analyzes → User confirms cleanup → Files in trash
2. **LLM failure**: App detects unreachable LLM → Shows setup screen
3. **Partial cleanup**: Batch delete → One file fails → Others succeed → Results shown per-item
4. **Chat flow**: User types natural language query → Agent scans → Results in file list
5. **Theme switch**: Toggle dark/light → All components render correctly
6. **Language switch**: Toggle EN/ZH → All text updates

## Code Quality

- **Go**: `golangci-lint` with default config + `go vet`
- **TypeScript**: Strict mode, ESLint + Prettier
- **No `any` types** in TypeScript (except auto-generated Wails bindings if needed)
- **No `// TODO` without a linked issue**

## Performance Benchmarks

| Metric | Target |
|--------|--------|
| Scan 10,000 files | < 5 seconds |
| Treemap render (1,000 items) | < 100ms |
| File list scroll (10,000 items, virtual) | 60fps |
| LLM analysis (50 items batch) | < 10 seconds (depends on model) |
| App cold start | < 2 seconds |
