package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// ---------------------------------------------------------------------------
//  Config types
// ---------------------------------------------------------------------------

type Provider struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	APIKey    string `json:"api_key"`
	APIFormat string `json:"api_format"` // "openai" (default) or "anthropic"
	Enabled   bool   `json:"enabled"`
}

// GetAPIFormat returns the provider's API format, defaulting to "openai"
func (p *Provider) GetAPIFormat() string {
	if p.APIFormat == "anthropic" {
		return "anthropic"
	}
	return "openai"
}

type ModelMapping struct {
	DisplayName string `json:"display_name"`
	RealModel   string `json:"real_model"`
	ProviderID  string `json:"provider"`
	Enabled     bool   `json:"enabled"`
}

type ListenConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type Config struct {
	Listen             ListenConfig         `json:"listen"`
	Providers          map[string]*Provider `json:"providers"`
	Models             []*ModelMapping      `json:"models"`
	LogLevel           string               `json:"log_level"`
	DebugMode          bool                 `json:"debug_mode"`
	AutoStart          bool                 `json:"auto_start"`
	OpenDashOnStart    bool                 `json:"open_dashboard_on_start"`
}

// ---------------------------------------------------------------------------
//  Config Manager
// ---------------------------------------------------------------------------

type ConfigManager struct {
	mu   sync.RWMutex
	path string
	data *Config
}

func NewConfigManager(path string) *ConfigManager {
	cm := &ConfigManager{path: path}
	cm.load()
	return cm
}

func (cm *ConfigManager) load() {
	cm.data = &Config{
		Listen:    ListenConfig{Host: "127.0.0.1", Port: 8089},
		Providers: make(map[string]*Provider),
		Models:    make([]*ModelMapping, 0),
		LogLevel:  "info",
		AutoStart: true,
	}

	f, err := os.Open(cm.path)
	if err != nil {
		log.Printf("[config] No config at %s, using defaults", cm.path)
		return
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(cm.data); err != nil {
		log.Printf("[config] Parse error: %v, using defaults", err)
	}
	if cm.data.Providers == nil {
		cm.data.Providers = make(map[string]*Provider)
	}
	if cm.data.Models == nil {
		cm.data.Models = make([]*ModelMapping, 0)
	}
}

func (cm *ConfigManager) Save() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.saveUnsafe()
}

func (cm *ConfigManager) saveUnsafe() error {
	dir := filepath.Dir(cm.path)
	os.MkdirAll(dir, 0755)

	f, err := os.Create(cm.path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(cm.data)
}

func (cm *ConfigManager) Get() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.data
}

func (cm *ConfigManager) GetJSON() []byte {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	b, _ := json.Marshal(cm.data)
	return b
}

func (cm *ConfigManager) Update(patch json.RawMessage) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if err := json.Unmarshal(patch, cm.data); err != nil {
		return err
	}
	return cm.saveUnsafe()
}

func (cm *ConfigManager) SetProviders(providers map[string]*Provider) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.data.Providers = providers
	cm.saveUnsafe()
}

func (cm *ConfigManager) AddProvider(id string, p *Provider) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.data.Providers[id] = p
	cm.saveUnsafe()
}

func (cm *ConfigManager) DeleteProvider(id string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.data.Providers, id)
	cm.saveUnsafe()
}

func (cm *ConfigManager) UpdateProvider(id string, p *Provider) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if existing, ok := cm.data.Providers[id]; ok {
		if p.Name != "" {
			existing.Name = p.Name
		}
		if p.BaseURL != "" {
			existing.BaseURL = p.BaseURL
		}
		if p.APIKey != "" {
			existing.APIKey = p.APIKey
		}
		if p.APIFormat != "" {
			existing.APIFormat = p.APIFormat
		}
		existing.Enabled = p.Enabled
	}
	cm.saveUnsafe()
}

func (cm *ConfigManager) AddModel(m *ModelMapping) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.data.Models = append(cm.data.Models, m)
	cm.saveUnsafe()
}

func (cm *ConfigManager) UpdateModel(idx int, patch *ModelMapping) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if idx >= 0 && idx < len(cm.data.Models) {
		existing := cm.data.Models[idx]
		if patch.DisplayName != "" {
			existing.DisplayName = patch.DisplayName
		}
		if patch.RealModel != "" {
			existing.RealModel = patch.RealModel
		}
		if patch.ProviderID != "" {
			existing.ProviderID = patch.ProviderID
		}
		existing.Enabled = patch.Enabled
	}
	cm.saveUnsafe()
}

func (cm *ConfigManager) DeleteModel(idx int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if idx >= 0 && idx < len(cm.data.Models) {
		cm.data.Models = append(cm.data.Models[:idx], cm.data.Models[idx+1:]...)
	}
	cm.saveUnsafe()
}

// BuildModelMap returns display_name -> (real_model, provider)
func (cm *ConfigManager) BuildModelMap() map[string]ResolvedModel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]ResolvedModel)
	for _, m := range cm.data.Models {
		if !m.Enabled {
			continue
		}
		prov, ok := cm.data.Providers[m.ProviderID]
		if !ok || !prov.Enabled {
			continue
		}
		result[m.DisplayName] = ResolvedModel{
			RealModel: m.RealModel,
			Provider:  prov,
		}
	}
	return result
}

func (cm *ConfigManager) GetFirstEnabledProvider() *Provider {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	for _, p := range cm.data.Providers {
		if p.Enabled {
			return p
		}
	}
	return nil
}

type ResolvedModel struct {
	RealModel string
	Provider  *Provider
}
