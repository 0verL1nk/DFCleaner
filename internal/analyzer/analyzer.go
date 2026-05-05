package analyzer

import (
	"context"
	"fmt"
	goruntime "runtime"
	"strings"

	"dfcleaner/internal/llm"
	"dfcleaner/internal/scanner"
	"dfcleaner/internal/store"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

type Analyzer struct {
	ctx    context.Context
	store  *store.Store
	llm    *llm.Provider
	agent  *react.Agent
	cancel context.CancelFunc
}

func New(ctx context.Context, l *llm.Provider, s *scanner.Scanner, st *store.Store) *Analyzer {
	return &Analyzer{ctx: ctx, llm: l, store: st}
}

func (a *Analyzer) Chat(message string) error {
	agent, err := a.getOrCreateAgent()
	if err != nil {
		wailsrt.EventsEmit(a.ctx, "chat:error", err.Error())
		return err
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	defer cancel()

	input := []*schema.Message{
		schema.UserMessage(message),
	}

	stream, err := agent.Stream(ctx, input)
	if err != nil {
		wailsrt.EventsEmit(a.ctx, "chat:error", err.Error())
		return err
	}

	defer stream.Close()

	var fullContent strings.Builder
	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		if chunk.Content != "" {
			fullContent.WriteString(chunk.Content)
			wailsrt.EventsEmit(a.ctx, "chat:chunk", chunk.Content)
		}
		if len(chunk.ToolCalls) > 0 {
			for _, tc := range chunk.ToolCalls {
				wailsrt.EventsEmit(a.ctx, "chat:tool_call", map[string]string{
					"name": tc.Function.Name,
				})
			}
		}
	}

	wailsrt.EventsEmit(a.ctx, "chat:complete", fullContent.String())
	return nil
}

func (a *Analyzer) Cancel() {
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *Analyzer) getOrCreateAgent() (*react.Agent, error) {
	if a.agent != nil {
		return a.agent, nil
	}

	chatModel, err := a.llm.GetActiveModel(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("get active LLM model: %w", err)
	}

	s := scanner.New(a.ctx)
	tools, err := scanner.RegisterTools(s)
	if err != nil {
		return nil, fmt.Errorf("register tools: %w", err)
	}

	toolsConfig := compose.ToolsNodeConfig{}
	for _, t := range tools {
		toolsConfig.Tools = append(toolsConfig.Tools, t)
	}

	systemPrompt := a.buildSystemPrompt()

	agentCfg := &react.AgentConfig{
		Model:           chatModel,
		ToolsConfig:     toolsConfig,
		MessageModifier: react.NewPersonaModifier(systemPrompt),
		MaxStep:         0, // no limit
	}

	agent, err := react.NewAgent(a.ctx, agentCfg)
	if err != nil {
		return nil, fmt.Errorf("create react agent: %w", err)
	}

	a.agent = agent
	return a.agent, nil
}

func (a *Analyzer) buildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString("You are DFCleaner, an AI-powered disk cleanup assistant. ")
	sb.WriteString("Your role is to help users analyze files and directories to determine if they are safe to delete.\n\n")

	sb.WriteString("## Available Tools\n")
	sb.WriteString("You have access to these read-only file system tools:\n")
	sb.WriteString("- scan_directory: Scan a directory and get an overview (subdir count, file count, total size, largest items)\n")
	sb.WriteString("- get_file_info: Get detailed metadata for a specific file or directory\n")
	sb.WriteString("- get_large_files: Find files larger than a threshold in a directory tree\n")
	sb.WriteString("- get_old_files: Find files not accessed within N days\n")
	sb.WriteString("- find_duplicates: Find potential duplicate files by name and size\n\n")

	sb.WriteString("## Risk Levels\n")
	sb.WriteString("When analyzing files, assign one of these risk levels:\n")
	sb.WriteString("- safe: Can be safely deleted (cache, temp files, logs, build artifacts)\n")
	sb.WriteString("- caution: Should be careful (old downloads, large media files, app data)\n")
	sb.WriteString("- dangerous: Do not delete (configs, documents, active projects, system files)\n\n")

	sb.WriteString("## Output Format\n")
	sb.WriteString("When providing analysis results, use this JSON format:\n")
	sb.WriteString("```json\n")
	sb.WriteString(`{"risk_level": "safe|caution|dangerous", "reason": "explanation", "category": "cache|temp|log|config|document|media", "confidence": 0.0-1.0}`)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Platform Info\n")
	sb.WriteString(fmt.Sprintf("- OS: %s/%s\n", goruntime.GOOS, goruntime.GOARCH))
	sb.WriteString("- Home directory: use ~/ as shorthand\n")

	if goruntime.GOOS == "darwin" {
		sb.WriteString("- macOS specific: .DS_Store files are safe to delete; ~/Library/Caches is system cache; /Library/Caches is system cache\n")
	} else if goruntime.GOOS == "linux" {
		sb.WriteString("- Linux specific: ~/.cache is user cache; ~/.local/share/Trash is trash; ~/.thumbnails can be cleaned\n")
	} else if goruntime.GOOS == "windows" {
		sb.WriteString("- Windows specific: Thumbs.db files are safe to delete; %TEMP% is temp files; prefetch files in C:\\Windows\\Prefetch are safe\n")
	}

	// Append user custom instructions
	if a.store != nil {
		custom := a.store.GetSetting("custom_ai_instruction")
		if custom != "" {
			sb.WriteString("\n## User Instructions\n")
			sb.WriteString(custom)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
