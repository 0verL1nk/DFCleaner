# Design Principles

## 1. Application First, Not Chat

DFCleaner is a desktop application with AI embedded as a feature. The main interface is visual (Treemap + file list). AI marks files with risk levels directly in the UI. Chat is a secondary entry point, not the primary interface.

## 2. AI Read-Only, User Executes

The AI Agent can only scan, analyze, and suggest. It cannot delete, move, or modify any file. All write operations are triggered by the user through the UI. This is an architectural constraint enforced at the tool registration level.

## 3. Iterate, Don't Over-Design

AI analysis strategy (trigger timing, batch sizes, depth levels) should be validated with real end-to-end testing. Get the basic flow working first, then optimize based on actual usage data.

## 4. i18n From Day One

All UI text is externalized via i18n keys. No hardcoded strings. Adding a new language should only require translating a JSON file.

## 5. No Degradation

DFCleaner without AI has no value. If LLM is unavailable, the app blocks at setup. We don't provide a "manual mode" fallback.

## 6. Simple > Clever

Three views (Dashboard, Scanner, Settings) instead of five. d3-hierarchy + custom SVG instead of a heavy chart library. GORM AutoMigrate instead of hand-rolled SQL migrations. Prefer the boring, well-tested solution.

## 7. Safety Net

- Deleted files go to system trash by default (recoverable)
- Permanent delete requires extra confirmation
- AI suggestions include risk levels and confidence scores
- Operation history is logged and viewable

## 8. Cross-Platform via Isolation

Platform-specific code lives in `internal/platform/`. The rest of the codebase uses Go standard library (`filepath`, `os`) and is platform-agnostic. Build tags handle per-OS compilation.
