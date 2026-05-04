// mcpscope — transparent proxy + DevTools-style UI for Model Context Protocol.
//
// v0.0.2-dev: stdio proxy + JSON-RPC parsing + SQLite persistence + sessions
// CLI. Web UI lands in week 2 per the roadmap in docs/.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sagetta1/mcpscope/internal/proxy"
	"github.com/sagetta1/mcpscope/internal/storage"
)

const version = "v0.0.2-dev"

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}

	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "wrap":
		os.Exit(runWrap(args))
	case "sessions":
		os.Exit(runSessions())
	case "show":
		os.Exit(runShow(args))
	case "ui":
		fmt.Fprintln(os.Stderr, "ui: not implemented yet — coming in week 2")
		os.Exit(1)
	case "install":
		fmt.Fprintln(os.Stderr, "install: not implemented yet — coming in week 3")
		os.Exit(1)
	case "version", "-v", "--version":
		fmt.Println("mcpscope", version)
	case "help", "-h", "--help":
		usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w *os.File) {
	fmt.Fprintln(w, `mcpscope — Chrome DevTools for the Model Context Protocol

Usage:
  mcpscope wrap -- <command> [args...]    Run command as MCP server, capture JSON-RPC traffic
  mcpscope sessions                        List recorded sessions
  mcpscope show <session_id>               Show messages from a session
  mcpscope ui                              Open web UI on localhost:3939 (planned)
  mcpscope install                         Auto-detect and patch Claude Desktop config (planned)
  mcpscope version                         Show version
  mcpscope help                            Show this help

Examples:
  mcpscope wrap -- npx -y @modelcontextprotocol/server-filesystem /tmp
  mcpscope wrap -- python3 my_mcp_server.py
  mcpscope sessions
  mcpscope show s_1234567890

Storage: ~/.mcpscope/sessions.db (SQLite, WAL mode)
Project: https://github.com/sagetta1/mcpscope`)
}

// defaultDBPath returns the v0 SQLite path. Configurable in a later release.
func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fall back to the working directory; better than crashing.
		return filepath.Join(".", ".mcpscope", "sessions.db")
	}
	return filepath.Join(home, ".mcpscope", "sessions.db")
}

func runWrap(args []string) int {
	if len(args) < 2 || args[0] != "--" {
		fmt.Fprintln(os.Stderr, "wrap: usage: mcpscope wrap -- <command> [args...]")
		return 2
	}

	store, err := storage.Open(defaultDBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "wrap: open store: %v\n", err)
		return 1
	}
	defer store.Close()

	target := args[1]
	targetArgs := args[2:]
	w := &proxy.Wrap{
		Store:     store,
		SessionID: proxy.NewSessionID(),
		TargetCmd: strings.Join(args[1:], " "),
	}
	fmt.Fprintf(os.Stderr, "mcpscope: capturing session %s\n", w.SessionID)

	// MCP clients (Claude Desktop, Claude Code) terminate the wrap process
	// with SIGTERM when they restart or shut down. Catch it so subprocess
	// shutdown propagates cleanly through cmd.Wait → defer EndSession,
	// otherwise sessions stay marked (running) forever in the timeline.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	code, err := w.Run(ctx, target, targetArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wrap: %v\n", err)
		return 1
	}
	return code
}

func runSessions() int {
	store, err := storage.Open(defaultDBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "sessions: open store: %v\n", err)
		return 1
	}
	defer store.Close()

	rows, err := store.ListSessions(context.Background(), 50)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sessions: query: %v\n", err)
		return 1
	}
	if len(rows) == 0 {
		fmt.Println("no sessions yet — run `mcpscope wrap -- <command>` first")
		return 0
	}
	fmt.Printf("%-22s  %-19s  %-19s  %5s  %s\n", "SESSION", "STARTED", "ENDED", "MSGS", "COMMAND")
	for _, r := range rows {
		started := time.UnixMilli(r.StartedAt).Format("2006-01-02 15:04:05")
		ended := "(running)"
		if r.EndedAt.Valid {
			ended = time.UnixMilli(r.EndedAt.Int64).Format("2006-01-02 15:04:05")
		}
		fmt.Printf("%-22s  %-19s  %-19s  %5d  %s\n", r.ID, started, ended, r.MsgCount, truncate(r.TargetCmd, 60))
	}
	return 0
}

func runShow(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "show: usage: mcpscope show <session_id>")
		return 2
	}
	store, err := storage.Open(defaultDBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "show: open store: %v\n", err)
		return 1
	}
	defer store.Close()

	msgs, err := store.ListMessages(context.Background(), args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "show: query: %v\n", err)
		return 1
	}
	if len(msgs) == 0 {
		fmt.Fprintf(os.Stderr, "show: no messages for session %s\n", args[0])
		return 1
	}
	for _, m := range msgs {
		ts := time.UnixMilli(m.TS).Format("15:04:05.000")
		fmt.Printf("%s  %-3s  %-12s  id=%-8s method=%-30s  %s\n",
			ts, m.Direction, m.Kind, nullStr(m.JSONRPCID), nullStr(m.Method), truncate(m.Raw, 200))
	}
	return 0
}

func nullStr(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return "-"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
