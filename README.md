<div align="center">

<img src="build/appicon.svg" width="128" height="128" alt="DFCleaner Logo" />

# DFCleaner

**基于 AI 的跨平台磁盘清理工具**

[![CI](https://github.com/0verL1nk/DFCleaner/actions/workflows/ci.yml/badge.svg)](https://github.com/0verL1nk/DFCleaner/actions/workflows/ci.yml)
[![Release](https://github.com/0verL1nk/DFCleaner/actions/workflows/release.yml/badge.svg)](https://github.com/0verL1nk/DFCleaner/actions/workflows/release.yml)
[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[中文](#功能特性) | [English](#features-en)

</div>

---

传统的磁盘清理工具依赖硬编码规则，无法理解文件的实际用途。DFCleaner 接入你自己的大模型（OpenAI、DeepSeek、Claude、Ollama 等），通过 AI 语义分析标记文件风险等级，让你放心清理而不再靠猜。

## 功能特性

- **AI 智能分析** — 扫描后自动分析文件，标记为安全 / 需谨慎 / 不建议删除，并给出理由
- **自带大模型** — 支持 OpenAI、DeepSeek、Claude、OpenRouter、百度千帆、Ollama 及任何 OpenAI 兼容 API
- **跨平台** — 基于 [Wails](https://wails.io) 的原生桌面应用，支持 Linux、macOS、Windows
- **智能快速扫描** — 预置常用清理目标：下载、缓存、临时文件、回收站、缩略图，以及平台级缓存（npm、pnpm、Cargo、Gradle 等）
- **磁盘自动识别** — 自动检测所有挂载的磁盘和分区
- **安全清理** — 支持移到回收站（可恢复）或永久删除，逐项跟踪进度
- **隐私优先** — API 密钥本地 AES-GCM 加密存储，所有处理都在你的机器上完成
- **多语言 & 主题** — 支持中文 / 英文，深色 / 浅色 / 跟随系统主题

## 下载

从 [Releases 页面](https://github.com/0verL1nk/DFCleaner/releases) 下载最新版本。

| 平台 | 架构 | 安装包 | 便携版 |
|------|------|--------|--------|
| Windows | amd64 | `DFCleaner-amd64-installer.exe` | `DFCleaner-windows-amd64-portable.zip` |
| macOS | arm64 | `DFCleaner-darwin-arm64.dmg` | `DFCleaner-darwin-arm64-portable.tar.gz` |
| Linux | amd64 | — | `DFCleaner-linux-amd64-portable.tar.gz` |

## 快速开始

### 从源码构建

前置依赖：
- [Go 1.23+](https://go.dev/dl/)
- [Node.js 22+](https://nodejs.org/) + [pnpm](https://pnpm.io/)
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)
- Linux 需要 `libwebkit2gtk-4.1-dev`

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest

git clone https://github.com/0verL1nk/DFCleaner.git
cd DFCleaner

# 开发模式
wails dev -tags webkit2_41

# 生产构建
wails build -tags webkit2_41
```

## 工作流程

```
  扫描 ──► 遍历目录树（并发池，分块输出）
    │
    ▼
  分析 ──► AI 批量分析文件元数据，标记风险等级
    │
    ▼
  筛选 ──► 实时展示可清理项（安全 / 需谨慎）
    │
    ▼
  清理 ──► 回收站或永久删除（逐项进度跟踪）
```

## 技术栈

| 层级 | 技术 |
|------|------|
| 桌面框架 | [Wails v2](https://wails.io)（Go + WebView） |
| 后端 | Go、[GORM](https://gorm.io)、SQLite |
| AI Agent | [CloudWeGo Eino](https://github.com/cloudwego/eino) |
| 大模型 | eino-ext（OpenAI、DeepSeek、Claude、Ollama 等） |
| 前端 | React、TypeScript、[Vite](https://vitejs.dev) |
| UI | [Tailwind CSS v4](https://tailwindcss.com)、[shadcn/ui](https://ui.shadcn.com)、[Radix UI](https://www.radix-ui.com) |
| 状态管理 | [Zustand](https://zustand-demo.pmnd.rs)、[TanStack Query](https://tanstack.com/query) |

## 配置

首次启动后，进入 **设置** 页面配置大模型：

1. 选择 **服务商**（OpenAI、DeepSeek、Claude、Ollama 等）
2. 填写 **API 地址**（如 `https://api.openai.com/v1`）
3. 填写 **API 密钥**（AES-GCM 加密本地存储）
4. 填写 **模型名称**（如 `gpt-4o`、`deepseek-chat`）
5. 点击 **测试连接** 验证

---

## Features (EN)

- **AI-Powered Analysis** — Automatically scans and analyzes files, marking them as Safe / Caution / Dangerous with explanations
- **Bring Your Own LLM** — Supports OpenAI, DeepSeek, Claude, OpenRouter, Qianfan, Ollama, and any OpenAI-compatible API
- **Cross-Platform** — Native desktop app for Linux, macOS, and Windows via Wails
- **Smart Quick Targets** — Pre-configured scan targets: Downloads, Cache, Temp, Trash, Thumbnails, and platform-specific caches
- **Safe Cleanup** — Move to trash or permanent delete, with per-item progress tracking
- **Privacy First** — API keys encrypted locally (AES-GCM), all processing on your machine

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/my-feature`)
3. Commit your changes (`git commit -m 'feat: add my feature'`)
4. Push to the branch (`git push origin feat/my-feature`)
5. Open a Pull Request

## License

[MIT](LICENSE)
