package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Default config file path.
const DefaultConfigPath = "~/.config/fabric/chronicle/config.yaml"

// Config holds all Chronicle configuration.
type Config struct {
	Retention  RetentionConfig  `yaml:"retention"`
	Capture    CaptureConfig    `yaml:"capture"`
	Embeddings EmbeddingsConfig `yaml:"embeddings"`
	Storage    StorageConfig    `yaml:"storage"`
	Daemon     DaemonConfig     `yaml:"daemon"`
	Logging    LoggingConfig    `yaml:"logging"`
	Fabric     FabricConfig     `yaml:"fabric"`
}

type RetentionConfig struct {
	Days               int `yaml:"days"`
	PruneIntervalHours int `yaml:"prune_interval_hours"`
}

type CaptureConfig struct {
	Mode                  string   `yaml:"mode"`
	ExcludeIncognito      bool     `yaml:"exclude_incognito"`
	AllowlistDomains      []string `yaml:"allowlist_domains"`
	DenylistDomains       []string `yaml:"denylist_domains"`
	DenylistRegex         []string `yaml:"denylist_regex"`
	BodyCaptureDomains    []string `yaml:"body_capture_domains"`
	DedupeIntervalSeconds int      `yaml:"dedupe_interval_seconds"`
}

type EmbeddingsConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Provider    string `yaml:"provider"`
	OllamaURL   string `yaml:"ollama_url"`
	Model       string `yaml:"model"`
	BatchSize   int    `yaml:"batch_size"`
	ContentOnly bool   `yaml:"content_only"`
}

type StorageConfig struct {
	Path              string `yaml:"path"`
	SQLiteFile        string `yaml:"sqlite_file"`
	VectorStore       string `yaml:"vector_store"`
	VectorDir         string `yaml:"vector_dir"`
	SQLiteJournalMode string `yaml:"sqlite_journal_mode"`
}

type DaemonConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	AuthToken      string `yaml:"auth_token"`
	MaxRequestSize int    `yaml:"max_request_size"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`
	File       string `yaml:"file"`
	AuditLog   bool   `yaml:"audit_log"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
}

type FabricConfig struct {
	PatternsDir string `yaml:"patterns_dir"`
	Binary      string `yaml:"binary"`
}

// Load reads a YAML config file at path and merges it with defaults.
// Returns an error if the file cannot be read or contains invalid YAML.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// ExcludeIncognito is always true regardless of config file.
	cfg.Capture.ExcludeIncognito = true

	return cfg, nil
}

// expandPath replaces a leading ~ with the user's home directory.
func expandPath(path string) (string, error) {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		return filepath.Join(home, path[1:]), nil
	}
	return path, nil
}

// LoadOrCreate loads the config from the default path. If the file does
// not exist, it creates the directory structure and writes defaults.
func LoadOrCreate() (*Config, error) {
	path, err := expandPath(DefaultConfigPath)
	if err != nil {
		return nil, err
	}
	return LoadOrCreateAt(path)
}

// LoadOrCreateAt loads the config from the given path. If the file does
// not exist, it creates the directory structure and writes defaults.
func LoadOrCreateAt(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := DefaultConfig()

		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating config directory: %w", err)
		}

		data, err := yaml.Marshal(cfg)
		if err != nil {
			return nil, fmt.Errorf("marshaling default config: %w", err)
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return nil, fmt.Errorf("writing default config: %w", err)
		}

		return cfg, nil
	}

	return Load(path)
}
