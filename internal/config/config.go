package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	WatchPaths     []string       `yaml:"watch_paths"`
	OutputDir      string         `yaml:"output_dir"`
	LLM            LLMConfig      `yaml:"llm"`
	Debounce       DebounceConfig `yaml:"debounce"`
	Polling        PollingConfig  `yaml:"polling"`
	InitialScan    bool           `yaml:"initial_scan"`
	FollowSymlinks bool           `yaml:"follow_symlinks"`
	IgnorePatterns []string       `yaml:"ignore_patterns"`
	LogLevel       string         `yaml:"log_level"`
}

type PollingConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}

type LLMConfig struct {
	BaseURL             string  `yaml:"base_url"`
	APIKey              string  `yaml:"api_key"`
	Model               string  `yaml:"model"`
	MaxTokens           int     `yaml:"max_tokens"`
	Temperature         float64 `yaml:"temperature"`
	MaxBatchSize        int     `yaml:"max_batch_size"`
	MaxFileContentBytes int     `yaml:"max_file_content_bytes"`
}

type DebounceConfig struct {
	Interval time.Duration `yaml:"interval"`
	MaxWait  time.Duration `yaml:"max_wait"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	setDefaults(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.LLM.Model == "" {
		cfg.LLM.Model = "claude-haiku-4-5"
	}
	if cfg.LLM.MaxTokens == 0 {
		cfg.LLM.MaxTokens = 4096
	}
	if cfg.LLM.Temperature == 0 {
		cfg.LLM.Temperature = 0.3
	}
	if cfg.LLM.MaxBatchSize == 0 {
		cfg.LLM.MaxBatchSize = 50
	}
	if cfg.LLM.MaxFileContentBytes == 0 {
		cfg.LLM.MaxFileContentBytes = 2048
	}
	if cfg.Debounce.Interval == 0 {
		cfg.Debounce.Interval = 3 * time.Second
	}
	if cfg.Debounce.MaxWait == 0 {
		cfg.Debounce.MaxWait = 30 * time.Second
	}
	if cfg.Polling.Enabled && cfg.Polling.Interval == 0 {
		cfg.Polling.Interval = 10 * time.Second
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if len(cfg.IgnorePatterns) == 0 {
		cfg.IgnorePatterns = []string{".git", "node_modules", ".DS_Store", "*.tmp", "*.swp"}
	}
}

func validate(cfg *Config) error {
	if len(cfg.WatchPaths) == 0 {
		return fmt.Errorf("watch_paths is required")
	}
	for _, p := range cfg.WatchPaths {
		info, err := os.Stat(p)
		if err != nil {
			return fmt.Errorf("watch_path %q: %w", p, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("watch_path %q is not a directory", p)
		}
	}
	if cfg.OutputDir == "" {
		return fmt.Errorf("output_dir is required")
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create output_dir %q: %w", cfg.OutputDir, err)
	}
	return nil
}
