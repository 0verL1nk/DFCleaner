package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"dfcleaner/internal/scanner"
	"dfcleaner/internal/store"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/tool/utils"
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

	allTools := append(scanTools, markTool)
	a.logger.Printf("[SmartScan] Agent config: %d total tools, maxIterations=150", len(allTools))

	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "DFCleaner",
		Description: "Autonomous disk cleanup agent that explores directories and identifies cleanable files",
		Instruction: smartScanSystemPrompt,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{Tools: allTools},
		},
		MaxIterations: 150,
	})
	if err != nil {
		a.logger.Printf("[SmartScan] ERROR: failed to create ADK agent: %v", err)
		wailsrt.EventsEmit(a.ctx, "smartscan:error", "Failed to create agent: "+err.Error())
		return
	}
	a.logger.Printf("[SmartScan] ADK agent created successfully, starting run")

	iter := agent.Run(ctx, &adk.AgentInput{
		Messages: []*schema.Message{
			schema.UserMessage(fmt.Sprintf(
				"Explore and clean: %s\n\nScan this directory, use subDirSummary to find large subdirectories, explore them, and mark cleanable items as you find them.",
				rootPath,
			)),
		},
		EnableStreaming: true,
	})

	stepCount := 0
	for {
		select {
		case <-ctx.Done():
			a.logger.Printf("[SmartScan] cancelled after %d steps, %d dirs, %d items", stepCount, dirsExplored, itemsFound)
			emitComplete(a.ctx, &mu, &itemsFound)
			return
		default:
		}

		event, ok := iter.Next()
		if !ok {
			a.logger.Printf("[SmartScan] iterator ended after %d steps, %d dirs explored, %d items found",
				stepCount, dirsExplored, itemsFound)
			break
		}

		if event.Err != nil {
			a.logger.Printf("[SmartScan] event error: %v", event.Err)
			continue
		}

		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}

		mv := event.Output.MessageOutput

		// Handle non-streaming text content
		if !mv.IsStreaming && mv.Message != nil {
			processAgentMessage(a, mv.Message, &mu, &dirsExplored, &itemsFound, &stepCount)
		}

		// Handle streaming messages
		if mv.IsStreaming && mv.MessageStream != nil {
			for {
				frame, err := mv.MessageStream.Recv()
				if err != nil {
					break
				}
				processAgentMessage(a, frame, &mu, &dirsExplored, &itemsFound, &stepCount)
			}
		}
	}

	mu.Lock()
	total := itemsFound
	mu.Unlock()
	a.logger.Printf("[SmartScan] complete: %d steps, %d dirs explored, %d items found", stepCount, dirsExplored, total)
	emitComplete(a.ctx, &mu, &itemsFound)
}

func processAgentMessage(a *App, msg *schema.Message, mu *sync.Mutex, dirsExplored, itemsFound, stepCount *int) {
	if msg.Content != "" {
		mu.Lock()
		de := *dirsExplored
		ifound := *itemsFound
		mu.Unlock()

		content := msg.Content
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

	for _, tc := range msg.ToolCalls {
		if tc.Function.Name == "" {
			continue
		}

		mu.Lock()
		*stepCount++
		if tc.Function.Name == "scan_directory" {
			*dirsExplored++
		}
		de := *dirsExplored
		ifound := *itemsFound
		sc := *stepCount
		mu.Unlock()

		action := toolDisplayName(tc.Function.Name)
		if tc.Function.Name == "scan_directory" {
			var args struct{ Path string `json:"path"` }
			if json.Unmarshal([]byte(tc.Function.Arguments), &args) == nil && args.Path != "" {
				action = fmt.Sprintf("Exploring: %s", args.Path)
			}
		}

		a.logger.Printf("[SmartScan] step #%d: tool=%s args=%s dirs=%d items=%d",
			sc, tc.Function.Name, tc.Function.Arguments, de, ifound)

		wailsrt.EventsEmit(a.ctx, "smartscan:progress", map[string]any{
			"phase":         "analyzing",
			"dirsExplored":  de,
			"itemsFound":    ifound,
			"currentAction": action,
		})
	}
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
	case "view_result":
		return "Viewing results..."
	case "read_file":
		return "Reading file..."
	case "read_file_tail":
		return "Reading file tail..."
	case "search_files":
		return "Searching files..."
	case "grep_content":
		return "Searching content..."
	case "get_directory_size":
		return "Calculating size..."
	case "analyze_disk_usage":
		return "Analyzing disk usage..."
	case "get_system_info":
		return "Getting system info..."
	case "calculate_hash":
		return "Calculating hash..."
	case "check_package_manager":
		return "Checking package manager..."
	case "get_path_info":
		return "Analyzing path..."
	case "mark_cleanable":
		return "Marking cleanable..."
	default:
		return name
	}
}

const smartScanSystemPrompt = `You are DFCleaner's autonomous disk cleanup agent. Your task is to explore directories and identify files/folders that can be safely cleaned up.

## Available Tools

### Directory exploration
- scan_directory: Scan a directory. Returns subDirSummary (all subdirs sorted by size), file list, total size.
- get_file_info: Get detailed metadata for a file or directory.
- view_result: Page through large results from other tools.

### File content inspection
- read_file: Read text file contents (first N lines). Use to inspect package.json, config files, log headers.
- read_file_tail: Read last N lines of a file. Use for recent log entries.
- search_files: Find files matching a glob pattern (e.g. *.log, *.tmp).
- grep_content: Search text inside files. Like grep.

### Disk analysis
- get_large_files: Find files larger than a threshold.
- get_old_files: Find files not accessed in N days.
- find_duplicates: Find duplicate files by name and size.
- get_directory_size: Calculate exact directory size recursively.
- analyze_disk_usage: Breakdown by file extension and category.
- calculate_hash: MD5 or SHA256 hash for precise duplicate detection.

### System & context
- get_system_info: OS, architecture, disk space (total/used/free).
- check_package_manager: Detect package manager (npm/pnpm/yarn/pip/go/cargo).
- get_path_info: Classify path type (cache/project/build-output/temp/downloads).

### Output
- mark_cleanable: Mark an item for cleanup. Use for EACH item you identify.

## Strategy
1. Start by scanning the root directory
2. Use subDirSummary to find the largest subdirectories
3. For promising directories, use get_path_info to classify them and check_package_manager to understand the project
4. Read package.json or config files with read_file when unsure if a directory is safe to clean
5. Use get_directory_size for accurate sizing before marking large directories
6. Mark cleanable items IMMEDIATELY as you find them — do not batch
7. Use grep_content to search for patterns like "cache", "temp", "log" when exploring
8. Use get_large_files (>50MB) and get_old_files (>90 days) for thorough analysis
9. Use analyze_disk_usage to understand what's taking up space

## What to mark as cleanable
Mark these as "safe":
- Cache directories (.cache, __pycache__, .npm, .yarn, .gradle/caches, .m2, .cargo, .pnpm-store, .conda)
- Build output (dist, build, target, .next, .nuxt, .output, .svelte-kit, out)
- Package dependencies (node_modules — fully reinstallable via npm/pnpm/yarn)
- Temp files (tmp, temp, .tmp, *.tmp, *.temp)
- Log files over 1MB
- Thumbnail caches (.thumbnails)

Mark these as "caution":
- Old large files in Downloads (>30 days, >100MB)
- Duplicate files (verify with calculate_hash for same-size files)
- Old media files not accessed in 180+ days

## Rules
- NEVER mark system directories (Windows, System32, /usr, /bin, /etc, /lib)
- NEVER mark user document folders (Documents, Desktop)
- NEVER mark directories containing .git as cleanable
- Only mark items as "safe" or "caution"
- Skip directories you can't access
- Always provide a clear reason in the reason field
- When in doubt, use read_file to inspect before marking

## Categories
- cache: Cached data that can be regenerated
- temp: Temporary files
- log: Log files
- build: Build artifacts that can be rebuilt
- download: Old/large downloads
- media: Large media files
- other: Items that don't fit other categories`
