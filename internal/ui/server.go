// Package ui exposes captured MCP traffic to the browser. It serves a small
// JSON API plus an SSE live stream from the same SQLite store the wrap
// subprocesses write into. The compiled React frontend is mounted at "/" via
// embed.FS (see embed.go).
//
// Live mode is implemented by polling the database every 500ms and pushing
// any new rows over Server-Sent Events. Polling beats fsnotify here because
// SQLite WAL writes don't reliably touch the main file's mtime, and a
// 500ms latency is invisible in a DevTools-style UI.
package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sagetta1/mcpscope/internal/storage"
)

// pollInterval is how often the SSE handler queries SQLite for new messages.
// 500ms keeps the UI feeling live without hammering the DB.
const pollInterval = 500 * time.Millisecond

// Server wires HTTP handlers against a storage.Store. Construct with
// NewServer, then pass Handler() to http.Server.
type Server struct {
	store *storage.Store
	mux   *http.ServeMux
}

// NewServer registers all API routes against the given store and mounts the
// embedded frontend at "/". The returned http.Handler is ready to serve.
func NewServer(store *storage.Store) http.Handler {
	s := &Server{store: store, mux: http.NewServeMux()}

	s.mux.HandleFunc("GET /api/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /api/sessions/{id}/messages", s.handleListMessages)
	s.mux.HandleFunc("GET /api/sessions/{id}/live", s.handleLive)

	// Frontend (provided by embed.go via FrontendHandler). Mounted last so
	// API routes win on conflict.
	s.mux.Handle("/", FrontendHandler())

	return s.mux
}

// --- handlers ---

type sessionDTO struct {
	ID         string `json:"id"`
	StartedAt  int64  `json:"started_at"`
	EndedAt    *int64 `json:"ended_at,omitempty"`
	TargetCmd  string `json:"target_cmd"`
	MsgCount   int    `json:"msg_count"`
}

type messageDTO struct {
	ID        int64   `json:"id"`
	TS        int64   `json:"ts"`
	Direction string  `json:"direction"`
	Kind      string  `json:"kind"`
	JSONRPCID *string `json:"jsonrpc_id,omitempty"`
	Method    *string `json:"method,omitempty"`
	Raw       string  `json:"raw"`
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	rows, err := s.store.ListSessions(r.Context(), limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]sessionDTO, 0, len(rows))
	for _, r := range rows {
		dto := sessionDTO{ID: r.ID, StartedAt: r.StartedAt, TargetCmd: r.TargetCmd, MsgCount: r.MsgCount}
		if r.EndedAt.Valid {
			v := r.EndedAt.Int64
			dto.EndedAt = &v
		}
		out = append(out, dto)
	}
	writeJSON(w, out)
}

func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("id")
	if sid == "" {
		writeErr(w, http.StatusBadRequest, errors.New("missing session id"))
		return
	}
	var afterID int64
	if v := r.URL.Query().Get("from"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			afterID = n
		}
	}
	rows, err := s.store.MessagesSince(r.Context(), sid, afterID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, messagesToDTO(rows))
}

func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("id")
	if sid == "" {
		writeErr(w, http.StatusBadRequest, errors.New("missing session id"))
		return
	}
	var afterID int64
	if v := r.URL.Query().Get("from"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			afterID = n
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, errors.New("streaming unsupported"))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Disable proxy buffering (we don't deploy behind nginx, but harmless).
	w.Header().Set("X-Accel-Buffering", "no")

	// Initial flush so the browser opens the EventSource cleanly.
	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rows, err := s.store.MessagesSince(ctx, sid, afterID)
			if err != nil {
				// Client likely disconnected; bail.
				return
			}
			for _, row := range rows {
				dto := messageRowToDTO(row)
				buf, err := json.Marshal(dto)
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(w, "data: %s\n\n", buf); err != nil {
					return
				}
				afterID = row.ID
			}
			flusher.Flush()
		}
	}
}

// --- helpers ---

func messagesToDTO(rows []storage.MessageRow) []messageDTO {
	out := make([]messageDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, messageRowToDTO(r))
	}
	return out
}

func messageRowToDTO(r storage.MessageRow) messageDTO {
	dto := messageDTO{
		ID:        r.ID,
		TS:        r.TS,
		Direction: r.Direction,
		Kind:      r.Kind,
		Raw:       r.Raw,
	}
	if r.JSONRPCID.Valid {
		v := r.JSONRPCID.String
		dto.JSONRPCID = &v
	}
	if r.Method.Valid {
		v := r.Method.String
		dto.Method = &v
	}
	return dto
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers already sent — best we can do is log to stderr.
		// This matches stdlib http.Error behavior on late failures.
	}
}

func writeErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

