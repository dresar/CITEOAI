package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type SandboxConfig struct {
	Mode         string   `json:"mode"`
	AllowedPaths []string `json:"allowed_paths"`
	DeniedPaths  []string `json:"denied_paths"`
}

type MemoryConfig struct {
	MaxNotes       int `json:"max_notes"`
	MaxPromptChars int `json:"max_prompt_chars"`
}

type Config struct {
	Provider          string        `json:"provider"`
	ModelName         string        `json:"model_name"`
	APIKey            string        `json:"api_key"`
	BaseURL           string        `json:"base_url"`
	SystemInstruction string        `json:"system_instruction"`
	MaxTokens         int           `json:"max_tokens"`
	Temperature       float64       `json:"temperature"`
	Sandbox           SandboxConfig `json:"sandbox"`
	Memory            MemoryConfig  `json:"memory"`
}

func DefaultConfig() Config {
	return Config{
		Provider:          "gemini",
		ModelName:         "gemini-3.6-flash",
		APIKey:            "",
		BaseURL:           "",
		SystemInstruction: "You are CITEOAI, an ultra-fast, autonomous coding and research agent.",
		MaxTokens:         8192,
		Temperature:       0.7,
		Sandbox: SandboxConfig{
			Mode:         "restricted",
			AllowedPaths: []string{"."},
			DeniedPaths: []string{
				".ssh",
				".gnupg",
			},
		},
		Memory: MemoryConfig{
			MaxNotes:       100,
			MaxPromptChars: 3000,
		},
	}
}

func UserHome() string {
	if customHome := os.Getenv("CITEOAI_HOME"); customHome != "" {
		return customHome
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".citeoai")
}

func DefaultConfigPath() string {
	home := UserHome()
	if home == "" {
		return "config.json"
	}
	return filepath.Join(home, "config.json")
}

func LoadConfig(path string) (*Config, error) {
	resolvedPath := path
	if resolvedPath == "" {
		resolvedPath = DefaultConfigPath()
	}

	cfg := DefaultConfig()

	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else {
		cleanedData := bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
		if err := json.Unmarshal(cleanedData, &cfg); err != nil {
			return nil, err
		}
	}

	applyEnvironmentOverrides(&cfg)
	return &cfg, nil
}

func applyEnvironmentOverrides(cfg *Config) {
	if provider := os.Getenv("CITEOAI_PROVIDER"); provider != "" {
		cfg.Provider = strings.TrimSpace(provider)
	}
	if model := os.Getenv("CITEOAI_MODEL"); model != "" {
		cfg.ModelName = strings.TrimSpace(model)
	}
	if apiKey := os.Getenv("CITEOAI_API_KEY"); apiKey != "" {
		cfg.APIKey = strings.TrimSpace(apiKey)
	}
	if baseURL := os.Getenv("CITEOAI_BASE_URL"); baseURL != "" {
		cfg.BaseURL = strings.TrimSpace(baseURL)
	}
	if sandboxMode := os.Getenv("CITEOAI_SANDBOX_MODE"); sandboxMode != "" {
		cfg.Sandbox.Mode = strings.TrimSpace(sandboxMode)
	}
}

func SaveConfig(path string, cfg *Config) error {
	resolvedPath := path
	if resolvedPath == "" {
		resolvedPath = DefaultConfigPath()
	}

	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(resolvedPath, data, 0600)
}
