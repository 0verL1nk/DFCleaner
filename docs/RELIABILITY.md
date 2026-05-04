# Reliability

## Error Handling Strategy

### Principle: Per-Item Reporting, No Rollback

Each file operation is independent. A failure on one file does not affect others. Results are reported per-item (success/failure/reason).

### Scan Errors

| Error | Behavior |
|-------|----------|
| Permission denied | Skip directory, mark as "access denied", continue scan |
| Symlink loop | Detect and skip, do not follow |
| Path too long | Skip, log warning |
| Disk I/O error | Skip file, log error, continue |

### AI Analysis Errors

| Error | Behavior |
|-------|----------|
| LLM API unreachable | Block app at setup screen (Q8: no degradation) |
| Rate limit (429) | Retry with exponential backoff, max 3 retries |
| Malformed LLM response | Mark file as `analysis_error`, risk level left empty, user decides |
| LLM timeout | Cancel current request, show partial results |
| Partial batch failure | Successful items processed normally, failed items marked individually |

### Cleanup Errors

| Error | Behavior |
|-------|----------|
| File locked (in use) | Report "file in use", skip, continue |
| Permission denied | Report "permission denied", skip, continue |
| Trash unavailable | Fall back to permanent delete (with extra warning) |
| Disk full during move | Report "disk full", stop remaining operations |

### User Cancellation

Agent execution supports cancellation via Go `context.Context`:
- User clicks "Stop" → context cancelled → Agent stops LLM request + future tool calls
- Already-completed tool results are preserved and shown to user
- If Agent finishes before cancel propagates, show full results normally

## LLM Availability

### Startup Check

App validates LLM connectivity on every startup:
1. Load config from SQLite
2. Send test request with function calling tool definition
3. Verify response contains valid tool call
4. Pass → Dashboard. Fail → Setup/config screen

### Runtime Check

Before each Agent invocation:
1. Quick health check (lightweight API call)
2. If unreachable → show error overlay, user must fix config or retry
3. No background retry loops — user initiates retry explicitly

## Data Integrity

### SQLite

- WAL mode for concurrent read/write
- GORM AutoMigrate for schema evolution
- Transaction wrapping for multi-row operations (cleanup_logs batch insert)

### Scan Result Cache

- Full directory tree stored in frontend Zustand store after scan
- Drill-down uses cache by default
- Directory mtime checked on drill: if changed since scan, auto-rescan that subtree
- User can force full rescan via refresh button
