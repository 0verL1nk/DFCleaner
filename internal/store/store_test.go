package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := InitDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	return New(db)
}

// --- LLM Config CRUD ---

func TestSaveAndGetLLMConfig(t *testing.T) {
	s := newTestStore(t)

	cfg := &LLMConfig{
		Provider:  "openai",
		Endpoint:  "https://api.openai.com/v1",
		APIKey:    "encrypted-key",
		ModelName: "gpt-4o",
		IsActive:  true,
	}
	if err := s.SaveLLMConfig(cfg); err != nil {
		t.Fatalf("SaveLLMConfig: %v", err)
	}

	configs := s.GetLLMConfigs()
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].Provider != "openai" {
		t.Errorf("Provider = %q, want 'openai'", configs[0].Provider)
	}

	active := s.GetActiveLLMConfig()
	if active == nil {
		t.Fatal("GetActiveLLMConfig returned nil")
	}
	if active.ModelName != "gpt-4o" {
		t.Errorf("ModelName = %q, want 'gpt-4o'", active.ModelName)
	}
}

func TestOnlyOneActiveConfig(t *testing.T) {
	s := newTestStore(t)

	s.SaveLLMConfig(&LLMConfig{Provider: "openai", Endpoint: "a", APIKey: "k1", ModelName: "gpt-4o", IsActive: true})
	s.SaveLLMConfig(&LLMConfig{Provider: "ollama", Endpoint: "b", APIKey: "k2", ModelName: "llama3", IsActive: true})

	active := s.GetActiveLLMConfig()
	if active == nil {
		t.Fatal("expected active config")
	}
	if active.Provider != "ollama" {
		t.Errorf("active provider = %q, want 'ollama' (latest active should win)", active.Provider)
	}
}

func TestDeleteLLMConfig(t *testing.T) {
	s := newTestStore(t)

	s.SaveLLMConfig(&LLMConfig{Provider: "openai", Endpoint: "a", APIKey: "k1", ModelName: "gpt-4o", IsActive: true})
	configs := s.GetLLMConfigs()
	if len(configs) != 1 {
		t.Fatalf("expected 1, got %d", len(configs))
	}

	if err := s.DeleteLLMConfig(configs[0].ID); err != nil {
		t.Fatalf("DeleteLLMConfig: %v", err)
	}

	if len(s.GetLLMConfigs()) != 0 {
		t.Error("expected 0 configs after delete")
	}
}

// --- User Settings ---

func TestSettingsCRUD(t *testing.T) {
	s := newTestStore(t)

	if v := s.GetSetting("theme"); v != "" {
		t.Errorf("unset setting should return '', got %q", v)
	}

	if err := s.SetSetting("theme", "dark"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	if v := s.GetSetting("theme"); v != "dark" {
		t.Errorf("theme = %q, want 'dark'", v)
	}

	// Update existing
	s.SetSetting("theme", "light")
	if v := s.GetSetting("theme"); v != "light" {
		t.Errorf("theme after update = %q, want 'light'", v)
	}
}

// --- Cleanup Logs ---

func TestCleanupLog(t *testing.T) {
	s := newTestStore(t)

	log := &CleanupLog{
		FilePath:   "/tmp/cache/test.log",
		Operation:  "trash",
		Result:     "success",
		FreedBytes: 1024,
	}
	if err := s.LogCleanup(log); err != nil {
		t.Fatalf("LogCleanup: %v", err)
	}

	recent := s.GetRecentCleanups(5)
	if len(recent) != 1 {
		t.Fatalf("expected 1 log, got %d", len(recent))
	}
	if recent[0].FilePath != "/tmp/cache/test.log" {
		t.Errorf("FilePath = %q", recent[0].FilePath)
	}
	if recent[0].FreedBytes != 1024 {
		t.Errorf("FreedBytes = %d, want 1024", recent[0].FreedBytes)
	}
}

func TestGetRecentCleanupsLimit(t *testing.T) {
	s := newTestStore(t)

	for i := 0; i < 10; i++ {
		s.LogCleanup(&CleanupLog{
			FilePath:  "/tmp/file",
			Operation: "delete",
			Result:    "success",
		})
	}

	recent := s.GetRecentCleanups(3)
	if len(recent) != 3 {
		t.Errorf("expected 3, got %d", len(recent))
	}
}
