# Security

## Architecture-Level Constraints

### AI Agent is Read-Only

The AI Agent's Eino Tools can only perform read operations:
- `scan_directory` — list files
- `get_file_info` — read metadata
- `get_large_files` — filter by size
- `get_old_files` — filter by age
- `find_duplicates` — find duplicate files

**No write tools are registered with the Agent.** This is enforced at tool registration time, not via runtime checks. The Agent physically cannot delete or modify files.

### User Confirmation Required

All destructive operations (delete, move, permanent delete) go through the Wails binding layer, triggered only by explicit user action in the UI:
1. User selects files → clicks "Clean"
2. Confirmation dialog shows file list + risk levels + reasons
3. User confirms → Go backend executes
4. Per-item results reported back

### Trash by Default

Deleted files go to the system trash/recycle bin, not permanently deleted:
- Linux: `~/.local/share/Trash/files`
- macOS: `~/.Trash`
- Windows: System recycle bin API

Permanent delete requires explicit user selection + additional confirmation dialog.

## Data Security

### API Key Storage

LLM API keys are encrypted before storage in SQLite:
- Use OS keyring when available (via `keyring` Go library)
- Fallback: AES-256 encryption with machine-specific key
- Keys never appear in plaintext in logs, config files, or UI
- UI shows masked keys: `sk-***abc`

### Privacy: File Content

- **Default**: Only file metadata (name, size, type, path, mtime) is sent to the LLM
- **Opt-in**: Users can enable "content preview" to send first 1KB of file content for more accurate analysis
- The setting is clearly labeled and off by default
- Binary files are never sent as content preview

### Local-First

- All data stored locally in SQLite
- No telemetry, no analytics, no phone-home
- LLM API calls are the only outbound network traffic
- Users choose their own LLM provider (self-hosted Ollama for zero data leakage)

## Attack Surface

| Vector | Mitigation |
|--------|------------|
| AI prompt injection via filenames | Filenames sanitized before sending to LLM; Agent cannot execute shell commands |
| Malicious LLM response | Agent output parsed as structured data, not executed; write operations require user confirmation |
| API key leak | Encrypted at rest, masked in UI, never logged |
| File system race condition | Agent tools are read-only; cleanup operations are sequential |
