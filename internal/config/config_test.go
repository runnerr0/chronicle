package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 30, cfg.Retention.Days)
	assert.Equal(t, 24, cfg.Retention.PruneIntervalHours)
	assert.Equal(t, "metadata_only", cfg.Capture.Mode)
	assert.True(t, cfg.Capture.ExcludeIncognito)
	assert.Equal(t, 300, cfg.Capture.DedupeIntervalSeconds)
	assert.False(t, cfg.Embeddings.Enabled)
	assert.Equal(t, "ollama", cfg.Embeddings.Provider)
	assert.Equal(t, "http://localhost:11434", cfg.Embeddings.OllamaURL)
	assert.Equal(t, "nomic-embed-text", cfg.Embeddings.Model)
	assert.Equal(t, 16, cfg.Embeddings.BatchSize)
	assert.Equal(t, "~/.config/fabric/chronicle", cfg.Storage.Path)
	assert.Equal(t, "chronicle.db", cfg.Storage.SQLiteFile)
	assert.Equal(t, "lancedb", cfg.Storage.VectorStore)
	assert.Equal(t, "vectors", cfg.Storage.VectorDir)
	assert.Equal(t, "wal", cfg.Storage.SQLiteJournalMode)
	assert.Equal(t, "127.0.0.1", cfg.Daemon.Host)
	assert.Equal(t, 8721, cfg.Daemon.Port)
	assert.Equal(t, 10485760, cfg.Daemon.MaxRequestSize)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "chronicle.log", cfg.Logging.File)
	assert.True(t, cfg.Logging.AuditLog)
	assert.Equal(t, 10485760, cfg.Logging.MaxSize)
	assert.Equal(t, 3, cfg.Logging.MaxBackups)
	assert.Equal(t, "~/.config/fabric/patterns", cfg.Fabric.PatternsDir)
	assert.Empty(t, cfg.Fabric.Binary)
}

func TestDefaultDenylistIsPopulated(t *testing.T) {
	domains := DefaultDenylistDomains()
	assert.NotEmpty(t, domains)
	assert.Greater(t, len(domains), 10)

	// Spot-check some categories
	assert.Contains(t, domains, "chase.com")
	assert.Contains(t, domains, "bankofamerica.com")
	assert.Contains(t, domains, "1password.com")
	assert.Contains(t, domains, "mychart.com")
}

func TestLoadValidYAMLOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
retention:
  days: 90
  prune_interval_hours: 12
capture:
  mode: "metadata_plus_body"
  dedupe_interval_seconds: 600
daemon:
  port: 9999
logging:
  level: "debug"
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(cfgPath)
	require.NoError(t, err)

	// Overridden values
	assert.Equal(t, 90, cfg.Retention.Days)
	assert.Equal(t, 12, cfg.Retention.PruneIntervalHours)
	assert.Equal(t, "metadata_plus_body", cfg.Capture.Mode)
	assert.Equal(t, 600, cfg.Capture.DedupeIntervalSeconds)
	assert.Equal(t, 9999, cfg.Daemon.Port)
	assert.Equal(t, "debug", cfg.Logging.Level)

	// Non-overridden values remain defaults
	assert.True(t, cfg.Capture.ExcludeIncognito)
	assert.Equal(t, "127.0.0.1", cfg.Daemon.Host)
	assert.Equal(t, "ollama", cfg.Embeddings.Provider)
	assert.Equal(t, "~/.config/fabric/chronicle", cfg.Storage.Path)
}

func TestLoadInvalidYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	err := os.WriteFile(cfgPath, []byte(":::not valid yaml{{{"), 0644)
	require.NoError(t, err)

	_, err = Load(cfgPath)
	assert.Error(t, err)
}

func TestLoadNonExistentFileReturnsError(t *testing.T) {
	_, err := Load("/tmp/nonexistent_path_12345/config.yaml")
	assert.Error(t, err)
}

func TestLoadOrCreateCreatesDefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sub", "deep", "config.yaml")

	cfg, err := LoadOrCreateAt(cfgPath)
	require.NoError(t, err)

	// Should return defaults
	assert.Equal(t, 30, cfg.Retention.Days)
	assert.Equal(t, "metadata_only", cfg.Capture.Mode)
	assert.Equal(t, "127.0.0.1", cfg.Daemon.Host)

	// File should now exist on disk
	_, statErr := os.Stat(cfgPath)
	assert.NoError(t, statErr)

	// File should be valid YAML loadable again
	cfg2, err := Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, cfg.Retention.Days, cfg2.Retention.Days)
}

func TestLoadOrCreateLoadsExistingFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
retention:
  days: 7
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadOrCreateAt(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, 7, cfg.Retention.Days)
	// Other fields remain defaults
	assert.Equal(t, "metadata_only", cfg.Capture.Mode)
}

func TestLoadPartialYAMLMergesWithDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	// Only override one nested field
	yamlContent := `
embeddings:
  enabled: true
  model: "all-minilm"
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(cfgPath)
	require.NoError(t, err)

	assert.True(t, cfg.Embeddings.Enabled)
	assert.Equal(t, "all-minilm", cfg.Embeddings.Model)
	// Other embeddings fields remain default
	assert.Equal(t, "ollama", cfg.Embeddings.Provider)
	assert.Equal(t, "http://localhost:11434", cfg.Embeddings.OllamaURL)
	assert.Equal(t, 16, cfg.Embeddings.BatchSize)
}

func TestLoadWithDenylistDomains(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
capture:
  denylist_domains:
    - "example.com"
    - "secret.org"
`
	err := os.WriteFile(cfgPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, []string{"example.com", "secret.org"}, cfg.Capture.DenylistDomains)
}
