package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"dfcleaner/internal/scanner"
	"dfcleaner/internal/store"

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
	a.logger.Printf("[SmartScan] starting for path=%s", path)

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
	a.logger.Printf("[SmartScan] cancel requested")
	if a.smartScanCancel != nil {
		a.smartScanCancel()
	}
}

func (a *App) runSmartScan(ctx context.Context, rootPath string) {
	a.logger.Printf("[SmartScan] runSmartScan: root=%s", rootPath)

	chatModel, err := a.llm.GetActiveModel(ctx)
	if err != nil {
		a.logger.Printf("[SmartScan] ERROR: LLM not configured: %v", err)
		wailsrt.EventsEmit(a.ctx, "smartscan:error", "LLM not configured: "+err.Error())
		return
	}
	a.logger.Printf("[SmartScan] LLM model acquired successfully")

	s := scanner.New(ctx)
	scanTools, err := scanner.RegisterTools(s)
	if err != nil {
		a.logger.Printf("[SmartScan] ERROR: failed to register tools: %v", err)
		wailsrt.EventsEmit(a.ctx, "smartscan:error", err.Error())
		return
	}
	a.logger.Printf("[SmartScan] %d scanner tools registered", len(scanTools))

	var mu sync.Mutex
	itemsFound := 0
	dirsExplored := 0

	markTool, err := utils.InferTool("mark_cleanable",
		"Mark a file or directory as cleanable. Call this for each item you determine should be cleaned. Only mark items as 'safe' or 'caution'.",
		func(toolCtx context.Context, input *markCleanableInput) (*markCleanableOutput, error) {
			if input.RiskLevel != "safe" && input.RiskLevel != "caution" {
				a.logger.Printf("[SmartScan] mark_cleanable REJECTED: path=%s riskLevel=%s (only safe/caution allowed)", input.Path, input.RiskLevel)
				return &markCleanableOutput{Success: false}, nil
			}

			mu.Lock()
			itemsFound++
			count := itemsFound
			mu.Unlock()

			a.logger.Printf("[SmartScan] mark_cleanable #%d: name=%s path=%s size=%d risk=%s category=%s reason=%q",
				count, input.Name, input.Path, input.Size, input.RiskLevel, input.Category, input.Reason)

			// Persist to database
			if a.store != nil {
				a.store.SaveCleanableItem(&store.CleanableItemDB{
					Path:      input.Path,
					Name:      input.Name,
					Size:      input.Size,
					IsDir:     input.IsDir,
					RiskLevel: input.RiskLevel,
					Reason:    input.Reason,
					Category:  input.Category,
					ScanPath:  rootPath,
				})
			}

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
		a.logger.Printf("[SmartScan] ERROR: failed to create mark_cleanable tool: %v", err)
		wailsrt.EventsEmit(a.ctx, "smartscan:error", err.Error())
		return
	}

	toolsConfig := compose.ToolsNodeConfig{}
	for _, t := range append(scanTools, markTool) {
		toolsConfig.Tools = append(toolsConfig.Tools, t)
	}
	a.logger.Printf("[SmartScan] Agent config: %d total tools, maxStep=150", len(toolsConfig.Tools))

	agentCfg := &react.AgentConfig{
		Model:           chatModel,
		ToolsConfig:     toolsConfig,
		MessageModifier: react.NewPersonaModifier(smartScanSystemPrompt),
		MaxStep:         150,
	}

	agent, err := react.NewAgent(ctx, agentCfg)
	if err != nil {
		a.logger.Printf("[SmartScan] ERROR: failed to create agent: %v", err)
		wailsrt.EventsEmit(a.ctx, "smartscan:error", "Failed to create agent: "+err.Error())
		return
	}
	a.logger.Printf("[SmartScan] Agent created successfully, starting stream")

	input := []*schema.Message{
		schema.UserMessage(fmt.Sprintf(
			"Explore and clean: %s\n\nScan this directory, use subDirSummary to find large subdirectories, explore them, and mark cleanable items as you find them.",
			rootPath,
		)),
	}

	stream, err := agent.Stream(ctx, input)
	if err != nil {
		a.logger.Printf("[SmartScan] ERROR: failed to start agent stream: %v", err)
		wailsrt.EventsEmit(a.ctx, "smartscan:error", err.Error())
		return
	}
	defer stream.Close()

	stepCount := 0
	for {
		select {
		case <-ctx.Done():
			a.logger.Printf("[SmartScan] cancelled after %d steps, %d dirs, %d items", stepCount, dirsExplored, itemsFound)
			emitComplete(a.ctx, &mu, &itemsFound)
			return
		default:
		}

		chunk, err := stream.Recv()
		if err != nil {
			a.logger.Printf("[SmartScan] stream ended after %d steps, %d dirs explored, %d items found: %v",
				stepCount, dirsExplored, itemsFound, err)
			break
		}

		// Emit Agent's reasoning text as current activity
		if chunk.Content != "" {
			mu.Lock()
			de := dirsExplored
			ifound := itemsFound
			mu.Unlock()

			content := chunk.Content
			if len(content) > 200 {
				content = content[:200] + "..."
			}

			a.logger.Printf("[SmartScan] agent thinking: %s", content)

			wailsrt.EventsEmit(a.ctx, "smartscan:progress", map[string]any{
				"phase":         "analyzing",
				"dirsExplored":  de,
				"itemsFound":    ifound,
				"currentAction": content,
			})
		}

		for _, tc := range chunk.ToolCalls {
			// Skip incremental streaming chunks (empty tool name = continuation of previous call)
			if tc.Function.Name == "" {
				a.logger.Printf("[SmartScan] stream chunk continuation: args_part=%s", tc.Function.Arguments)
				continue
			}

			stepCount++
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

			a.logger.Printf("[SmartScan] step #%d: tool=%s args=%s dirs=%d items=%d",
				stepCount, tc.Function.Name, tc.Function.Arguments, de, ifound)

			wailsrt.EventsEmit(a.ctx, "smartscan:progress", map[string]any{
				"phase":         "analyzing",
				"dirsExplored":  de,
				"itemsFound":    ifound,
				"currentAction": action,
			})
		}
	}

	mu.Lock()
	total := itemsFound
	mu.Unlock()
	a.logger.Printf("[SmartScan] complete: %d steps, %d dirs explored, %d items found", stepCount, dirsExplored, total)
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
	case "view_result":
		return "Viewing results..."
	default:
		return name
	}
}

const smartScanSystemPrompt = `You are DFCleaner's autonomous disk cleanup agent. Your task is to explore directories and identify files/folders that can be safely cleaned up.

## Key: How scan_directory works
When you call scan_directory, you receive:
- subDirSummary: A list of ALL subdirectories with their file counts and total sizes, sorted largest first. USE THIS to decide where to explore next.
- largestItems: ALL files sorted by size (no limit). If too many to fit in one response, moreItems > 0 and resultFile is set.
- When resultFile is set, call view_result with {path: resultFile, offset: 0, limit: 50} to see more items.

## Strategy
1. Start by scanning the root directory
2. Look at subDirSummary — pick the largest or most suspicious subdirectories to explore next
3. For each promising subdirectory, call scan_directory again to drill deeper
4. When you find cleanable items (directories or files), call mark_cleanable IMMEDIATELY — do not wait until you finish exploring
5. Use get_large_files (threshold > 50MB) and get_old_files (90+ days) in directories with many files
6. Be aggressive: if a directory name matches known cleanable patterns, explore it and mark items
7. If a tool response has moreItems > 0 and a resultFile, you can call view_result to see more — but focus on marking items you already see first

## What to mark as cleanable
Mark these as "safe":
- Cache directories (.cache, __pycache__, .npm, .yarn, .gradle/caches, .m2, .cargo, .pnpm-store, .conda, pip cache)
- Build output (dist, build, target, .next, .nuxt, .output, .svelte-kit, out)
- Package dependencies (node_modules — these are fully reinstallable)
- Temp files (tmp, temp, .tmp)
- Log files over 1MB
- Thumbnail caches (.thumbnails)

Mark these as "caution":
- Old large files in Downloads (>30 days, >100MB)
- Duplicate files (same name+size in multiple locations)
- Old media files that haven't been accessed in 180+ days

## Rules
- NEVER mark system directories (Windows, System32, /usr, /bin, /etc, /lib)
- NEVER mark user document folders (Documents, Desktop)
- NEVER mark directories containing .git as cleanable (active projects)
- Only mark items as "safe" or "caution"
- Skip directories you can't access
- Always provide a clear reason in the reason field
- When in doubt about a directory, explore it first with scan_directory before marking

## Categories
- cache: Cached data that can be regenerated
- temp: Temporary files
- log: Log files
- build: Build artifacts that can be rebuilt
- download: Old/large downloads
- media: Large media files
- other: Items that don't fit other categories`
