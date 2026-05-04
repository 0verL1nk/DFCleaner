# Product Sense

## Core Value Proposition

DFCleaner answers one question that existing disk cleanup tools can't: **"Is this file safe to delete?"**

Not "is it a temp file?" or "is it in a cache directory?" — but actually understanding what the file is, where it came from, and what happens if you delete it. And explaining why in plain language.

## Why Not Just Use Claude Code / Codex?

| What DFCleaner provides | What CLI agents can't |
|--------------------------|----------------------|
| Treemap visualization — see space distribution at a glance | Text output can't show spatial relationships |
| Safe deletion pipeline — trash, confirmation, risk badges | `rm -rf` has no safety net |
| Structured file tools — pre-processed metadata, not raw `ls` output | Generic shell commands waste tokens on raw data |
| Desktop UX — non-developers can use it | Requires terminal proficiency |
| AI analysis inline in file list — see risk levels while browsing | Separate from file management |

## Target Users

**Primary**: Developers and power users who understand disk space is finite but don't want to spend hours deciding what to delete.

**Secondary**: Non-technical users who are afraid to delete anything because they don't know what's safe.

## UX Principles

### 1. Application, Not Chatbot

Users interact with a visual interface first. The Treemap and file list are the primary views. AI results appear inline as colored badges. Chat is available but not required.

### 2. Progressive Disclosure

- Dashboard: high-level space summary
- Scanner: file-level detail with AI suggestions
- Click a file: see AI's full reasoning
- Chat: ask complex questions in natural language

### 3. Color = Risk

Users should understand risk at a glance:
- Green = safe to delete (temp files, caches)
- Yellow = be careful (old logs, large downloads you might need)
- Red = don't delete (configs, documents, active projects)
- Gray = AI couldn't analyze (analysis error)

### 4. Trust Through Transparency

Every AI suggestion shows:
- Risk level with color
- Category (cache, temp, log, config, document, media)
- Natural language reason ("This is X's cache directory, safe to delete, will be recreated")
- Confidence score

Users can see WHY the AI recommends something, not just THAT it recommends it.

### 5. Safe by Default

- Trash, not permanent delete
- Confirmation dialog shows what will happen
- Operation history is always accessible
- No "select all + delete" without explicit risk acknowledgment

## i18n Philosophy

The app supports English and Chinese from day one. AI output language follows user locale. This is not optional — i18n infrastructure is a hard requirement from v1.

## Theme

Dark/light mode follows system setting. Users can override. Risk level colors work in both themes.
