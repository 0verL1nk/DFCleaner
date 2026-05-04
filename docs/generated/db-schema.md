# Database Schema

> Auto-generated reference. GORM AutoMigrate manages runtime schema. SQL DDL files in `internal/store/schema/`.

## Tables

### llm_configs

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | INTEGER | PK, AUTOINCREMENT | |
| provider | TEXT | NOT NULL | Adapter type: openai, claude, ollama, gemini, deepseek, qwen, ark, openrouter, qianfan |
| endpoint | TEXT | NOT NULL | API endpoint URL |
| api_key | TEXT | NOT NULL | Encrypted API key |
| model_name | TEXT | NOT NULL | Model identifier (e.g., gpt-4o, claude-sonnet-4-20250514) |
| is_active | BOOLEAN | DEFAULT false | Currently active provider |
| created_at | DATETIME | AUTO | |
| updated_at | DATETIME | AUTO | |

### scan_history

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | INTEGER | PK, AUTOINCREMENT | |
| path | TEXT | NOT NULL | Scanned directory path |
| scanned_at | DATETIME | NOT NULL | Timestamp of scan |
| file_count | INTEGER | DEFAULT 0 | Total files found |
| total_size | INTEGER | DEFAULT 0 | Total size in bytes |
| duration_ms | INTEGER | DEFAULT 0 | Scan duration |
| created_at | DATETIME | AUTO | |

### cleanup_logs

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | INTEGER | PK, AUTOINCREMENT | |
| scan_id | INTEGER | FK → scan_history.id | Associated scan |
| file_path | TEXT | NOT NULL | Path of cleaned file |
| operation | TEXT | NOT NULL | trash, delete, or move |
| target_path | TEXT | NULL | Destination for move operations |
| result | TEXT | NOT NULL | success, failed, skipped |
| error_message | TEXT | NULL | Error details if failed |
| freed_bytes | INTEGER | DEFAULT 0 | Space freed |
| created_at | DATETIME | AUTO | |

### user_settings

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | INTEGER | PK, AUTOINCREMENT | |
| key | TEXT | UNIQUE, NOT NULL | Setting key (e.g., theme, language, content_preview) |
| value | TEXT | NOT NULL | Setting value (JSON for complex values) |
| updated_at | DATETIME | AUTO | |

## Key Settings (user_settings)

| Key | Values | Default | Description |
|-----|--------|---------|-------------|
| theme | light, dark, system | system | UI theme |
| language | en, zh | en | UI language |
| content_preview | true, false | false | Send file content (1KB) to LLM |
| default_delete_mode | trash, permanent | trash | Default cleanup mode |
| custom_ai_instruction | string | "" | Appended to AI System Prompt |
