// Package proxy implements the stdio transparent proxy between an MCP
// client (parent process stdin/stdout) and an MCP server (subprocess).
//
// The MCP stdio transport is newline-delimited JSON: each JSON-RPC message
// is one line, no embedded newlines, no Content-Length framing. We forward
// every line verbatim with no mutation, then parse + persist on the side.
//
// Forwarding is on the hot path; persistence runs in a background goroutine
// fed via a buffered channel so a slow disk never adds latency to the
// client→server round-trip.
package proxy

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/sagetta1/mcpscope/internal/storage"
)

// Max line size (10 MiB). MCP tool results can be large (file dumps,
// search results) so the default 64 KiB scanner buffer is not enough.
const maxLineBytes = 10 * 1024 * 1024

// NewSessionID returns a sortable, collision-free id rooted in the wall
// clock. Format: s_<unix_nanos>. Collisions inside one nanosecond on one
// machine are not a concern for v0.
func NewSessionID() string {
	return fmt.Sprintf("s_%d", time.Now().UnixNano())
}

// Wrap is one capture session: a target subprocess + the store it writes to.
type Wrap struct {
	Store     *storage.Store
	SessionID string
	TargetCmd string // human-readable joined cmdline, stored on the session row

	// captured is buffered so the hot pump goroutines never block on disk.
	captured chan capturedMessage
}

type capturedMessage struct {
	tsMs      int64
	direction string
	line      []byte
}

// Run starts the target subprocess, opens stdin/stdout pipes, and pumps
// bytes in both directions. Blocks until the subprocess exits and all
// buffered messages are persisted. Returns the subprocess exit code.
func (w *Wrap) Run(ctx context.Context, command string, args []string) (int, error) {
	startMs := time.Now().UnixMilli()
	if err := w.Store.StartSession(w.SessionID, w.TargetCmd, startMs); err != nil {
		return 1, fmt.Errorf("start session: %w", err)
	}
	defer func() {
		_ = w.Store.EndSession(w.SessionID, time.Now().UnixMilli())
	}()

	w.captured = make(chan capturedMessage, 1024)

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return 1, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("start subprocess: %w", err)
	}

	// Persistence consumer.
	persistDone := make(chan struct{})
	go func() {
		defer close(persistDone)
		w.persistLoop()
	}()

	// in-pump is detached: when subprocess exits, this goroutine may still
	// be blocked in scanner.Scan() reading os.Stdin (clients keep stdin
	// open for the lifetime of the connection). We let the OS reap it on
	// process exit rather than try to interrupt a blocking syscall.
	go func() {
		defer stdin.Close()
		w.pump(os.Stdin, stdin, "in")
	}()

	// We wait on the out-pump: when subprocess closes its stdout (i.e.
	// exits or shuts down), Scanner returns and we know the session is
	// over from the server's perspective.
	var outWG sync.WaitGroup
	outWG.Add(1)
	go func() {
		defer outWG.Done()
		w.pump(stdout, os.Stdout, "out")
	}()
	outWG.Wait()

	waitErr := cmd.Wait()

	// Drain remaining captures, then wait for persistence loop to finish.
	close(w.captured)
	<-persistDone

	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, fmt.Errorf("wait subprocess: %w", waitErr)
	}
	return 0, nil
}

// pump forwards src→dst line-by-line and emits a capture for each
// non-empty line. Forwarding happens before classification so capture
// never delays the client.
func (w *Wrap) pump(src io.Reader, dst io.Writer, direction string) {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	for scanner.Scan() {
		raw := scanner.Bytes()
		// Forward synchronously, line then newline. Two writes is fine
		// for a pipe; the OS will not interleave with the other direction.
		if _, err := dst.Write(raw); err != nil {
			fmt.Fprintf(os.Stderr, "mcpscope: forward %s: %v\n", direction, err)
			return
		}
		if _, err := dst.Write([]byte{'\n'}); err != nil {
			return
		}

		// Hand a copy to the persistence loop. scanner.Bytes() reuses its
		// underlying buffer on the next Scan, so we MUST copy.
		cp := make([]byte, len(raw))
		copy(cp, raw)
		select {
		case w.captured <- capturedMessage{
			tsMs:      time.Now().UnixMilli(),
			direction: direction,
			line:      cp,
		}:
		default:
			// Disk fell behind by more than 1024 messages. Drop and warn
			// rather than block the proxy hot path.
			fmt.Fprintln(os.Stderr, "mcpscope: capture buffer full, dropping message")
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "mcpscope: read %s: %v\n", direction, err)
	}
}

func (w *Wrap) persistLoop() {
	for msg := range w.captured {
		kind, id, method := classify(msg.line)
		if err := w.Store.AppendMessage(w.SessionID, msg.direction, kind, id, method, string(msg.line), msg.tsMs); err != nil {
			fmt.Fprintf(os.Stderr, "mcpscope: persist: %v\n", err)
		}
	}
}

// classify parses a JSON-RPC line minimally — only enough to label it and
// pull out id+method for indexing. The raw bytes are stored as-is, so the
// UI/CLI can re-parse for full detail without our help.
func classify(line []byte) (kind, id, method string) {
	trimmed := strings.TrimSpace(string(line))
	if trimmed == "" {
		return "invalid", "", ""
	}
	var msg map[string]json.RawMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return "invalid", "", ""
	}
	if v, ok := msg["id"]; ok && string(v) != "null" {
		id = strings.Trim(string(v), `"`)
	}
	if v, ok := msg["method"]; ok {
		s := string(v)
		// method must be a JSON string per JSON-RPC 2.0
		if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
			method = s[1 : len(s)-1]
		}
	}
	switch {
	case method != "" && id != "":
		return "request", id, method
	case method != "" && id == "":
		return "notification", "", method
	case method == "" && id != "":
		return "response", id, ""
	default:
		return "invalid", id, method
	}
}
