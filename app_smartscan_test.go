package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"dfcleaner/internal/scanner"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
)

// --- Tracer bullet: pure function ---

func TestToolDisplayName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"scan_directory", "Scanning directory..."},
		{"get_file_info", "Inspecting file..."},
		{"get_large_files", "Finding large files..."},
		{"get_old_files", "Finding old files..."},
		{"find_duplicates", "Finding duplicates..."},
		{"mark_cleanable", "Marking cleanable..."},
		{"unknown_tool", "unknown_tool"},
	}
	for _, tc := range cases {
		got := toolDisplayName(tc.input)
		if got != tc.want {
			t.Errorf("toolDisplayName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- Mock ChatModel ---

type mockChatModel struct {
	responses []*schema.Message
	callCount int
}

func (m *mockChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if m.callCount >= len(m.responses) {
		return schema.AssistantMessage("Done.", nil), nil
	}
	resp := m.responses[m.callCount]
	m.callCount++
	return resp, nil
}

func (m *mockChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("not implemented in test")
}

func (m *mockChatModel) BindTools(tools []*schema.ToolInfo) error {
	return nil
}

// --- Integration: Agent marks cleanable items ---

func TestSmartScanAgentMarksCleanableItem(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".cache"), 0755)
	os.WriteFile(filepath.Join(dir, ".cache", "data.tmp"), []byte("cached"), 0644)

	var mu sync.Mutex
	var markedItems []markCleanableInput

	markTool, err := utils.InferTool("mark_cleanable",
		"Mark a file or directory as cleanable.",
		func(ctx context.Context, input *markCleanableInput) (*markCleanableOutput, error) {
			mu.Lock()
			markedItems = append(markedItems, *input)
			mu.Unlock()
			return &markCleanableOutput{Success: true}, nil
		})
	if err != nil {
		t.Fatal(err)
	}

	s := scanner.New(context.Background())
	scanTools, err := scanner.RegisterTools(s)
	if err != nil {
		t.Fatal(err)
	}

	mockModel := &mockChatModel{
		responses: []*schema.Message{
			// Round 1: Agent decides to scan the root directory
			schema.AssistantMessage("", []schema.ToolCall{{
				ID: "tc1",
				Function: schema.FunctionCall{
					Name:      "scan_directory",
					Arguments: fmt.Sprintf(`{"path": "%s", "maxDepth": 1}`, dir),
				},
			}}),
			// Round 2: Agent marks the cache as cleanable
			schema.AssistantMessage("", []schema.ToolCall{{
				ID: "tc2",
				Function: schema.FunctionCall{
					Name: "mark_cleanable",
					Arguments: fmt.Sprintf(`{
						"path": "%s/.cache",
						"name": ".cache",
						"size": 4096,
						"isDir": true,
						"riskLevel": "safe",
						"reason": "Cache directory, safe to clean",
						"category": "cache"
					}`, dir),
				},
			}}),
			// Round 3: Agent finishes
			schema.AssistantMessage("Finished exploring. Found 1 cleanable item.", nil),
		},
	}

	toolsConfig := compose.ToolsNodeConfig{}
	for _, t := range append(scanTools, markTool) {
		toolsConfig.Tools = append(toolsConfig.Tools, t)
	}

	agent, err := react.NewAgent(context.Background(), &react.AgentConfig{
		Model:       mockModel,
		ToolsConfig: toolsConfig,
		MaxStep:     10,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	resp, err := agent.Generate(context.Background(), []*schema.Message{
		schema.UserMessage(fmt.Sprintf("Explore: %s", dir)),
	})
	if err != nil {
		t.Fatalf("Agent error: %v", err)
	}

	if resp == nil || resp.Content == "" {
		t.Error("Expected non-empty response from agent")
	}

	mu.Lock()
	items := len(markedItems)
	mu.Unlock()

	if items != 1 {
		t.Fatalf("Expected 1 marked item, got %d", items)
	}

	if markedItems[0].RiskLevel != "safe" {
		t.Errorf("RiskLevel = %q, want 'safe'", markedItems[0].RiskLevel)
	}
	if markedItems[0].Category != "cache" {
		t.Errorf("Category = %q, want 'cache'", markedItems[0].Category)
	}
	if !markedItems[0].IsDir {
		t.Error("IsDir should be true")
	}
}

// --- Integration: Agent rejects dangerous risk level ---

func TestSmartScanAgentRejectsDangerousMark(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "important"), 0755)

	var mu sync.Mutex
	var markedItems []markCleanableInput

	markTool, err := utils.InferTool("mark_cleanable",
		"Mark a file or directory as cleanable.",
		func(ctx context.Context, input *markCleanableInput) (*markCleanableOutput, error) {
			if input.RiskLevel != "safe" && input.RiskLevel != "caution" {
				return &markCleanableOutput{Success: false}, nil
			}
			mu.Lock()
			markedItems = append(markedItems, *input)
			mu.Unlock()
			return &markCleanableOutput{Success: true}, nil
		})
	if err != nil {
		t.Fatal(err)
	}

	s := scanner.New(context.Background())
	scanTools, err := scanner.RegisterTools(s)
	if err != nil {
		t.Fatal(err)
	}

	mockModel := &mockChatModel{
		responses: []*schema.Message{
			schema.AssistantMessage("", []schema.ToolCall{{
				ID: "tc1",
				Function: schema.FunctionCall{
					Name:      "scan_directory",
					Arguments: fmt.Sprintf(`{"path": "%s"}`, dir),
				},
			}}),
			// Agent tries to mark as dangerous — should be rejected
			schema.AssistantMessage("", []schema.ToolCall{{
				ID: "tc2",
				Function: schema.FunctionCall{
					Name: "mark_cleanable",
					Arguments: fmt.Sprintf(`{
						"path": "%s/important",
						"name": "important",
						"size": 4096,
						"isDir": true,
						"riskLevel": "dangerous",
						"reason": "Important directory",
						"category": "other"
					}`, dir),
				},
			}}),
			schema.AssistantMessage("Finished.", nil),
		},
	}

	toolsConfig := compose.ToolsNodeConfig{}
	for _, t := range append(scanTools, markTool) {
		toolsConfig.Tools = append(toolsConfig.Tools, t)
	}

	agent, err := react.NewAgent(context.Background(), &react.AgentConfig{
		Model:       mockModel,
		ToolsConfig: toolsConfig,
		MaxStep:     10,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	_, err = agent.Generate(context.Background(), []*schema.Message{
		schema.UserMessage(fmt.Sprintf("Explore: %s", dir)),
	})
	if err != nil {
		t.Fatalf("Agent error: %v", err)
	}

	mu.Lock()
	items := len(markedItems)
	mu.Unlock()

	if items != 0 {
		t.Errorf("Expected 0 marked items (dangerous rejected), got %d", items)
	}
}
