package db

const schema = `
CREATE TABLE IF NOT EXISTS registrations (
	id           TEXT PRIMARY KEY,
	display_name TEXT NOT NULL,
	username     TEXT NOT NULL UNIQUE,
	email        TEXT NOT NULL UNIQUE,
	password     TEXT NOT NULL,
	status       TEXT NOT NULL DEFAULT 'pending',
	created_at   TEXT NOT NULL DEFAULT (datetime('now')),
	reviewed_at  TEXT,
	reviewed_by  TEXT
);

CREATE TABLE IF NOT EXISTS audit_log (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	reg_id     TEXT NOT NULL REFERENCES registrations(id),
	action     TEXT NOT NULL,
	detail     TEXT,
	created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_reg_status ON registrations(status);
`
