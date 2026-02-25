# Quinfi Development Guide

Quinfi is an AI-powered fund investment research assistant (AI 基金投研助手) for Chinese mutual fund investors. See `README.md` for project structure and quick-start commands.

## Cursor Cloud specific instructions

### System prerequisites

- **Go 1.25+** is required (installed at `/usr/local/go/bin`; ensure `PATH` includes it).
- **Node.js 22+** and npm for the React frontend (`web/`).

### Running services

| Service | Command | Port | Notes |
|---------|---------|------|-------|
| Go backend | `make run` (from repo root) | 8080 | Requires `config.yaml` (copy from `config.example.yaml` if missing) |
| React frontend | `cd web && npm run dev` | 5173 | Vite dev server; proxies `/api` to backend on 8080 |

### Key caveats

- The backend starts RSS feed fetching and LLM summarization immediately on boot. Without a configured LLM endpoint (`llm.base_url` in `config.yaml` or `LLM_BASE_URL` env), you will see repeated connection-refused warnings in logs—these are harmless and do not prevent the server from handling non-AI API requests (fund data, news, quant endpoints).
- `config.yaml` is gitignored. If it doesn't exist, copy from `config.example.yaml` before starting the backend.
- SQLite database (`data/quinfi.db`) is auto-created on first run. No external database needed.
- `npm run build` in `web/` has pre-existing TypeScript errors (`tsc -b` fails). The Vite dev server (`npm run dev`) works fine since it skips strict type-checking.

### Useful commands (see also `Makefile`)

- **Lint**: `make lint` (runs `go vet`; golangci-lint optional), `cd web && npm run lint` (ESLint)
- **Test**: `make test` (55 Go tests across agent, config, db, debate, memory, quant packages)
- **Build**: `make build` (produces `bin/quinfi`)
