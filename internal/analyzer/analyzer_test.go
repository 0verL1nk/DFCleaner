package analyzer

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"dfcleaner/internal/config"
	"dfcleaner/internal/llm"
	"dfcleaner/internal/scanner"
	"dfcleaner/internal/store"

	"github.com/adrg/xdg"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// mockChatModel returns a fixed response for testing the agent pipeline.
type mockChatModel struct {
	response string
	err      error
}

func (m *mockChatModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &schema.Message{Content: m.response}, nil
}

func (m *mockChatModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockChatModel) BindTools(_ []*schema.ToolInfo) error { return nil }

// newMockAgent creates a react.Agent backed by a mock model that returns the given response.
func newMockAgent(t *testing.T, response string) *react.Agent {
	t.Helper()
	agent, err := react.NewAgent(context.Background(), &react.AgentConfig{
		Model:       &mockChatModel{response: response},
		ToolsConfig: compose.ToolsNodeConfig{},
		MaxStep:     1,
	})
	if err != nil {
		t.Fatalf("create mock agent: %v", err)
	}
	return agent
}

// newErrorMockAgent creates a react.Agent backed by a mock model that always errors.
func newErrorMockAgent(t *testing.T) *react.Agent {
	t.Helper()
	agent, err := react.NewAgent(context.Background(), &react.AgentConfig{
		Model:       &mockChatModel{err: fmt.Errorf("mock generate error")},
		ToolsConfig: compose.ToolsNodeConfig{},
		MaxStep:     1,
	})
	if err != nil {
		t.Fatalf("create error mock agent: %v", err)
	}
	return agent
}

func TestNew(t *testing.T) {
	ctx := context.Background()
	l := llm.New(nil)
	s := scanner.New(ctx)

	a := New(ctx, l, s, nil)
	if a == nil {
		t.Fatal("New returned nil")
	}
	if a.ctx != ctx {
		t.Error("context not set correctly")
	}
	if a.llm != l {
		t.Error("llm provider not set correctly")
	}
	if a.agent != nil {
		t.Error("agent should be nil on creation")
	}
}

func TestNew_WithStore(t *testing.T) {
	st := newTestDB(t)

	ctx := context.Background()
	a := New(ctx, nil, nil, st)
	if a.store != st {
		t.Error("store not set correctly")
	}
}

func TestCancel_NoCancelFunc(t *testing.T) {
	a := &Analyzer{}
	// Should not panic when cancel is nil
	a.Cancel()
}

func TestCancel_WithCancelFunc(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := &Analyzer{
		ctx:    ctx,
		cancel: cancel,
	}
	// Should not panic
	a.Cancel()

	// Verify context is cancelled
	select {
	case <-ctx.Done():
		// expected
	default:
		t.Error("context should be cancelled after Cancel()")
	}
}

func TestBuildSystemPrompt_Basic(t *testing.T) {
	a := &Analyzer{}
	prompt := a.buildSystemPrompt()

	if !strings.Contains(prompt, "DFCleaner") {
		t.Error("prompt should mention DFCleaner")
	}
	if !strings.Contains(prompt, "Available Tools") {
		t.Error("prompt should list available tools")
	}
	if !strings.Contains(prompt, "scan_directory") {
		t.Error("prompt should mention scan_directory tool")
	}
	if !strings.Contains(prompt, "get_file_info") {
		t.Error("prompt should mention get_file_info tool")
	}
	if !strings.Contains(prompt, "get_large_files") {
		t.Error("prompt should mention get_large_files tool")
	}
	if !strings.Contains(prompt, "get_old_files") {
		t.Error("prompt should mention get_old_files tool")
	}
	if !strings.Contains(prompt, "find_duplicates") {
		t.Error("prompt should mention find_duplicates tool")
	}
	if !strings.Contains(prompt, "Risk Levels") {
		t.Error("prompt should contain risk levels")
	}
	if !strings.Contains(prompt, "safe") {
		t.Error("prompt should mention 'safe' risk level")
	}
	if !strings.Contains(prompt, "caution") {
		t.Error("prompt should mention 'caution' risk level")
	}
	if !strings.Contains(prompt, "dangerous") {
		t.Error("prompt should mention 'dangerous' risk level")
	}
	if !strings.Contains(prompt, "Output Format") {
		t.Error("prompt should contain output format")
	}
	if !strings.Contains(prompt, "risk_level") {
		t.Error("prompt should mention risk_level in output format")
	}
	if !strings.Contains(prompt, "confidence") {
		t.Error("prompt should mention confidence")
	}
}

func TestBuildSystemPrompt_PlatformInfo(t *testing.T) {
	a := &Analyzer{}
	prompt := a.buildSystemPrompt()

	if !strings.Contains(prompt, "Platform Info") {
		t.Error("prompt should contain platform info section")
	}
	if !strings.Contains(prompt, runtime.GOOS) {
		t.Error("prompt should mention current OS")
	}
	if !strings.Contains(prompt, runtime.GOARCH) {
		t.Error("prompt should mention current architecture")
	}
	if !strings.Contains(prompt, "~/") {
		t.Error("prompt should mention home directory shorthand")
	}
}

func TestBuildSystemPrompt_PlatformSpecific(t *testing.T) {
	a := &Analyzer{}
	prompt := a.buildSystemPrompt()

	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(prompt, ".DS_Store") {
			t.Error("macOS prompt should mention .DS_Store")
		}
		if !strings.Contains(prompt, "Library/Caches") {
			t.Error("macOS prompt should mention Library/Caches")
		}
	case "linux":
		if !strings.Contains(prompt, "~/.cache") {
			t.Error("Linux prompt should mention ~/.cache")
		}
		if !strings.Contains(prompt, "~/.local/share/Trash") {
			t.Error("Linux prompt should mention ~/.local/share/Trash")
		}
	case "windows":
		if !strings.Contains(prompt, "Thumbs.db") {
			t.Error("Windows prompt should mention Thumbs.db")
		}
		if !strings.Contains(prompt, "%TEMP%") {
			t.Logf("Windows prompt should mention %%TEMP%%")
			t.Fail()
		}
	}
}

func newTestDB(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(dir+"/test.db"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&store.UserSetting{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return store.New(db)
}

func TestBuildSystemPrompt_WithCustomInstructions(t *testing.T) {
	st := newTestDB(t)

	if err := st.SetSetting("custom_ai_instruction", "Always respond in Chinese"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}

	a := &Analyzer{store: st}
	prompt := a.buildSystemPrompt()

	if !strings.Contains(prompt, "User Instructions") {
		t.Error("prompt should contain User Instructions section")
	}
	if !strings.Contains(prompt, "Always respond in Chinese") {
		t.Error("prompt should contain custom instruction text")
	}
}

func TestBuildSystemPrompt_EmptyCustomInstructions(t *testing.T) {
	st := newTestDB(t)

	a := &Analyzer{store: st}
	prompt := a.buildSystemPrompt()

	if strings.Contains(prompt, "User Instructions") {
		t.Error("prompt should not contain User Instructions section when empty")
	}
}

func TestBuildSystemPrompt_NilStore(t *testing.T) {
	a := &Analyzer{store: nil}
	prompt := a.buildSystemPrompt()

	if strings.Contains(prompt, "User Instructions") {
		t.Error("prompt should not contain User Instructions with nil store")
	}
}

func TestBuildSystemPrompt_ContainsCategories(t *testing.T) {
	a := &Analyzer{}
	prompt := a.buildSystemPrompt()

	categories := []string{"cache", "temp", "log", "config", "document", "media"}
	for _, cat := range categories {
		if !strings.Contains(prompt, cat) {
			t.Errorf("prompt should mention category %q", cat)
		}
	}
}

func TestBatchAnalyze_Empty(t *testing.T) {
	a := &Analyzer{ctx: context.Background()}
	result, err := a.BatchAnalyze(context.Background(), nil)
	if err != nil {
		t.Fatalf("BatchAnalyze nil: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for nil input, got %v", result)
	}

	result, err = a.BatchAnalyze(context.Background(), []scanner.FileEntry{})
	if err != nil {
		t.Fatalf("BatchAnalyze empty: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for empty input, got %v", result)
	}
}

func TestBatchAnalyze_AgentCreationFails(t *testing.T) {
	// Use a real LLM provider with no active config, so getOrCreateAgent
	// returns a proper error instead of nil-pointer panic.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	xdg.Reload()
	mgr, err := config.NewManager()
	if err != nil {
		t.Fatalf("config.NewManager: %v", err)
	}
	l := llm.New(mgr)

	a := &Analyzer{ctx: context.Background(), llm: l}

	entries := []scanner.FileEntry{
		{Path: "/tmp/test", Name: "test", Size: 100},
	}

	_, err = a.BatchAnalyze(context.Background(), entries)
	if err == nil {
		t.Fatal("expected error when agent creation fails")
	}
	if !strings.Contains(err.Error(), "get active LLM model") {
		t.Errorf("error should mention LLM model, got: %v", err)
	}
}

func TestGetOrCreateAgent_Cached(t *testing.T) {
	// When agent is already set, getOrCreateAgent should return it immediately.
	mockAgent := newMockAgent(t, "test")
	a := &Analyzer{
		ctx:   context.Background(),
		agent: mockAgent,
	}

	agent, err := a.getOrCreateAgent()
	if err != nil {
		t.Fatalf("getOrCreateAgent: %v", err)
	}
	if agent != mockAgent {
		t.Error("should return cached agent")
	}
}

func TestBatchAnalyze_WithAgent(t *testing.T) {
	jsonResponse := `[{"path":"/tmp/cache/a.log","risk_level":"safe","reason":"Log file","category":"log","confidence":0.9},{"path":"/tmp/config/settings.json","risk_level":"dangerous","reason":"Config","category":"config","confidence":0.95}]`

	mockAgent := newMockAgent(t, jsonResponse)
	a := &Analyzer{
		ctx:   context.Background(),
		agent: mockAgent,
	}

	entries := []scanner.FileEntry{
		{Path: "/tmp/cache/a.log", Name: "a.log", Size: 1024, Extension: "log", IsDir: false, ModTime: time.Now()},
		{Path: "/tmp/config/settings.json", Name: "settings.json", Size: 512, Extension: "json", IsDir: false, ModTime: time.Now()},
	}

	results, err := a.BatchAnalyze(context.Background(), entries)
	if err != nil {
		t.Fatalf("BatchAnalyze: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].RiskLevel != "safe" {
		t.Errorf("results[0].RiskLevel = %q, want 'safe'", results[0].RiskLevel)
	}
	if results[1].RiskLevel != "dangerous" {
		t.Errorf("results[1].RiskLevel = %q, want 'dangerous'", results[1].RiskLevel)
	}
}

func TestBatchAnalyze_WithAccessTime(t *testing.T) {
	jsonResponse := `[{"path":"/tmp/old.dat","risk_level":"caution","reason":"Old file","category":"temp","confidence":0.7}]`

	mockAgent := newMockAgent(t, jsonResponse)
	a := &Analyzer{
		ctx:   context.Background(),
		agent: mockAgent,
	}

	entries := []scanner.FileEntry{
		{
			Path:       "/tmp/old.dat",
			Name:       "old.dat",
			Size:       2048,
			IsDir:      false,
			ModTime:    time.Now().AddDate(0, -6, 0),
			AccessTime: time.Now().AddDate(0, -3, 0),
		},
	}

	results, err := a.BatchAnalyze(context.Background(), entries)
	if err != nil {
		t.Fatalf("BatchAnalyze: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].RiskLevel != "caution" {
		t.Errorf("RiskLevel = %q, want 'caution'", results[0].RiskLevel)
	}
}

func TestBatchAnalyze_AgentGenerateError(t *testing.T) {
	// Create an agent with a model that returns unparseable JSON,
	// which triggers the parseAnalysisResponse error path.
	mockAgent := newMockAgent(t, "not json")
	a := &Analyzer{
		ctx:   context.Background(),
		agent: mockAgent,
	}

	entries := []scanner.FileEntry{
		{Path: "/tmp/a", Name: "a", Size: 100},
		{Path: "/tmp/b", Name: "b", Size: 200},
	}

	results, err := a.BatchAnalyze(context.Background(), entries)
	if err != nil {
		t.Fatalf("BatchAnalyze: %v", err)
	}
	for _, r := range results {
		if r.Error != "analysis_error" {
			t.Errorf("expected analysis_error, got %q", r.Error)
		}
	}
}

func TestBatchAnalyze_MissingPaths(t *testing.T) {
	// LLM returns results for only some entries
	jsonResponse := `[{"path":"/tmp/a","risk_level":"safe","reason":"OK","category":"temp","confidence":0.8}]`

	mockAgent := newMockAgent(t, jsonResponse)
	a := &Analyzer{
		ctx:   context.Background(),
		agent: mockAgent,
	}

	entries := []scanner.FileEntry{
		{Path: "/tmp/a", Name: "a", Size: 100},
		{Path: "/tmp/b", Name: "b", Size: 200},
	}

	results, err := a.BatchAnalyze(context.Background(), entries)
	if err != nil {
		t.Fatalf("BatchAnalyze: %v", err)
	}
	if results[0].RiskLevel != "safe" {
		t.Errorf("results[0] RiskLevel = %q, want 'safe'", results[0].RiskLevel)
	}
	if results[1].Error != "analysis_error" {
		t.Errorf("results[1] should have analysis_error for missing path, got %q", results[1].Error)
	}
}

func TestGetOrCreateAgent_FullCreation(t *testing.T) {
	// Create a real LLM provider with saved config
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	xdg.Reload()
	mgr, err := config.NewManager()
	if err != nil {
		t.Fatalf("config.NewManager: %v", err)
	}
	l := llm.New(mgr)

	err = l.SaveConfig(&llm.LLMConfig{
		Provider:  "openai",
		Endpoint:  "https://invalid.example.com/v1",
		APIKey:    "test-key",
		ModelName: "gpt-4o",
		IsActive:  true,
	})
	if err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	a := &Analyzer{
		ctx: context.Background(),
		llm: l,
	}

	agent, err := a.getOrCreateAgent()
	if err != nil {
		t.Fatalf("getOrCreateAgent: %v", err)
	}
	if agent == nil {
		t.Fatal("agent should not be nil")
	}

	// Second call should return cached agent
	agent2, err := a.getOrCreateAgent()
	if err != nil {
		t.Fatalf("getOrCreateAgent cached: %v", err)
	}
	if agent != agent2 {
		t.Error("should return same cached agent")
	}
}

func TestBatchAnalyze_AgentGenerateFails(t *testing.T) {
	errAgent := newErrorMockAgent(t)
	a := &Analyzer{
		ctx:   context.Background(),
		agent: errAgent,
	}

	entries := []scanner.FileEntry{
		{Path: "/tmp/test", Name: "test", Size: 100},
	}

	_, err := a.BatchAnalyze(context.Background(), entries)
	if err == nil {
		t.Fatal("expected error when agent.Generate fails")
	}
	if !strings.Contains(err.Error(), "LLM generate") {
		t.Errorf("error should mention LLM generate, got: %v", err)
	}
}
