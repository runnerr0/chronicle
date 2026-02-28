package storage

import "database/sql"

// migrateV001 creates the initial Chronicle schema: all tables, indexes,
// and default exclusion rules. Every statement uses IF NOT EXISTS for
// idempotency.
func migrateV001(tx *sql.Tx) error {
	stmts := []string{
		// ── Tables ──────────────────────────────────────────────

		`CREATE TABLE IF NOT EXISTS events (
			id            TEXT PRIMARY KEY,
			ts            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			url           TEXT NOT NULL,
			title         TEXT NOT NULL DEFAULT '',
			domain        TEXT NOT NULL DEFAULT '',
			browser       TEXT NOT NULL DEFAULT '',
			source        TEXT NOT NULL DEFAULT 'extension',
			has_body      BOOLEAN NOT NULL DEFAULT 0,
			has_embedding BOOLEAN NOT NULL DEFAULT 0,
			content_hash  TEXT,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS content (
			event_id   TEXT PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
			format     TEXT NOT NULL DEFAULT 'md',
			body       TEXT NOT NULL,
			byte_size  INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS exclusions (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_type  TEXT NOT NULL CHECK (rule_type IN ('domain', 'regex')),
			rule_value TEXT NOT NULL,
			reason     TEXT NOT NULL DEFAULT '',
			is_default BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(rule_type, rule_value)
		)`,

		`CREATE TABLE IF NOT EXISTS config (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS embedding_metadata (
			event_id      TEXT PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
			model_name    TEXT NOT NULL,
			model_version TEXT NOT NULL DEFAULT '',
			dimensions    INTEGER NOT NULL,
			embedded_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS audit_log (
			id       INTEGER PRIMARY KEY AUTOINCREMENT,
			action   TEXT NOT NULL,
			detail   TEXT NOT NULL DEFAULT '',
			event_id TEXT,
			ts       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,

		// ── Indexes ────────────────────────────────────────────

		`CREATE INDEX IF NOT EXISTS idx_events_ts           ON events(ts)`,
		`CREATE INDEX IF NOT EXISTS idx_events_domain       ON events(domain)`,
		`CREATE INDEX IF NOT EXISTS idx_events_browser      ON events(browser)`,
		`CREATE INDEX IF NOT EXISTS idx_events_source       ON events(source)`,
		`CREATE INDEX IF NOT EXISTS idx_events_content_hash ON events(content_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_events_ts_domain    ON events(ts, domain)`,
		`CREATE INDEX IF NOT EXISTS idx_events_flags        ON events(has_body, has_embedding)`,
		`CREATE INDEX IF NOT EXISTS idx_exclusions_rule     ON exclusions(rule_type, rule_value)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_ts        ON audit_log(ts)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_log_action    ON audit_log(action)`,
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	// ── Default exclusion rules ────────────────────────────────
	if err := seedDefaultExclusions(tx); err != nil {
		return err
	}

	return nil
}

// seedDefaultExclusions inserts the curated denylist. Uses INSERT OR IGNORE
// so re-running is safe.
func seedDefaultExclusions(tx *sql.Tx) error {
	type rule struct {
		RuleType  string
		RuleValue string
		Reason    string
	}

	defaults := []rule{
		// Banking & Financial
		{"domain", "chase.com", "Banking - financial privacy"},
		{"domain", "bankofamerica.com", "Banking - financial privacy"},
		{"domain", "wellsfargo.com", "Banking - financial privacy"},
		{"domain", "citi.com", "Banking - financial privacy"},
		{"domain", "capitalone.com", "Banking - financial privacy"},
		{"domain", "usbank.com", "Banking - financial privacy"},
		{"domain", "schwab.com", "Banking - financial privacy"},
		{"domain", "fidelity.com", "Banking - financial privacy"},
		{"domain", "vanguard.com", "Banking - financial privacy"},
		{"domain", "paypal.com", "Payment - financial privacy"},
		{"domain", "venmo.com", "Payment - financial privacy"},
		// Password Managers
		{"domain", "1password.com", "Password manager - credential privacy"},
		{"domain", "bitwarden.com", "Password manager - credential privacy"},
		{"domain", "lastpass.com", "Password manager - credential privacy"},
		{"domain", "dashlane.com", "Password manager - credential privacy"},
		// Auth Providers
		{"domain", "accounts.google.com", "Auth provider - credential privacy"},
		{"domain", "login.microsoftonline.com", "Auth provider - credential privacy"},
		{"domain", "auth0.com", "Auth provider - credential privacy"},
		{"domain", "okta.com", "Auth provider - credential privacy"},
		// Healthcare
		{"domain", "mychart.com", "Healthcare - HIPAA privacy"},
		{"domain", "patient.myuhc.com", "Healthcare - HIPAA privacy"},
		// Tax / Government
		{"domain", "irs.gov", "Tax - financial privacy"},
		{"domain", "turbotax.intuit.com", "Tax - financial privacy"},
		// Adult content (regex)
		{"regex", `.*\.xxx$`, "Adult content exclusion"},
		{"regex", `.*pornhub\.com$`, "Adult content exclusion"},
	}

	const insertSQL = `INSERT OR IGNORE INTO exclusions (rule_type, rule_value, reason, is_default) VALUES (?, ?, ?, 1)`

	for _, r := range defaults {
		if _, err := tx.Exec(insertSQL, r.RuleType, r.RuleValue, r.Reason); err != nil {
			return err
		}
	}

	return nil
}
