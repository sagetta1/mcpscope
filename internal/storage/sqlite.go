// Package storage persists captured MCP sessions to SQLite.
//
// Schema is intentionally narrow for v0: we record raw JSON-RPC messages
// alongside enough denormalized fields (kind, jsonrpc_id, method) to drive
// the timeline UI without re-parsing on every query. Correlation between
// requests and responses is done at query time by joining on jsonrpc_id.
//
// Driver is modernc.org/sqlite — pure Go, no CGO. This keeps the binary
// cross-compilable with `go build` alone.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite connection pool used by the proxy and CLI.
type Store struct {
	db *sql.DB
}

// Open creates the parent directory if missing, opens the database with WAL
// mode + a 5s busy timeout, and applies migrations.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("storage: mkdir: %w", err)
	}
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("storage: open: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    started_at INTEGER NOT NULL,
    ended_at   INTEGER,
    target_cmd TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT NOT NULL REFERENCES sessions(id),
    ts          INTEGER NOT NULL,
    direction   TEXT NOT NULL CHECK (direction IN ('in', 'out')),
    kind        TEXT NOT NULL CHECK (kind IN ('request', 'response', 'notification', 'invalid')),
    jsonrpc_id  TEXT,
    method      TEXT,
    raw         TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_session_ts ON messages(session_id, ts);
CREATE INDEX IF NOT EXISTS idx_messages_jsonrpc_id ON messages(jsonrpc_id);
`)
	if err != nil {
		return fmt.Errorf("storage: migrate: %w", err)
	}
	return nil
}

// StartSession inserts a new session row. ID is caller-generated so the
// proxy can log it before persistence completes.
func (s *Store) StartSession(id, targetCmd string, startedAtMs int64) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions(id, started_at, target_cmd) VALUES (?, ?, ?)`,
		id, startedAtMs, targetCmd,
	)
	return err
}

// EndSession sets ended_at on the given session.
func (s *Store) EndSession(id string, endedAtMs int64) error {
	_, err := s.db.Exec(`UPDATE sessions SET ended_at = ? WHERE id = ?`, endedAtMs, id)
	return err
}

// AppendMessage records a single classified JSON-RPC line. Empty jsonrpcID
// or method are stored as NULL.
func (s *Store) AppendMessage(sessionID, direction, kind, jsonrpcID, method, raw string, tsMs int64) error {
	var jid, m sql.NullString
	if jsonrpcID != "" {
		jid = sql.NullString{String: jsonrpcID, Valid: true}
	}
	if method != "" {
		m = sql.NullString{String: method, Valid: true}
	}
	_, err := s.db.Exec(
		`INSERT INTO messages(session_id, ts, direction, kind, jsonrpc_id, method, raw)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sessionID, tsMs, direction, kind, jid, m, raw,
	)
	return err
}

// SessionRow is one row returned by ListSessions.
type SessionRow struct {
	ID        string
	StartedAt int64
	EndedAt   sql.NullInt64
	TargetCmd string
	MsgCount  int
}

// ListSessions returns the most recent sessions, newest first.
func (s *Store) ListSessions(ctx context.Context, limit int) ([]SessionRow, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT s.id, s.started_at, s.ended_at, s.target_cmd, COUNT(m.id)
FROM sessions s
LEFT JOIN messages m ON m.session_id = s.id
GROUP BY s.id, s.started_at, s.ended_at, s.target_cmd
ORDER BY s.started_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionRow
	for rows.Next() {
		var r SessionRow
		if err := rows.Scan(&r.ID, &r.StartedAt, &r.EndedAt, &r.TargetCmd, &r.MsgCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MessageRow is one row returned by ListMessages.
type MessageRow struct {
	TS        int64
	Direction string
	Kind      string
	JSONRPCID sql.NullString
	Method    sql.NullString
	Raw       string
}

// ListMessages returns all messages in a session, oldest first.
func (s *Store) ListMessages(ctx context.Context, sessionID string) ([]MessageRow, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT ts, direction, kind, jsonrpc_id, method, raw
FROM messages
WHERE session_id = ?
ORDER BY id ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MessageRow
	for rows.Next() {
		var r MessageRow
		if err := rows.Scan(&r.TS, &r.Direction, &r.Kind, &r.JSONRPCID, &r.Method, &r.Raw); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
