package config

// DefaultConfig returns a Config populated with all default values.
func DefaultConfig() *Config {
	return &Config{
		Retention: RetentionConfig{
			Days:               30,
			PruneIntervalHours: 24,
		},
		Capture: CaptureConfig{
			Mode:                  "metadata_only",
			ExcludeIncognito:      true,
			AllowlistDomains:      []string{},
			DenylistDomains:       []string{},
			DenylistRegex:         []string{},
			BodyCaptureDomains:    []string{},
			DedupeIntervalSeconds: 300,
		},
		Embeddings: EmbeddingsConfig{
			Enabled:     false,
			Provider:    "ollama",
			OllamaURL:   "http://localhost:11434",
			Model:       "nomic-embed-text",
			BatchSize:   16,
			ContentOnly: false,
		},
		Storage: StorageConfig{
			Path:              "~/.config/fabric/chronicle",
			SQLiteFile:        "chronicle.db",
			VectorStore:       "lancedb",
			VectorDir:         "vectors",
			SQLiteJournalMode: "wal",
		},
		Daemon: DaemonConfig{
			Host:           "127.0.0.1",
			Port:           8721,
			AuthToken:      "",
			MaxRequestSize: 10485760,
		},
		Logging: LoggingConfig{
			Level:      "info",
			File:       "chronicle.log",
			AuditLog:   true,
			MaxSize:    10485760,
			MaxBackups: 3,
		},
		Fabric: FabricConfig{
			PatternsDir: "~/.config/fabric/patterns",
			Binary:      "",
		},
	}
}
