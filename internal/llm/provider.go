package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"dfcleaner/internal/store"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type Provider struct {
	store   *store.Store
	current model.ChatModel
}

func New(s *store.Store) *Provider {
	return &Provider{store: s}
}

func (p *Provider) TestConnection(ctx context.Context, config LLMConfig) (*ConnectionTestResult, error) {
	chatModel, err := p.createChatModel(ctx, config)
	if err != nil {
		return &ConnectionTestResult{
			Success: false,
			Error:   fmt.Sprintf("failed to create model: %v", err),
		}, nil
	}

	// Test with a function calling request
	toolInfo := &schema.ToolInfo{
		Name: "test_tool",
		Desc: "A test tool to verify function calling support",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"message": {Type: schema.String, Desc: "A test message"},
		}),
	}

	messages := []*schema.Message{
		schema.SystemMessage("You are a test assistant. Use the provided tool."),
		schema.UserMessage("Say hello using the test tool."),
	}

	resp, err := chatModel.Generate(ctx, messages,
		model.WithTools([]*schema.ToolInfo{toolInfo}),
	)
	if err != nil {
		return &ConnectionTestResult{
			Success: false,
			Error:   fmt.Sprintf("API call failed: %v", err),
		}, nil
	}

	// Check if response contains tool calls
	hasToolCall := len(resp.ToolCalls) > 0
	if !hasToolCall {
		return &ConnectionTestResult{
			Success:           true,
			NoFunctionCalling: true,
			ModelInfo:         config.ModelName,
		}, nil
	}

	return &ConnectionTestResult{
		Success:   true,
		ModelInfo: config.ModelName,
	}, nil
}

func (p *Provider) GetActiveModel(ctx context.Context) (model.ChatModel, error) {
	if p.current != nil {
		return p.current, nil
	}

	cfg := p.store.GetActiveLLMConfig()
	if cfg == nil {
		return nil, fmt.Errorf("no active LLM configuration")
	}

	apiKey, err := decryptAPIKey(cfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt API key: %w", err)
	}

	llmCfg := LLMConfig{
		Provider:  cfg.Provider,
		Endpoint:  cfg.Endpoint,
		APIKey:    apiKey,
		ModelName: cfg.ModelName,
	}

	chatModel, err := p.createChatModel(ctx, llmCfg)
	if err != nil {
		return nil, err
	}

	p.current = chatModel
	return p.current, nil
}

func (p *Provider) InvalidateCache() {
	p.current = nil
}

func (p *Provider) createChatModel(ctx context.Context, config LLMConfig) (model.ChatModel, error) {
	switch config.Provider {
	case "openai", "deepseek", "openrouter", "qianfan":
		return p.createOpenAICompatible(ctx, config)
	case "claude":
		return p.createClaude(ctx, config)
	case "ollama":
		return p.createOllama(ctx, config)
	default:
		// Default: try OpenAI compatible
		return p.createOpenAICompatible(ctx, config)
	}
}

func (p *Provider) createOpenAICompatible(ctx context.Context, config LLMConfig) (model.ChatModel, error) {
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: config.Endpoint,
		APIKey:  config.APIKey,
		Model:   config.ModelName,
	})
}

func (p *Provider) createClaude(ctx context.Context, config LLMConfig) (model.ChatModel, error) {
	// eino-ext/components/model/claude
	// Import dynamically to avoid hard dependency
	// For now, fall back to OpenAI compatible with Claude endpoint
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: config.Endpoint,
		APIKey:  config.APIKey,
		Model:   config.ModelName,
	})
}

func (p *Provider) createOllama(ctx context.Context, config LLMConfig) (model.ChatModel, error) {
	return openai.NewChatModel(ctx, &openai.ChatModelConfig{
		BaseURL: config.Endpoint,
		APIKey:  "ollama", // Ollama doesn't need a real key
		Model:   config.ModelName,
	})
}

// GetActiveConfig returns the decrypted active LLM config for app.go binding.
func (p *Provider) GetActiveConfig() (*LLMConfig, error) {
	cfg := p.store.GetActiveLLMConfig()
	if cfg == nil {
		return nil, nil
	}

	apiKey, err := decryptAPIKey(cfg.APIKey)
	if err != nil {
		return nil, err
	}

	return &LLMConfig{
		Provider:  cfg.Provider,
		Endpoint:  cfg.Endpoint,
		APIKey:    apiKey,
		ModelName: cfg.ModelName,
		IsActive:  cfg.IsActive,
	}, nil
}

// SaveConfig encrypts and saves an LLM config.
func (p *Provider) SaveConfig(cfg *LLMConfig) error {
	encrypted, err := encryptAPIKey(cfg.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt API key: %w", err)
	}

	dbCfg := &store.LLMConfig{
		Provider:  cfg.Provider,
		Endpoint:  cfg.Endpoint,
		APIKey:    encrypted,
		ModelName: cfg.ModelName,
		IsActive:  cfg.IsActive,
	}

	if err := p.store.SaveLLMConfig(dbCfg); err != nil {
		return err
	}

	p.InvalidateCache()
	return nil
}

// MaskAPIKey returns a masked version of the API key for display.
func MaskAPIKey(key string) string {
	if len(key) <= 6 {
		return "***"
	}
	return key[:3] + "***" + key[len(key)-3:]
}

// toJSON is a helper for tool result serialization.
func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
