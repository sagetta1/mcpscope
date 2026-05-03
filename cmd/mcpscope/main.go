// mcpscope — transparent proxy + DevTools-style UI for Model Context Protocol.
//
// v0.0.1-dev: skeleton only. The wrap subcommand spawns the target binary and
// pipes stdio transparently; JSON-RPC parsing, SQLite persistence, and the web
// UI land in subsequent commits per the roadmap in docs/.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

const version = "v0.0.1-dev"

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}

	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "wrap":
		os.Exit(runWrap(args))
	case "ui":
		fmt.Fprintln(os.Stderr, "ui: not implemented yet — coming in week 2")
		os.Exit(1)
	case "install":
		fmt.Fprintln(os.Stderr, "install: not implemented yet — coming in week 3")
		os.Exit(1)
	case "sessions":
		fmt.Fprintln(os.Stderr, "sessions: not implemented yet — coming in week 1")
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
  mcpscope ui                              Open web UI on localhost:3939 (planned)
  mcpscope install                         Auto-detect and patch Claude Desktop config (planned)
  mcpscope sessions                        List recorded sessions (planned)
  mcpscope version                         Show version
  mcpscope help                            Show this help

Examples:
  mcpscope wrap -- npx -y @modelcontextprotocol/server-filesystem /tmp
  mcpscope wrap -- python3 my_mcp_server.py

Project: https://github.com/sagetta1/mcpscope`)
}

// runWrap spawns the target command with stdio attached. v0.0.1 is a pure
// passthrough; JSON-RPC capture lands once the proxy package exists.
func runWrap(args []string) int {
	if len(args) < 2 || args[0] != "--" {
		fmt.Fprintln(os.Stderr, "wrap: usage: mcpscope wrap -- <command> [args...]")
		return 2
	}

	target := exec.Command(args[1], args[2:]...)
	target.Stdin = os.Stdin
	target.Stdout = os.Stdout
	target.Stderr = os.Stderr

	if err := target.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "wrap: failed to run %q: %v\n", args[1], err)
		return 1
	}
	return 0
}
