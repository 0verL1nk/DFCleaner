//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"dfcleaner/internal/scanner"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// Integration tests for the Smart Scan agent.
//
// Config via env vars:
//
//	LLM_ENDPOINT  (required)
//	LLM_API_KEY   (required)
//	LLM_MODEL     (default: gpt-4o)
//
// Run:
//
//	go test -tags integration ./internal/integration/ -v -timeout 15m

// --- Config & helpers ---

type testLLMConfig struct {
	Provider  string
	Endpoint  string
	APIKey    string
	ModelName string
}

func loadTestConfig(t *testing.T) testLLMConfig {
	t.Helper()
	cfg := testLLMConfig{
		Provider:  envOr("LLM_PROVIDER", "openai"),
		Endpoint:  os.Getenv("LLM_ENDPOINT"),
		APIKey:    os.Getenv("LLM_API_KEY"),
		ModelName: envOr("LLM_MODEL", "gpt-4o"),
	}
	if cfg.Endpoint == "" || cfg.APIKey == "" {
		t.Skip("Set LLM_ENDPOINT and LLM_API_KEY to run integration tests")
	}
	t.Logf("config: provider=%s model=%s endpoint=%s", cfg.Provider, cfg.ModelName, cfg.Endpoint)
	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func newTestChatModel(ctx context.Context, cfg testLLMConfig, extraOpts ...func(*openai.ChatModelConfig)) (*openai.ChatModel, error) {
	maxTokens := 16384
	modelCfg := &openai.ChatModelConfig{
		BaseURL:   cfg.Endpoint,
		APIKey:    cfg.APIKey,
		Model:     cfg.ModelName,
		MaxTokens: &maxTokens,
	}
	for _, opt := range extraOpts {
		opt(modelCfg)
	}
	return openai.NewChatModel(ctx, modelCfg)
}

// --- Test fixtures ---

type markedItem struct {
	Path      string
	Name      string
	Size      int64
	IsDir     bool
	RiskLevel string
	Reason    string
	Category  string
}

// createTestFixtures builds a temp directory with known cleanable and safe items.
//
//	tmpDir/
//	├── project/
//	│   ├── node_modules/lodash/index.js  (2MB, cache — safe to clean)
//	│   ├── dist/bundle.js                (1MB, build — safe to clean)
//	│   ├── .cache/data.json              (512KB, cache — safe to clean)
//	│   └── src/main.ts                   (1KB, source — do NOT mark)
//	├── logs/app.log                       (5MB, log — safe to clean)
//	├── temp/download.tmp                  (3MB, temp — safe to clean)
//	├── important/report.pdf              (10MB, user file — do NOT mark)
//	└── .git/HEAD                          (git repo — do NOT mark)
func createTestFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	mkfile := func(path string, size int) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		data := make([]byte, size)
		for i := range data {
			data[i] = byte('A' + i%26)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	mkfile(filepath.Join(dir, "project", "node_modules", "lodash", "index.js"), 2*1024*1024)
	mkfile(filepath.Join(dir, "project", "dist", "bundle.js"), 1024*1024)
	mkfile(filepath.Join(dir, "project", ".cache", "data.json"), 512*1024)
	mkfile(filepath.Join(dir, "project", "src", "main.ts"), 1024)
	mkfile(filepath.Join(dir, "logs", "app.log"), 5*1024*1024)
	mkfile(filepath.Join(dir, "temp", "download.tmp"), 3*1024*1024)
	mkfile(filepath.Join(dir, "important", "report.pdf"), 10*1024*1024)
	mkfile(filepath.Join(dir, ".git", "HEAD"), 50)

	return dir
}

// buildAgent creates a ReAct agent with scanner + mark_cleanable tools and system prompt.
func buildAgent(t *testing.T, ctx context.Context, chatModel model.ChatModel, markMu *sync.Mutex, marked *[]markedItem) *react.Agent {
	t.Helper()

	scan := scanner.New(ctx)
	scanTools, err := scanner.RegisterTools(scan)
	if err != nil {
		t.Fatalf("register tools: %v", err)
	}
	t.Logf("scanner tools: %d", len(scanTools))

	markTool, err := utils.InferTool("mark_cleanable",
		"Mark a file or directory as cleanable. Call this for each item you determine should be cleaned. Only mark items as 'safe' or 'caution'.",
		func(toolCtx context.Context, input *struct {
			Path      string `json:"path" jsonschema:"description=Full path of the file or directory"`
			Name      string `json:"name" jsonschema:"description=Name of the file or directory"`
			Size      int64  `json:"size" jsonschema:"description=Size in bytes"`
			IsDir     bool   `json:"isDir" jsonschema:"description=Whether this is a directory"`
			RiskLevel string `json:"riskLevel" jsonschema:"description=Risk level,enum=safe,enum=caution"`
			Reason    string `json:"reason" jsonschema:"description=Why this item can be cleaned"`
			Category  string `json:"category" jsonschema:"description=Category,enum=cache,enum=temp,enum=log,enum=build,enum=download,enum=media,enum=other"`
		}) (*struct{ Success bool }, error) {
			markMu.Lock()
			*marked = append(*marked, markedItem{
				Path: input.Path, Name: input.Name, Size: input.Size,
				IsDir: input.IsDir, RiskLevel: input.RiskLevel,
				Reason: input.Reason, Category: input.Category,
			})
			n := len(*marked)
			markMu.Unlock()
			t.Logf("[mark #%d] %s risk=%s cat=%s — %s", n, input.Name, input.RiskLevel, input.Category, input.Reason)
			return &struct{ Success bool }{Success: true}, nil
		})
	if err != nil {
		t.Fatalf("create mark tool: %v", err)
	}

	tc := compose.ToolsNodeConfig{}
	for _, tool := range append(scanTools, markTool) {
		tc.Tools = append(tc.Tools, tool)
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		Model:           chatModel,
		ToolsConfig:     tc,
		MaxStep:         30,
		MessageModifier: react.NewPersonaModifier(smartScanSystemPrompt),
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	return agent
}

// --- Assertions ---

func assertNoFalsePositives(t *testing.T, items []markedItem) {
	t.Helper()
	for _, it := range items {
		switch {
		case filepath.Base(it.Path) == "report.pdf":
			t.Errorf("FALSE POSITIVE: marked user file: %s", it.Path)
		case filepath.Base(it.Path) == "main.ts":
			t.Errorf("FALSE POSITIVE: marked source file: %s", it.Path)
		case strings.Contains(it.Path, ".git"):
			t.Errorf("FALSE POSITIVE: marked .git: %s", it.Path)
		}
	}
}

// --- System prompt (matches production app_smartscan.go) ---

const smartScanSystemPrompt = `You are DFCleaner's autonomous disk cleanup agent. Your task is to explore directories and identify files/folders that can be safely cleaned up.

## Strategy
1. Start by scanning the root directory
2. Look at subDirSummary — pick the largest or most suspicious subdirectories to explore next
3. For each promising subdirectory, call scan_directory again to drill deeper
4. When you find cleanable items, call mark_cleanable IMMEDIATELY
5. Use your own judgment to decide what is safe to clean — consider whether the item can be regenerated, is a temporary artifact, or is otherwise disposable

## Guidelines
- Assess each item based on what it IS, not just its name
- Source code, user documents, and version control data should generally be preserved
- Use risk levels: "safe" (clearly disposable) or "caution" (user should review before cleaning)
- Always provide a clear reason in the reason field
- Categories: cache, temp, log, build, download, media, other`

// --- Tests ---

// TestAgentConnection verifies the model supports function calling.
func TestAgentConnection(t *testing.T) {
	cfg := loadTestConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	chatModel, err := newTestChatModel(ctx, cfg)
	if err != nil {
		t.Fatalf("create model: %v", err)
	}

	toolInfo := &schema.ToolInfo{
		Name: "test_tool",
		Desc: "A test tool",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"msg": {Type: schema.String, Desc: "test"},
		}),
	}
	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage("Use the provided tool."),
		schema.UserMessage("Say hello using the test tool."),
	}, model.WithTools([]*schema.ToolInfo{toolInfo}))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if len(resp.ToolCalls) == 0 {
		t.Error("model does NOT support function calling — agent will not work")
	} else {
		t.Logf("function calling verified: tool_calls=%d", len(resp.ToolCalls))
	}
}

// TestDirectModelWithScannerTools tests the model directly with all scanner tools
// (no ReAct agent) to verify tool compatibility.
func TestDirectModelWithScannerTools(t *testing.T) {
	cfg := loadTestConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	chatModel, err := newTestChatModel(ctx, cfg)
	if err != nil {
		t.Fatalf("create model: %v", err)
	}

	scan := scanner.New(ctx)
	scanTools, err := scanner.RegisterTools(scan)
	if err != nil {
		t.Fatalf("register tools: %v", err)
	}

	var toolInfos []*schema.ToolInfo
	for _, tool := range scanTools {
		info, err := tool.Info(ctx)
		if err != nil {
			t.Fatalf("tool info: %v", err)
		}
		toolInfos = append(toolInfos, info)
	}
	t.Logf("collected %d tool infos", len(toolInfos))

	// Test 1: Generate with tools via WithTools option
	t.Log("Test 1: Generate with scanner tools via model.WithTools...")
	resp, err := chatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("Scan the directory /tmp and tell me what you find."),
	}, model.WithTools(toolInfos))
	if err != nil {
		t.Errorf("Test 1 FAILED: %v", err)
	} else {
		t.Logf("Test 1 OK: toolCalls=%d", len(resp.ToolCalls))
		if len(resp.ToolCalls) == 0 {
			t.Error("Test 1: expected at least 1 tool call")
		}
	}

	// Test 2: BindTools first, then Generate
	t.Log("Test 2: BindTools then Generate...")
	chatModel2, err := newTestChatModel(ctx, cfg)
	if err != nil {
		t.Fatalf("create model2: %v", err)
	}
	if err := chatModel2.BindTools(toolInfos); err != nil {
		t.Fatalf("bind tools: %v", err)
	}
	resp2, err := chatModel2.Generate(ctx, []*schema.Message{
		schema.UserMessage("Scan /tmp and find cleanable items."),
	})
	if err != nil {
		t.Errorf("Test 2 FAILED: %v", err)
	} else {
		t.Logf("Test 2 OK: toolCalls=%d", len(resp2.ToolCalls))
		if len(resp2.ToolCalls) == 0 {
			t.Error("Test 2: expected at least 1 tool call")
		}
	}
}

// TestE2ESmartScanGenerate runs the full agent pipeline using Generate (non-streaming)
// and verifies it finds cleanable items without false positives.
func TestE2ESmartScanGenerate(t *testing.T) {
	cfg := loadTestConfig(t)
	testDir := createTestFixtures(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	chatModel, err := newTestChatModel(ctx, cfg)
	if err != nil {
		t.Fatalf("create model: %v", err)
	}

	var mu sync.Mutex
	var marked []markedItem

	agent := buildAgent(t, ctx, chatModel, &mu, &marked)

	t.Log("=== E2E: agent.Generate ===")
	resultMsg, err := agent.Generate(ctx, []*schema.Message{
		schema.UserMessage(fmt.Sprintf(
			"Explore and clean: %s\n\nScan this directory and mark cleanable items.",
			testDir,
		)),
	})
	if err != nil {
		t.Fatalf("agent generate: %v", err)
	}

	mu.Lock()
	items := make([]markedItem, len(marked))
	copy(items, marked)
	mu.Unlock()

	// Report
	t.Logf("result: content_len=%d toolCalls=%d items=%d",
		len(resultMsg.Content), len(resultMsg.ToolCalls), len(items))
	for i, it := range items {
		t.Logf("  [%d] %-30s  risk=%-8s cat=%-8s  %s", i+1, it.Name, it.RiskLevel, it.Category, it.Reason)
	}

	// Assertions
	if len(items) == 0 {
		t.Fatal("expected at least 1 cleanable item, got 0")
	}
	assertNoFalsePositives(t, items)
}

// TestE2ESmartScanStream runs the full agent pipeline using Stream.
func TestE2ESmartScanStream(t *testing.T) {
	cfg := loadTestConfig(t)
	testDir := createTestFixtures(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	chatModel, err := newTestChatModel(ctx, cfg)
	if err != nil {
		t.Fatalf("create model: %v", err)
	}

	var mu sync.Mutex
	var marked []markedItem

	agent := buildAgent(t, ctx, chatModel, &mu, &marked)

	t.Log("=== E2E: agent.Stream ===")
	stream, err := agent.Stream(ctx, []*schema.Message{
		schema.UserMessage(fmt.Sprintf(
			"Explore and clean: %s\n\nScan this directory and mark cleanable items.",
			testDir,
		)),
	})
	if err != nil {
		t.Fatalf("start stream: %v", err)
	}
	defer stream.Close()

	for {
		chunk, err := stream.Recv()
		if err != nil {
			t.Logf("stream ended: %v", err)
			break
		}

		if chunk.Content != "" {
			c := chunk.Content
			if len(c) > 300 {
				c = c[:300] + "..."
			}
			t.Logf("[think] %s", c)
		}

		for _, tc := range chunk.ToolCalls {
			if tc.Function.Name == "" {
				continue
			}
			args := tc.Function.Arguments
			if len(args) > 200 {
				args = args[:200] + "..."
			}
			t.Logf("[tool] %s(%s)", tc.Function.Name, args)
		}
	}

	mu.Lock()
	items := make([]markedItem, len(marked))
	copy(items, marked)
	mu.Unlock()

	t.Log("=== STREAM RESULTS ===")
	t.Logf("items=%d", len(items))
	for i, it := range items {
		t.Logf("  [%d] %-30s  risk=%-8s cat=%-8s  %s", i+1, it.Name, it.RiskLevel, it.Category, it.Reason)
	}

	if len(items) == 0 {
		t.Fatal("expected at least 1 cleanable item, got 0")
	}
	assertNoFalsePositives(t, items)
}

// TestE2ESmartScanPrecision verifies the agent's precision — it should find
// specific cleanable categories and NOT mark user files.
func TestE2ESmartScanPrecision(t *testing.T) {
	cfg := loadTestConfig(t)
	testDir := createTestFixtures(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	chatModel, err := newTestChatModel(ctx, cfg)
	if err != nil {
		t.Fatalf("create model: %v", err)
	}

	var mu sync.Mutex
	var marked []markedItem

	agent := buildAgent(t, ctx, chatModel, &mu, &marked)

	_, err = agent.Generate(ctx, []*schema.Message{
		schema.UserMessage(fmt.Sprintf(
			"Explore and clean: %s\n\nScan this directory and mark ALL cleanable items. Be thorough but precise.",
			testDir,
		)),
	})
	if err != nil {
		t.Fatalf("agent generate: %v", err)
	}

	mu.Lock()
	items := make([]markedItem, len(marked))
	copy(items, marked)
	mu.Unlock()

	t.Logf("precision test: %d items marked", len(items))
	for i, it := range items {
		t.Logf("  [%d] %-30s  risk=%-8s cat=%-8s", i+1, it.Name, it.RiskLevel, it.Category)
	}

	if len(items) == 0 {
		t.Fatal("expected cleanable items, got 0")
	}

	assertNoFalsePositives(t, items)

	// Verify categories
	categories := map[string]bool{}
	for _, it := range items {
		categories[it.Category] = true
	}
	foundCleanable := categories["cache"] || categories["temp"] || categories["log"] || categories["build"]
	if !foundCleanable {
		t.Error("expected at least one item in cache/temp/log/build categories")
	}

	// All risk levels must be safe or caution
	for _, it := range items {
		if it.RiskLevel != "safe" && it.RiskLevel != "caution" {
			t.Errorf("invalid risk level %q for %s", it.RiskLevel, it.Name)
		}
	}
}
