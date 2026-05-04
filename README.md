# mcpscope

> Chrome DevTools for the Model Context Protocol.

A transparent proxy that sits between an MCP client (Claude Desktop, Cursor,
Continue) and an MCP server, capturing JSON-RPC traffic and exposing it through
a local web UI: timeline, request/response diff, replay, token cost overlay.

**Status:** pre-alpha (`v0.0.3-dev`). The stdio proxy captures, classifies,
and persists JSON-RPC traffic to SQLite. CLI inspection works (`sessions`,
`show <id>`). Web UI on `localhost:3939` is wired up — sessions list,
timeline, request/response detail, JSON diff, and a live SSE feed are
shipped. See the roadmap before trying to use this in anger.

## Why

When you debug an MCP server connected to a real client, you're flying blind:
the client shows the model's final answer, but the JSON-RPC traffic between the
client and your server is invisible. Existing tooling either runs as its own
client (Anthropic's MCP Inspector, MCPJam) or instruments the application
through an SDK (Langfuse, Helicone, Phoenix). None of them passively capture
real client traffic.

mcpscope sits in the path. You point Claude Desktop's config at `mcpscope wrap`
instead of your server binary, and every JSON-RPC message is recorded with no
changes to the client or the server.

## Roadmap

- **Week 1** — ✅ stdio proxy + JSON-RPC parsing + SQLite persistence + `sessions` / `show` CLI
- **Week 2** — ✅ React + Vite UI embedded in the binary; five screens (sessions list, timeline, message detail, diff, live SSE feed)
- **Week 3** — `mcpscope install` auto-detects Claude Desktop config; tiktoken-based cost overlay; GoReleaser cross-platform builds
- **Week 4** — `v0.1.0` release, landing page, first design-partner outreach

## Build from source

Requires Go 1.24+ and Node 20+. The Makefile bundles the React UI into the
Go binary via `embed.FS` — one binary, no runtime deps.

```sh
git clone https://github.com/sagetta1/mcpscope
cd mcpscope
make build              # cd internal/ui/frontend && npm install && npm run build, then go build
./mcpscope wrap -- npx -y @modelcontextprotocol/server-filesystem /tmp
./mcpscope sessions     # CLI list
./mcpscope ui           # opens http://localhost:3939 in your browser
```

Captures land in `~/.mcpscope/sessions.db` (SQLite, WAL mode).

For frontend dev with hot reload, run two terminals:

```sh
./mcpscope ui --port 3939 --no-open       # backend
cd internal/ui/frontend && npm run dev    # vite proxies /api → :3939
```

## License

Apache 2.0. See [LICENSE](LICENSE).
