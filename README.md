<div align="center">

<img src="build/appicon.svg" width="128" height="128" alt="DFCleaner Logo" />

# DFCleaner

**AI-Powered Cross-Platform Disk Cleanup Tool**

[![CI](https://github.com/0verL1nk/DFCleaner/actions/workflows/ci.yml/badge.svg)](https://github.com/0verL1nk/DFCleaner/actions/workflows/ci.yml)
[![Release](https://github.com/0verL1nk/DFCleaner/actions/workflows/release.yml/badge.svg)](https://github.com/0verL1nk/DFCleaner/actions/workflows/release.yml)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[English](#features) | [中文](#功能特性)

</div>

---

Traditional disk cleaners rely on hardcoded rules — they don't understand *what* your files are or *why* they exist. DFCleaner uses your own LLM (OpenAI, DeepSeek, Claude, Ollama, etc.) to analyze files semantically and mark them with risk levels, so you can clean with confidence instead of guessing.

## Features

- **AI-Powered Analysis** — Automatically scans and analyzes files after each scan, marking them as Safe / Caution / Dangerous with explanations
- **Bring Your Own LLM** — Supports OpenAI, DeepSeek, Claude, OpenRouter, Qianfan, Ollama, and any OpenAI-compatible API
- **Cross-Platform** — Native desktop app for Linux, macOS, and Windows via [Wails](https://wails.io)
- **Smart Quick Targets** — Pre-configured scan targets: Downloads, Cache, Temp, Trash, Thumbnails, and platform-specific caches (npm, pnpm, Cargo, Gradle, etc.)
- **System Drive Detection** — Auto-detects all mounted drives and partitions
- **Safe Cleanup** — Move to trash (recoverable) or permanent delete, with per-item progress tracking
- **Privacy First** — API keys encrypted locally (AES-GCM), all processing happens on your machine
- **Dark / Light / System Theme** — With i18n support (English & Chinese)

## Screenshots

> *Coming soon — first release in progress*

## Download

Grab the latest release for your platform from the [Releases page](https://github.com/0verL1nk/DFCleaner/releases).

| Platform | Arch | File |
|----------|------|------|
| Linux | amd64 | `dfcleaner-linux-amd64` |
| macOS | arm64 | `DFCleaner.app` |
| Windows | amd64 | `DFCleaner.exe` |

## Quick Start

### From Source

Prerequisites:
- [Go 1.23+](https://go.dev/dl/)
- [Node.js 22+](https://nodejs.org/) with [pnpm](https://pnpm.io/)
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)
- Linux: `libwebkit2gtk-4.1-dev`

```bash
# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Clone
git clone https://github.com/0verL1nk/DFCleaner.git
cd DFCleaner

# Run in dev mode
wails dev -tags webkit2_41

# Build production binary
wails build -tags webkit2_41
```

## How It Works

```
┌──────────────────────────────────────────────┐
│              DFCleaner Flow                   │
│                                              │
│  1. Scan ──► Walk directory tree             │
│       │       (goroutine pool, chunked emit) │
│       ▼                                      │
│  2. Analyze ──► Send file metadata to LLM    │
│       │         Batch analysis, risk levels   │
│       ▼                                      │
│  3. Review ──► Sort by risk, filter, select  │
│       │         (Safe / Caution / Dangerous)  │
│       ▼                                      │
│  4. Clean ──► Trash or permanent delete      │
│               (per-item progress & logging)   │
└──────────────────────────────────────────────┘
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Desktop Framework | [Wails v2](https://wails.io) (Go + WebView) |
| Backend | Go, [GORM](https://gorm.io), SQLite |
| AI Agent | [CloudWeGo Eino](https://github.com/cloudwego/eino) |
| LLM Providers | eino-ext (OpenAI, DeepSeek, Claude, Ollama, ...) |
| Frontend | React, TypeScript, [Vite](https://vitejs.dev) |
| UI | [Tailwind CSS v4](https://tailwindcss.com), [shadcn/ui](https://ui.shadcn.com), [Radix UI](https://www.radix-ui.com) |
| State | [Zustand](https://zustand-demo.pmnd.rs), [TanStack Query](https://tanstack.com/query) |
| Routing | [TanStack Router](https://tanstack.com/router) |

## Project Structure

```
DFCleaner/
├── app.go                          # Wails-bound Go methods
├── main.go                         # App entry point
├── wails.json                      # Wails project config
├── internal/
│   ├── analyzer/                   # AI batch analysis (Eino)
│   ├── cleaner/                    # File cleanup (trash/delete)
│   ├── llm/                        # LLM provider + API key encryption
│   ├── platform/                   # OS-specific: drives, quick targets
│   ├── scanner/                    # Directory scanner + AI tools
│   └── store/                      # SQLite models + GORM queries
├── frontend/
│   ├── src/
│   │   ├── components/ui/          # shadcn/ui components
│   │   ├── hooks/wails.ts          # TanStack Query hooks for Go bindings
│   │   ├── views/                  # Dashboard, Scanner, Settings
│   │   ├── stores/                 # Zustand state
│   │   └── i18n/                   # en.json, zh.json
│   └── wailsjs/                    # Auto-generated Wails bindings
├── build/                          # Cross-platform build assets
│   ├── darwin/                     # macOS .plist files
│   └── windows/                    # .ico, manifest, NSIS installer
└── .github/workflows/              # CI + multi-platform release
```

## Configuration

DFCleaner stores all config in a local SQLite database (`dfcleaner.db`). No config files to manage.

On first launch, go to **Settings** and configure your LLM:
1. Select a **Provider** (OpenAI, DeepSeek, Claude, Ollama, etc.)
2. Enter the **API Endpoint** (e.g. `https://api.openai.com/v1`)
3. Enter your **API Key** (stored encrypted with AES-GCM)
4. Enter the **Model Name** (e.g. `gpt-4o`, `deepseek-chat`)
5. Click **Test Connection** to verify

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/my-feature`)
3. Commit your changes (`git commit -m 'feat: add my feature'`)
4. Push to the branch (`git push origin feat/my-feature`)
5. Open a Pull Request

## License

[MIT](LICENSE)
