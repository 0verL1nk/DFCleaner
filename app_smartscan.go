package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"dfcleaner/internal/scanner"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

type markCleanableInput struct {
	Path      string `json:"path" jsonschema:"description=Full path of the file or directory"`
	Name      string `json:"name" jsonschema:"description=Name of the file or directory"`
	Size      int64  `json:"size" jsonschema:"description=Size in bytes"`
	IsDir     bool   `json:"isDir" jsonschema:"description=Whether this is a directory"`
	RiskLevel string `json:"riskLevel" jsonschema:"description=Risk level,enum=safe,enum=caution"`
	Reason    string `json:"reason" jsonschema:"description=Why this item can be cleaned"`
	Category  string `json:"category" jsonschema:"description=Category,enum=cache,enum=temp,enum=log,enum=build,enum=download,enum=media,enum=other"`
}

type markCleanableOutput struct {
	Success bool `json:"success"`
}

func (a *App) SmartScan(path string, opts scanner.ScanOptions) error {
	a.logger.Printf("SmartScan: path=%s", path)

	ctx, cancel := context.WithCancel(a.ctx)
	a.smartScanCancel = cancel

	wailsrt.EventsEmit(a.ctx, "smartscan:progress", map[string]any{
		"phase":         "analyzing",
		"dirsExplored":  0,
		"itemsFound":    0,
		"currentAction": "Initializing AI agent...",
	})

	go a.runSmartScan(ctx, path)
	return nil
}

func (a *App) CancelSmartScan() {
	if a.smartScanCancel != nil {
		a.smartScanCancel()
	}
}

func (a *App) runSmartScan(ctx context.Context, rootPath string) {
	chatModel, err := a.llm.GetActiveModel(ctx)
	if err != nil {
		wailsrt.EventsEmit(a.ctx, "smartscan:error", "LLM not configured: "+err.Error())
		return
	}

	s := scanner.New(ctx)
	scanTools, err := scanner.RegisterTools(s)
	if err != nil {
		wailsrt.EventsEmit(a.ctx, "smartscan:error", err.Error())
		return
	}

	var mu sync.Mutex
	itemsFound := 0
	dirsExplored := 0

	markTool, err := utils.InferTool("mark_cleanable",
		"Mark a file or directory as cleanable. Call this for each item you determine should be cleaned. Only mark items as 'safe' or 'caution'.",
		func(toolCtx context.Context, input *markCleanableInput) (*markCleanableOutput, error) {
			if input.RiskLevel != "safe" && input.RiskLevel != "caution" {
				return &markCleanableOutput{Success: false}, nil
			}

			mu.Lock()
			itemsFound++
			count := itemsFound
			mu.Unlock()

			wailsrt.EventsEmit(a.ctx, "smartscan:items", []map[string]any{{
				"path":      input.Path,
				"name":      input.Name,
				"size":      input.Size,
				"isDir":     input.IsDir,
				"riskLevel": input.RiskLevel,
				"reason":    input.Reason,
				"category":  input.Category,
			}})

			mu.Lock()
			de := dirsExplored
			mu.Unlock()
			wailsrt.EventsEmit(a.ctx, "smartscan:progress", map[string]any{
				"phase":         "analyzing",
				"dirsExplored":  de,
				"itemsFound":    count,
				"currentAction": fmt.Sprintf("Marked: %s", input.Name),
			})

			return &markCleanableOutput{Success: true}, nil
		})
	if err != nil {
		wailsrt.EventsEmit(a.ctx, "smartscan:error", err.Error())
		return
	}

	toolsConfig := compose.ToolsNodeConfig{}
	for _, t := range append(scanTools, markTool) {
		toolsConfig.Tools = append(toolsConfig.Tools, t)
	}

	agentCfg := &react.AgentConfig{
		Model:           chatModel,
		ToolsConfig:     toolsConfig,
		MessageModifier: react.NewPersonaModifier(smartScanSystemPrompt),
		MaxStep:         150,
	}

	agent, err := react.NewAgent(ctx, agentCfg)
	if err != nil {
		wailsrt.EventsEmit(a.ctx, "smartscan:error", "Failed to create agent: "+err.Error())
		return
	}

	input := []*schema.Message{
		schema.UserMessage(fmt.Sprintf(
			"Explore the directory tree starting from: %s\n\n"+
				"Steps:\n"+
				"1. Scan the root directory to understand the structure\n"+
				"2. Recursively explore subdirectories that might contain cleanable items\n"+
				"3. Use get_large_files and get_old_files in promising directories\n"+
				"4. Mark each cleanable item with mark_cleanable\n"+
				"5. Be thorough - explore at least 2 levels deep in promising directories\n"+
				"6. Skip clearly important directories (Documents, Projects with .git) quickly",
			rootPath,
		)),
	}

	stream, err := agent.Stream(ctx, input)
	if err != nil {
		wailsrt.EventsEmit(a.ctx, "smartscan:error", err.Error())
		return
	}
	defer stream.Close()

	for {
		select {
		case <-ctx.Done():
			emitComplete(a.ctx, &mu, &itemsFound)
			return
		default:
		}

		chunk, err := stream.Recv()
		if err != nil {
			break
		}

		for _, tc := range chunk.ToolCalls {
			mu.Lock()
			if tc.Function.Name == "scan_directory" {
				dirsExplored++
			}
			de := dirsExplored
			ifound := itemsFound
			mu.Unlock()

			action := toolDisplayName(tc.Function.Name)
			if tc.Function.Name == "scan_directory" {
				var args struct{ Path string `json:"path"` }
				if json.Unmarshal([]byte(tc.Function.Arguments), &args) == nil && args.Path != "" {
					action = fmt.Sprintf("Exploring: %s", args.Path)
				}
			}

			wailsrt.EventsEmit(a.ctx, "smartscan:progress", map[string]any{
				"phase":         "analyzing",
				"dirsExplored":  de,
				"itemsFound":    ifound,
				"currentAction": action,
			})
		}
	}

	emitComplete(a.ctx, &mu, &itemsFound)
}

func emitComplete(ctx context.Context, mu *sync.Mutex, itemsFound *int) {
	mu.Lock()
	total := *itemsFound
	mu.Unlock()
	wailsrt.EventsEmit(ctx, "smartscan:complete", map[string]any{
		"itemsFound": total,
	})
}

func toolDisplayName(name string) string {
	switch name {
	case "scan_directory":
		return "Scanning directory..."
	case "get_file_info":
		return "Inspecting file..."
	case "get_large_files":
		return "Finding large files..."
	case "get_old_files":
		return "Finding old files..."
	case "find_duplicates":
		return "Finding duplicates..."
	case "mark_cleanable":
		return "Marking cleanable..."
	default:
		return name
	}
}

const smartScanSystemPrompt = `You are DFCleaner's autonomous disk cleanup agent. Your task is to explore directories and identify files/folders that can be safely cleaned up.

## Strategy
1. Start by scanning the root directory to understand the overall structure
2. Prioritize exploring directories likely to contain cleanable items:
   - Cache: .cache, __pycache__, .npm, .yarn, node_modules/.cache, .gradle/caches, .m2/repository, .pnpm-store, .cargo/registry, .nuget, pip cache, go module cache
   - Temp: tmp, temp, .tmp
   - Build: dist, build, .next, target, bin, obj, out, .output
   - Logs: logs, *.log
   - Downloads: old or large files in Downloads
   - Trash/recycle bins
3. For each promising directory, use scan_directory to see what's inside
4. Use get_large_files and get_old_files to find candidates
5. Use get_file_info to examine specific items when needed
6. For each cleanable item, call mark_cleanable

## Rules
- NEVER mark system directories as cleanable
- NEVER mark user document folders (Documents, Desktop) as cleanable
- Skip directories with .git (active projects) except for build artifacts inside them
- Only mark items as "safe" or "caution" - never "dangerous"
- Be thorough: explore at least 2 levels deep in promising directories
- Skip directories you can't access
- If a directory is clearly not cleanable, skip it quickly
- Mark cache/temp/build artifacts as "safe"
- Mark old downloads, large media files as "caution"
- Always provide a clear reason

## Categories
- cache: Cached data (.cache, npm cache, etc.)
- temp: Temporary files (tmp, temp)
- log: Log files
- build: Build artifacts (dist, build, target)
- download: Old/large downloads
- media: Large media files
- other: Items that don't fit other categories`
