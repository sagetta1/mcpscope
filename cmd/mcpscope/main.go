// mcpscope — transparent proxy + DevTools-style UI for Model Context Protocol.
//
// v0.0.2-dev: stdio proxy + JSON-RPC parsing + SQLite persistence + sessions
// CLI. Web UI lands in week 2 per the roadmap in docs/.
package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/sagetta1/mcpscope/internal/install"
	"github.com/sagetta1/mcpscope/internal/proxy"
	"github.com/sagetta1/mcpscope/internal/storage"
	"github.com/sagetta1/mcpscope/internal/ui"
)

const version = "v0.0.3-dev"

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
		os.Exit(runUI(args))
	case "install":
		os.Exit(runInstall(args))
	case "uninstall":
		os.Exit(runUninstall(args))
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
  mcpscope ui [--port N] [--no-open]       Open web UI on localhost:3939
  mcpscope install [--apply] [--target=desktop|code] [--all]
                                           Wrap MCP servers in Claude config (dry-run by default)
  mcpscope uninstall [--apply] [--target=desktop|code] [--all]
                                           Reverse install (dry-run by default)
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

func runUI(args []string) int {
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	port := fs.Int("port", 3939, "TCP port to listen on (localhost only)")
	noOpen := fs.Bool("no-open", false, "do not open the browser")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	store, err := storage.Open(defaultDBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui: open store: %v\n", err)
		return 1
	}
	defer store.Close()

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ui: listen %s: %v\n", addr, err)
		return 1
	}

	url := fmt.Sprintf("http://%s/", ln.Addr().String())
	fmt.Fprintf(os.Stderr, "mcpscope ui: serving %s (Ctrl+C to stop)\n", url)
	if !*noOpen {
		go openBrowser(url)
	}

	srv := &http.Server{Handler: ui.NewServer(store), ReadHeaderTimeout: 5 * time.Second}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return 0
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "ui: serve: %v\n", err)
			return 1
		}
		return 0
	}
}

// openBrowser tries to open url in the user's default browser. Best-effort —
// failure is silent because users on headless boxes will pass --no-open.
func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default: // linux + bsd
		cmd = "xdg-open"
		args = []string{url}
	}
	_ = exec.Command(cmd, args...).Start()
}

// --- install / uninstall ---

func runInstall(args []string) int {
	return runWrapOrUnwrap(args, "install", install.WrapServer, false)
}

func runUninstall(args []string) int {
	return runWrapOrUnwrap(args, "uninstall", nil, true)
}

// runWrapOrUnwrap is the shared flow for both directions. transformIn
// is the wrap function (install). For uninstall, isUninstall=true and
// install.UnwrapServer is used implicitly.
func runWrapOrUnwrap(
	args []string,
	cmdName string,
	transformIn func(install.Server, string) install.Server,
	isUninstall bool,
) int {
	fs := flag.NewFlagSet(cmdName, flag.ContinueOnError)
	apply := fs.Bool("apply", false, "actually write changes (default: dry-run)")
	all := fs.Bool("all", false, "skip per-server prompt; act on every applicable server")
	targetStr := fs.String("target", "desktop", "which client config to edit: desktop or code")
	binPath := fs.String("mcpscope-path", "", "override the mcpscope binary path written into the config (default: this binary)")
	configPath := fs.String("config-path", "", "override the JSON config path (advanced/testing)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	target := install.Target(*targetStr)
	if target != install.TargetDesktop && target != install.TargetCode {
		fmt.Fprintf(os.Stderr, "%s: --target must be 'desktop' or 'code', got %q\n", cmdName, *targetStr)
		return 2
	}

	var path string
	if *configPath != "" {
		path = *configPath
	} else {
		var err error
		path, err = install.ConfigPath(target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", cmdName, err)
			return 1
		}
	}

	mcpscopePath, err := install.CurrentMcpscopePath(*binPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: locate mcpscope binary: %v\n", cmdName, err)
		return 1
	}

	cfg, err := install.ReadConfig(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "%s: %s does not exist. Is the client installed and run at least once?\n", cmdName, path)
			return 1
		}
		fmt.Fprintf(os.Stderr, "%s: read %s: %v\n", cmdName, path, err)
		return 1
	}

	servers, err := cfg.Servers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmdName, err)
		return 1
	}

	if len(servers) == 0 {
		fmt.Fprintf(os.Stderr, "%s: no MCP servers in %s. Add one in %s first.\n", cmdName, path, target)
		return 0
	}

	// Snapshot original bytes for diff output later.
	beforeBytes, _ := cfg.Bytes()

	// Decide what to change for each server.
	changed := false
	reader := bufio.NewReader(os.Stdin)
	for name, s := range servers {
		var newServer install.Server
		var act bool

		if isUninstall {
			unwrapped, ok := install.UnwrapServer(s, mcpscopePath)
			if !ok {
				fmt.Fprintf(os.Stderr, "  [%s] not wrapped — skip\n", name)
				continue
			}
			act = *all || promptYN(reader, fmt.Sprintf("  [%s] unwrap (was: %s wrap -- %s ...)? [y/N] ", name, mcpscopePath, unwrapped.Command))
			newServer = unwrapped
		} else {
			if install.IsWrapped(s, mcpscopePath) {
				fmt.Fprintf(os.Stderr, "  [%s] already wrapped — skip\n", name)
				continue
			}
			act = *all || promptYN(reader, fmt.Sprintf("  [%s] wrap (command=%s)? [y/N] ", name, s.Command))
			newServer = transformIn(s, mcpscopePath)
		}

		if !act {
			continue
		}
		servers[name] = newServer
		changed = true
	}

	if !changed {
		fmt.Fprintf(os.Stderr, "%s: nothing to do.\n", cmdName)
		return 0
	}

	if err := cfg.SetServers(servers); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmdName, err)
		return 1
	}
	afterBytes, err := cfg.Bytes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cmdName, err)
		return 1
	}

	// Print diff (always, even at apply — the user wants a final receipt).
	fmt.Fprintln(os.Stderr)
	install.PrintUnifiedDiff(os.Stderr, path, path+" (proposed)", string(beforeBytes), string(afterBytes), isatty(os.Stderr))
	fmt.Fprintln(os.Stderr)

	if !*apply {
		fmt.Fprintln(os.Stderr, "Dry-run only. Re-run with --apply to write the change.")
		return 0
	}

	backupPath, err := install.BackupConfig(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: backup failed: %v\n", cmdName, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "Backup written: %s\n", backupPath)

	if err := install.WriteConfigAtomic(path, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write failed (backup is at %s): %v\n", cmdName, backupPath, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "Wrote %s. Restart the client for the change to take effect.\n", path)
	fmt.Fprintf(os.Stderr, "Revert any time: mv %s %s\n", backupPath, path)
	return 0
}

func promptYN(r *bufio.Reader, prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	line, err := r.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}

func isatty(f *os.File) bool {
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
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
