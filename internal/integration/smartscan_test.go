//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

// Integration test for the Smart Scan agent.
//
// Config via env vars:
//
//	LLM_PROVIDER  (default: openai)
//	LLM_ENDPOINT  (required)
//	LLM_API_KEY   (required)
//	LLM_MODEL     (default: gpt-4o)
//
// Run:
//
//	go test -tags integration ./internal/integration/ -v -timeout 5m

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

// createTestFixtures builds a temp directory with known cleanable and safe items.
//
//	tmpDir/
//	├── project/
//	│   ├── node_modules/lodash/index.js  (2MB, cache — safe)
//	│   ├── dist/bundle.js                (1MB, build — safe)
//	│   ├── .cache/data.json              (512KB, cache — safe)
//	│   └── src/main.ts                   (1KB, source — do NOT mark)
//	├── logs/app.log                       (5MB, log — safe)
//	├── temp/download.tmp                  (3MB, temp — safe)
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

type markedItem struct {
	Path      string
	Name      string
	Size      int64
	IsDir     bool
	RiskLevel string
	Reason    string
	Category  string
}

// TestAgentConnection verifies the model supports function calling.
func TestAgentConnection(t *testing.T) {
	cfg := loadTestConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Build chat model directly
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: cfg.Endpoint,
		APIKey:  cfg.APIKey,
		Model:   cfg.ModelName,
	})
	if err != nil {
		t.Fatalf("create model: %v", err)
	}

	// Test with a tool call
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

	hasToolCall := len(resp.ToolCalls) > 0
	t.Logf("response has tool calls: %v (content: %q)", hasToolCall, resp.Content)

	if !hasToolCall {
		t.Error("model does NOT support function calling — agent will not work")
	}
}

// TestSmartScanAgent runs the full agent pipeline on test fixtures
// and verifies it finds cleanable items without false positives.
func TestSmartScanAgent(t *testing.T) {
	cfg := loadTestConfig(t)
	testDir := createTestFixtures(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Build chat model directly
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: cfg.Endpoint,
		APIKey:  cfg.APIKey,
		Model:   cfg.ModelName,
	})
	if err != nil {
		t.Fatalf("create model: %v", err)
	}

	t.Logf("model acquired: %s/%s", cfg.Provider, cfg.ModelName)

	// Scanner tools
	scan := scanner.New(ctx)
	scanTools, err := scanner.RegisterTools(scan)
	if err != nil {
		t.Fatalf("register tools: %v", err)
	}
	t.Logf("scanner tools: %d", len(scanTools))

	// mark_cleanable tool — collects results
	var mu sync.Mutex
	var marked []markedItem

	markTool, err := utils.InferTool("mark_cleanable",
		"Mark a file or directory as cleanable. Only mark items as 'safe' or 'caution'.",
		func(toolCtx context.Context, input *struct {
			Path      string `json:"path" jsonschema:"description=Full path"`
			Name      string `json:"name" jsonschema:"description=Name"`
			Size      int64  `json:"size" jsonschema:"description=Size in bytes"`
			IsDir     bool   `json:"isDir" jsonschema:"description=Is directory"`
			RiskLevel string `json:"riskLevel" jsonschema:"description=Risk,enum=safe,enum=caution"`
			Reason    string `json:"reason" jsonschema:"description=Why cleanable"`
			Category  string `json:"category" jsonschema:"description=Category,enum=cache,enum=temp,enum=log,enum=build,enum=download,enum=media,enum=other"`
		}) (*struct{ Success bool }, error) {
			mu.Lock()
			marked = append(marked, markedItem{
				Path: input.Path, Name: input.Name, Size: input.Size,
				IsDir: input.IsDir, RiskLevel: input.RiskLevel,
				Reason: input.Reason, Category: input.Category,
			})
			n := len(marked)
			mu.Unlock()
			t.Logf("[mark #%d] %s risk=%s cat=%s — %s", n, input.Name, input.RiskLevel, input.Category, input.Reason)
			return &struct{ Success bool }{Success: true}, nil
		})
	if err != nil {
		t.Fatalf("create mark tool: %v", err)
	}

	// Build agent
	tc := compose.ToolsNodeConfig{}
	for _, t := range append(scanTools, markTool) {
		tc.Tools = append(tc.Tools, t)
	}

	agent, err := react.NewAgent(ctx, &react.AgentConfig{
		Model:       chatModel,
		ToolsConfig: tc,
		MaxStep:     30,
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// Run
	stream, err := agent.Stream(ctx, []*schema.Message{
		schema.UserMessage(fmt.Sprintf(
			"Explore and clean: %s\n\nScan this directory, use subDirSummary to find large subdirectories, explore them, and mark cleanable items.",
			testDir,
		)),
	})
	if err != nil {
		t.Fatalf("start stream: %v", err)
	}
	defer stream.Close()

	steps := 0
	for {
		chunk, err := stream.Recv()
		if err != nil {
			t.Logf("stream ended after %d steps: %v", steps, err)
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
			steps++
			args := tc.Function.Arguments
			if len(args) > 200 {
				args = args[:200] + "..."
			}
			t.Logf("[step %d] %s(%s)", steps, tc.Function.Name, args)
		}
	}

	mu.Lock()
	items := make([]markedItem, len(marked))
	copy(items, marked)
	mu.Unlock()

	// Report
	t.Log("=== RESULTS ===")
	t.Logf("steps=%d items=%d", steps, len(items))
	for i, it := range items {
		t.Logf("  [%d] %-30s  risk=%-8s cat=%-8s  %s", i+1, it.Name, it.RiskLevel, it.Category, it.Reason)
	}

	// Assertions
	if steps == 0 {
		t.Error("FAIL: 0 tool calls — model may not support function calling")
	}
	if len(items) == 0 {
		t.Error("FAIL: 0 items marked — expected cache/build/temp/log items to be found")
	}

	// False positives — important files must NOT be marked
	for _, it := range items {
		switch {
		case filepath.Base(it.Path) == "report.pdf":
			t.Errorf("FALSE POSITIVE: marked user file: %s", it.Path)
		case filepath.Base(it.Path) == "main.ts":
			t.Errorf("FALSE POSITIVE: marked source file: %s", it.Path)
		case contains(it.Path, ".git"):
			t.Errorf("FALSE POSITIVE: marked .git: %s", it.Path)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
