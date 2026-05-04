.PHONY: build frontend backend clean dev test

# Default: full build (frontend bundle + Go binary with embedded UI).
build: frontend backend

# Frontend: install deps if missing, then production build into
# internal/ui/frontend/dist (consumed by embed.go via //go:embed).
frontend:
	@if [ ! -d internal/ui/frontend/node_modules ]; then \
		echo "==> npm install"; \
		cd internal/ui/frontend && npm install; \
	fi
	@echo "==> npm run build"
	@cd internal/ui/frontend && npm run build

# Backend: builds the single Go binary. Requires frontend/dist to exist
# (otherwise //go:embed all:frontend/dist fails at compile time).
backend:
	@echo "==> go build"
	@go build -o mcpscope ./cmd/mcpscope
	@echo "==> ./mcpscope $(shell ./mcpscope version)"

dev:
	@echo "Run two terminals:"
	@echo "  1) ./mcpscope ui --port 3939 --no-open"
	@echo "  2) cd internal/ui/frontend && npm run dev"
	@echo "Then open http://localhost:5173 (vite proxies /api → :3939)"

test:
	@go test ./...

clean:
	@rm -f mcpscope
	@rm -rf internal/ui/frontend/dist
	@echo "==> cleaned"
