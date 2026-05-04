// Package install reads, modifies, and writes the JSON configs that
// Claude Desktop and Claude Code use to declare MCP servers. The whole
// point is to wrap each server's command with `mcpscope wrap -- ...`
// while preserving every other field byte-for-byte semantically — so
// the user can run `mcpscope install`, restart their client, and have
// every MCP call captured to ~/.mcpscope/sessions.db without thinking
// about it.
//
// Safety contract:
//   - dry-run by default (caller decides whether to write)
//   - backup ALWAYS before write, to <path>.before-mcpscope-<UTC ISO>
//   - atomic write: marshal → write to .tmp → os.Rename
//
// JSON ordering: Go's encoding/json sorts map keys alphabetically. We
// accept that for v0 — the diff is still readable and the backup is
// the source of truth for "what it looked like before."
package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Target picks which client's config we're editing.
type Target string

const (
	TargetDesktop Target = "desktop" // Claude Desktop app
	TargetCode    Target = "code"    // Claude Code CLI / VSCode ext
)

// Server is one MCP server entry. Mirrors the shape both Claude Desktop
// and Claude Code use; Type is optional (Claude Code sets "stdio" for
// stdio servers, Claude Desktop omits it).
type Server struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// Config is the typed view of the on-disk JSON. Raw preserves any
// top-level keys we don't touch (preferences, customApiKeyResponses, etc.)
// so re-serialization doesn't drop them.
type Config struct {
	Raw map[string]json.RawMessage
}

// ConfigPath returns the on-disk location for the given target.
func ConfigPath(t Target) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch t {
	case TargetDesktop:
		switch runtime.GOOS {
		case "darwin":
			return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
		case "windows":
			appdata := os.Getenv("APPDATA")
			if appdata == "" {
				return "", errors.New("APPDATA not set")
			}
			return filepath.Join(appdata, "Claude", "claude_desktop_config.json"), nil
		default:
			return "", fmt.Errorf("Claude Desktop not available on %s", runtime.GOOS)
		}
	case TargetCode:
		// Same on every platform.
		return filepath.Join(home, ".claude.json"), nil
	default:
		return "", fmt.Errorf("unknown target %q", t)
	}
}

// ReadConfig parses the JSON file at path. Missing file is a sentinel
// error (os.ErrNotExist) so callers can distinguish "not installed"
// from "broken JSON."
func ReadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		// Treat truly empty file as empty config — same as missing.
		return &Config{Raw: map[string]json.RawMessage{}}, nil
	}
	c := &Config{Raw: map[string]json.RawMessage{}}
	if err := json.Unmarshal(body, &c.Raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return c, nil
}

// Servers returns the parsed mcpServers map. Empty map (not nil) when
// the field is missing, so callers can iterate without nil checks.
func (c *Config) Servers() (map[string]Server, error) {
	raw, ok := c.Raw["mcpServers"]
	if !ok || len(raw) == 0 {
		return map[string]Server{}, nil
	}
	out := map[string]Server{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse mcpServers: %w", err)
	}
	return out, nil
}

// SetServers replaces the mcpServers field. Pass an empty map to set
// "mcpServers": {} (not the same as deleting the key, but close enough
// for our needs).
func (c *Config) SetServers(servers map[string]Server) error {
	raw, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	c.Raw["mcpServers"] = raw
	return nil
}

// Bytes returns the formatted JSON. Stable indentation (2 spaces) so
// the diff vs. backup is meaningful.
func (c *Config) Bytes() ([]byte, error) {
	return json.MarshalIndent(c.Raw, "", "  ")
}

// WrapServer returns a new Server whose command is mcpscopePath and
// whose args are ["wrap", "--", origCommand, origArgs...]. Env is
// preserved verbatim. Type is preserved (Claude Code keeps "stdio").
func WrapServer(s Server, mcpscopePath string) Server {
	newArgs := make([]string, 0, len(s.Args)+3)
	newArgs = append(newArgs, "wrap", "--", s.Command)
	newArgs = append(newArgs, s.Args...)
	return Server{
		Type:    s.Type,
		Command: mcpscopePath,
		Args:    newArgs,
		Env:     s.Env,
	}
}

// UnwrapServer reverses WrapServer. Returns the inner Server and true
// if the input looked wrapped; otherwise returns input unchanged and false.
func UnwrapServer(s Server, mcpscopePath string) (Server, bool) {
	if !IsWrapped(s, mcpscopePath) {
		return s, false
	}
	// Layout: command=<mcpscope>, args=["wrap","--",innerCmd, innerArgs...]
	if len(s.Args) < 3 {
		return s, false
	}
	inner := Server{
		Type:    s.Type,
		Command: s.Args[2],
		Args:    append([]string(nil), s.Args[3:]...),
		Env:     s.Env,
	}
	return inner, true
}

// IsWrapped reports whether s was produced by WrapServer.
//
// It checks two things: the command points at any binary literally named
// "mcpscope" (basename), AND the first two args are "wrap" and "--".
// We compare basename rather than full path so the user moving the
// binary doesn't break detection.
func IsWrapped(s Server, mcpscopePath string) bool {
	if filepath.Base(s.Command) != "mcpscope" && s.Command != mcpscopePath {
		return false
	}
	if len(s.Args) < 2 {
		return false
	}
	return s.Args[0] == "wrap" && s.Args[1] == "--"
}

// BackupConfig copies path to <path>.before-mcpscope-<UTC ISO> and
// returns the backup path. No-op + nil if the source doesn't exist.
func BackupConfig(path string) (string, error) {
	src, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer src.Close()

	// Stamp at millisecond precision so back-to-back operations (e.g. an
	// install --apply immediately followed by uninstall --apply during
	// testing) don't collide on O_EXCL. The dot-separated millis read
	// fine to a human and stay sortable.
	now := time.Now().UTC()
	stamp := now.Format("2006-01-02T150405") + fmt.Sprintf(".%03dZ", now.Nanosecond()/int(time.Millisecond))
	dst := fmt.Sprintf("%s.before-mcpscope-%s", path, stamp)
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("create backup: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		_ = os.Remove(dst)
		return "", fmt.Errorf("copy to backup: %w", err)
	}
	return dst, nil
}

// WriteConfigAtomic writes c.Bytes() to <path>.tmp then renames to path.
// Either the old file or the new file exists at all times — no torn
// half-written JSON if the process is killed mid-write.
//
// Caller is responsible for taking a backup first. WriteConfigAtomic
// does NOT do its own backup because callers may want to skip it
// (e.g. tests).
func WriteConfigAtomic(path string, c *Config) error {
	body, err := c.Bytes()
	if err != nil {
		return err
	}
	// Trailing newline — standard for JSON config files.
	body = append(body, '\n')

	tmp := path + ".tmp"
	// 0o600 — config may contain secrets; tighter than typical 0o644.
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open tmp: %w", err)
	}
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("sync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename tmp → final: %w", err)
	}
	return nil
}

// CurrentMcpscopePath returns the absolute path of the running mcpscope
// binary — the path we want to write into the wrapped config. If the
// caller passes an explicit override, that wins (used by tests).
func CurrentMcpscopePath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Clean(strings.TrimSpace(exe)), nil
}
