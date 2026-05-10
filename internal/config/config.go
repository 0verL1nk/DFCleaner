package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/adrg/xdg"
	"github.com/pelletier/go-toml/v2"
)

type LLMEntry struct {
	Provider  string `toml:"provider" json:"provider"`
	Endpoint  string `toml:"endpoint" json:"endpoint"`
	APIKey    string `toml:"api_key" json:"apiKey"`
	ModelName string `toml:"model_name" json:"modelName"`
	IsActive  bool   `toml:"is_active" json:"isActive"`
}

type AppConfig struct {
	Theme    string     `toml:"theme"`
	Language string     `toml:"language"`
	SafeMode bool       `toml:"safe_mode"`
	LLM      []LLMEntry `toml:"llm"`
}

type Manager struct {
	mu     sync.RWMutex
	path   string
	config AppConfig
}

func NewManager() (*Manager, error) {
	configDir := filepath.Join(xdg.ConfigHome, "dfcleaner")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	configPath := filepath.Join(configDir, "config.toml")
	m := &Manager{path: configPath}

	if _, err := os.Stat(configPath); err == nil {
		if err := m.load(); err != nil {
			return nil, err
		}
	} else {
		m.config = AppConfig{
			Theme:    "system",
			Language: "en",
		}
		if err := m.save(); err != nil {
			return nil, err
		}
	}

	return m, nil
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	return toml.Unmarshal(data, &m.config)
}

func (m *Manager) save() error {
	data, err := toml.Marshal(&m.config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(m.path, data, 0644)
}

func (m *Manager) GetTheme() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Theme
}

func (m *Manager) SetTheme(theme string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Theme = theme
	return m.save()
}

func (m *Manager) GetLanguage() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Language
}

func (m *Manager) SetLanguage(lang string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Language = lang
	return m.save()
}

func (m *Manager) GetSafeMode() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.SafeMode
}

func (m *Manager) SetSafeMode(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.SafeMode = enabled
	return m.save()
}

func (m *Manager) GetActiveLLM() *LLMEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range m.config.LLM {
		if m.config.LLM[i].IsActive {
			return &m.config.LLM[i]
		}
	}
	return nil
}

func (m *Manager) GetAllLLMs() []LLMEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]LLMEntry, len(m.config.LLM))
	copy(result, m.config.LLM)
	return result
}

func (m *Manager) SaveLLM(entry LLMEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.IsActive {
		for i := range m.config.LLM {
			m.config.LLM[i].IsActive = false
		}
	}

	for i := range m.config.LLM {
		if m.config.LLM[i].Provider == entry.Provider &&
			m.config.LLM[i].Endpoint == entry.Endpoint &&
			m.config.LLM[i].ModelName == entry.ModelName {
			m.config.LLM[i] = entry
			return m.save()
		}
	}

	m.config.LLM = append(m.config.LLM, entry)
	return m.save()
}

// ImportFromStore migrates settings from SQLite store on first run.
func (m *Manager) ImportFromStore(theme, language string, llmEntries []LLMEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Theme = theme
	m.config.Language = language
	m.config.LLM = llmEntries
	return m.save()
}
