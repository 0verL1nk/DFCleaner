package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
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

func TestImportFromStore(t *testing.T) {
	m := newTestManager(t)

	entries := []LLMEntry{
		{Provider: "openai", Endpoint: "a", APIKey: "k", ModelName: "gpt-4o", IsActive: true},
		{Provider: "ollama", Endpoint: "b", APIKey: "k", ModelName: "llama3", IsActive: false},
	}

	if err := m.ImportFromStore("dark", "zh", entries); err != nil {
		t.Fatalf("ImportFromStore: %v", err)
	}

	if v := m.GetTheme(); v != "dark" {
		t.Errorf("theme = %q, want 'dark'", v)
	}
	if v := m.GetLanguage(); v != "zh" {
		t.Errorf("language = %q, want 'zh'", v)
	}
	configs := m.GetAllLLMs()
	if len(configs) != 2 {
		t.Fatalf("expected 2 LLM entries, got %d", len(configs))
	}
	if configs[0].Provider != "openai" {
		t.Errorf("first provider = %q, want 'openai'", configs[0].Provider)
	}
}

func TestImportFromStoreOverwrites(t *testing.T) {
	m := newTestManager(t)

	// First import
	m.ImportFromStore("light", "en", []LLMEntry{})

	// Second import should overwrite
	m.ImportFromStore("dark", "zh", []LLMEntry{
		{Provider: "ollama", Endpoint: "x", APIKey: "y", ModelName: "llama3", IsActive: true},
	})

	if v := m.GetTheme(); v != "dark" {
		t.Errorf("theme = %q, want 'dark'", v)
	}
	configs := m.GetAllLLMs()
	if len(configs) != 1 {
		t.Fatalf("expected 1 LLM entry after overwrite, got %d", len(configs))
	}
}

func TestImportFromStorePersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	m := &Manager{path: path, config: AppConfig{Theme: "system", Language: "en"}}
	_ = m.save()

	m.ImportFromStore("dark", "zh", []LLMEntry{
		{Provider: "openai", Endpoint: "a", APIKey: "k", ModelName: "gpt-4o", IsActive: true},
	})

	// Load into a new manager and verify
	m2 := &Manager{path: path}
	if err := m2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if m2.GetTheme() != "dark" {
		t.Errorf("theme after reload = %q, want 'dark'", m2.GetTheme())
	}
	if m2.GetLanguage() != "zh" {
		t.Errorf("language after reload = %q, want 'zh'", m2.GetLanguage())
	}
	if len(m2.GetAllLLMs()) != 1 {
		t.Errorf("expected 1 LLM after reload")
	}
}

func TestNewManager(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	xdg.Reload()

	m, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if m.GetTheme() != "system" {
		t.Errorf("default theme = %q, want 'system'", m.GetTheme())
	}
	if m.GetLanguage() != "en" {
		t.Errorf("default language = %q, want 'en'", m.GetLanguage())
	}

	// Verify config file was created
	configPath := filepath.Join(dir, "dfcleaner", "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file should exist")
	}
}

func TestNewManagerLoadsExisting(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	xdg.Reload()

	// First call creates default config
	m1, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager first: %v", err)
	}
	m1.SetTheme("dark")
	m1.SetLanguage("zh")

	// Second call should load existing config
	m2, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager second: %v", err)
	}
	if m2.GetTheme() != "dark" {
		t.Errorf("loaded theme = %q, want 'dark'", m2.GetTheme())
	}
	if m2.GetLanguage() != "zh" {
		t.Errorf("loaded language = %q, want 'zh'", m2.GetLanguage())
	}
}

func TestSaveErrorPath(t *testing.T) {
	m := &Manager{
		path:   "/nonexistent/deeply/nested/dir/config.toml",
		config: AppConfig{Theme: "test", Language: "en"},
	}
	// save should fail because parent directory does not exist
	if err := m.save(); err == nil {
		t.Error("expected error saving to nonexistent directory")
	}
}

func TestLoadErrorBadPath(t *testing.T) {
	m := &Manager{path: "/nonexistent/dir/config.toml"}
	if err := m.load(); err == nil {
		t.Error("expected error loading from nonexistent path")
	}
}

func TestNewManagerCreatesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	// Simulate NewManager by directly creating manager with temp path
	// (NewManager uses xdg.ConfigHome which is harder to redirect)
	m := &Manager{path: configPath}
	m.config = AppConfig{Theme: "system", Language: "en"}
	if err := m.save(); err != nil {
		t.Fatalf("save default config: %v", err)
	}

	// Verify file exists and can be loaded
	m2 := &Manager{path: configPath}
	if err := m2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if m2.GetTheme() != "system" {
		t.Errorf("default theme = %q, want 'system'", m2.GetTheme())
	}
	if m2.GetLanguage() != "en" {
		t.Errorf("default language = %q, want 'en'", m2.GetLanguage())
	}
}

func TestSaveLLMInactiveDoesNotDeactivateOthers(t *testing.T) {
	m := newTestManager(t)

	m.SaveLLM(LLMEntry{Provider: "openai", Endpoint: "a", APIKey: "k1", ModelName: "gpt-4o", IsActive: true})
	m.SaveLLM(LLMEntry{Provider: "ollama", Endpoint: "b", APIKey: "k2", ModelName: "llama3", IsActive: false})

	// First should still be active
	active := m.GetActiveLLM()
	if active == nil || active.Provider != "openai" {
		t.Errorf("openai should still be active")
	}
}

func TestGetAllLLMsReturnsCopy(t *testing.T) {
	m := newTestManager(t)

	m.SaveLLM(LLMEntry{Provider: "openai", Endpoint: "a", APIKey: "k", ModelName: "gpt-4o", IsActive: true})

	configs := m.GetAllLLMs()
	configs[0].APIKey = "modified"

	// Original should be unchanged
	original := m.GetAllLLMs()
	if original[0].APIKey == "modified" {
		t.Error("GetAllLLMs should return a copy, not a reference")
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

func TestSafeModeCRUD(t *testing.T) {
	m := newTestManager(t)

	if v := m.GetSafeMode(); v {
		t.Errorf("default safeMode = %v, want false", v)
	}
	if err := m.SetSafeMode(true); err != nil {
		t.Fatalf("SetSafeMode: %v", err)
	}
	if v := m.GetSafeMode(); !v {
		t.Errorf("safeMode = %v, want true", v)
	}
}

func TestSafeModePersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	m1 := &Manager{path: path, config: AppConfig{Theme: "system", Language: "en"}}
	_ = m1.save()
	m1.SetSafeMode(true)

	m2 := &Manager{path: path}
	if err := m2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !m2.GetSafeMode() {
		t.Error("safe mode should persist as true after reload")
	}
}
