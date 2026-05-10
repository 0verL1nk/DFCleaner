package config

import (
	"path/filepath"
	"testing"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	m := &Manager{
		path:   filepath.Join(dir, "config.toml"),
		config: AppConfig{Theme: "system", Language: "en"},
	}
	if err := m.save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	return m
}

func TestThemeCRUD(t *testing.T) {
	m := newTestManager(t)

	if v := m.GetTheme(); v != "system" {
		t.Errorf("default theme = %q, want 'system'", v)
	}
	if err := m.SetTheme("dark"); err != nil {
		t.Fatalf("SetTheme: %v", err)
	}
	if v := m.GetTheme(); v != "dark" {
		t.Errorf("theme = %q, want 'dark'", v)
	}
}

func TestLanguageCRUD(t *testing.T) {
	m := newTestManager(t)

	if err := m.SetLanguage("zh"); err != nil {
		t.Fatalf("SetLanguage: %v", err)
	}
	if v := m.GetLanguage(); v != "zh" {
		t.Errorf("language = %q, want 'zh'", v)
	}
}

func TestSaveAndGetLLM(t *testing.T) {
	m := newTestManager(t)

	entry := LLMEntry{
		Provider:  "openai",
		Endpoint:  "https://api.openai.com/v1",
		APIKey:    "encrypted-key",
		ModelName: "gpt-4o",
		IsActive:  true,
	}
	if err := m.SaveLLM(entry); err != nil {
		t.Fatalf("SaveLLM: %v", err)
	}

	configs := m.GetAllLLMs()
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].Provider != "openai" {
		t.Errorf("Provider = %q, want 'openai'", configs[0].Provider)
	}

	active := m.GetActiveLLM()
	if active == nil {
		t.Fatal("GetActiveLLM returned nil")
	}
	if active.ModelName != "gpt-4o" {
		t.Errorf("ModelName = %q, want 'gpt-4o'", active.ModelName)
	}
}

func TestOnlyOneActiveLLM(t *testing.T) {
	m := newTestManager(t)

	m.SaveLLM(LLMEntry{Provider: "openai", Endpoint: "a", APIKey: "k1", ModelName: "gpt-4o", IsActive: true})
	m.SaveLLM(LLMEntry{Provider: "ollama", Endpoint: "b", APIKey: "k2", ModelName: "llama3", IsActive: true})

	active := m.GetActiveLLM()
	if active == nil {
		t.Fatal("expected active config")
	}
	if active.Provider != "ollama" {
		t.Errorf("active provider = %q, want 'ollama'", active.Provider)
	}
}

func TestUpdateExistingLLM(t *testing.T) {
	m := newTestManager(t)

	m.SaveLLM(LLMEntry{Provider: "openai", Endpoint: "a", APIKey: "k1", ModelName: "gpt-4o", IsActive: true})
	m.SaveLLM(LLMEntry{Provider: "openai", Endpoint: "a", APIKey: "k2", ModelName: "gpt-4o", IsActive: true})

	configs := m.GetAllLLMs()
	if len(configs) != 1 {
		t.Fatalf("expected 1 config (updated), got %d", len(configs))
	}
	if configs[0].APIKey != "k2" {
		t.Errorf("APIKey = %q, want 'k2' (updated)", configs[0].APIKey)
	}
}

func TestNoActiveLLM(t *testing.T) {
	m := newTestManager(t)

	if active := m.GetActiveLLM(); active != nil {
		t.Errorf("expected nil, got %+v", active)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	m1 := &Manager{path: path, config: AppConfig{Theme: "system", Language: "en"}}
	_ = m1.save()
	m1.SetTheme("dark")
	m1.SetLanguage("zh")
	m1.SaveLLM(LLMEntry{Provider: "openai", Endpoint: "a", APIKey: "k", ModelName: "gpt-4o", IsActive: true})

	// Load into new manager
	m2 := &Manager{path: path}
	if err := m2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if m2.GetTheme() != "dark" {
		t.Errorf("theme = %q, want 'dark'", m2.GetTheme())
	}
	if m2.GetLanguage() != "zh" {
		t.Errorf("language = %q, want 'zh'", m2.GetLanguage())
	}
	if len(m2.GetAllLLMs()) != 1 {
		t.Errorf("expected 1 LLM config")
	}
}
