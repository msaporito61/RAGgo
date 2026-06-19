# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

RAGgo is a full rewrite of the RAG system at `/Users/msaporito/Development/RAG` (Python/FastAPI) targeting a new stack:

| Layer | Technology |
|---|---|
| Frontend | Vite + React + TypeScript + Tailwind CSS + Shadcn/UI + Cult-UI (cult-ui.com) |
| Backend | Go |
| Vector DB | Qdrant |
| MCP Server | Go or Python (same stdio transport as the original) |

The original system is the canonical reference for features and API contracts. Read its `CLAUDE.md` and `README.md` at `/Users/msaporito/Development/RAG/` before implementing any feature.

## Source System Feature Set

All of these must be preserved in RAGgo:

- **Document ingestion**: PDF, DOCX, XLSX, MD, TXT — parse → chunk → embed → upsert into Qdrant
- **Semantic + hybrid search** (vector + BM25 keyword rerank)
- **Streaming RAG chat** (SSE) with conversation sessions and multi-collection context
- **Multi-tenancy**: per-user Qdrant collections named `rag_<username>_<slug>`
- **Collection CRUD**: create, rename, delete; default collection auto-created on first use
- **Document operations**: upload, list (paginated, newest-first), view content, delete, move between collections
- **Web scraping**: fetch URL → chunk → index (`/scrape`)
- **Auth**: JWT Bearer (30 min access + 7 day refresh) and `X-API-Key` for programmatic access
- **User management**: admin role + regular users; per-user API keys stored as bcrypt hashes
- **MCP Server**: 22 tools over stdio — see `/Users/msaporito/Development/RAG/README.md` for the full tool list
- **Admin endpoints**: list all collections, user CRUD, activity/audit log

## Commands

> Commands will be added here as the project is scaffolded.

### Backend (Go)

```bash
go build ./...          # Build all
go test ./...           # Run all unit tests
go test ./... -run TestName    # Run a single test
go vet ./...            # Vet
```

### Frontend (Node)

```bash
cd frontend && npm run dev          # Dev server on :3000
cd frontend && npm run test:run     # Vitest single pass
cd frontend && npm run build        # Type-check + production build
cd frontend && npm run lint         # ESLint
```

### Infrastructure

```bash
docker run -d -p 6333:6333 qdrant/qdrant   # Qdrant for local dev
docker compose up -d                        # Full stack
docker compose logs -f api                  # API logs
```

## Architecture

### Backend (Go)

Structure the Go backend following clean layering:

```
cmd/server/          — main entry point, wires everything
internal/
  api/               — HTTP handlers (Gin or Chi), thin layer
  services/          — business logic (chat, search, embed, collection, document)
  vectorstore/       — Qdrant client wrapper
  database/          — SQLite (users, collections, audit log) via sqlc or GORM
  config/            — config struct loaded from env
  middleware/         — JWT auth, API key auth, rate limit, SSRF guard
  mcp/               — MCP server (stdio transport)
```

Key invariants from the original system:
- All AI services (embed, search, chat) should be initialized once at startup and injected — no per-request construction.
- Qdrant collection names: `rag_<username>_<slug>` (max 128 chars). Slugs are derived from `display_name`.
- JWT has priority over `X-API-Key` when both are present.
- Public routes (no auth): `/health`, `/auth/login`, `/auth/refresh`, Swagger/OpenAPI.
- Admin user seeded from env on first startup; subsequent runs skip if user exists.
- Document list is always sorted by `uploaded_at` DESC before pagination.
- Multi-collection chat: when no collection is selected, search all of the user's collections; merge results by score, deduplicate by text fingerprint.
- Chat prompt injects a document index (all user docs) separate from retrieved chunks so the LLM knows what exists.

### Frontend (React + Vite + TypeScript)

```
frontend/src/
  lib/api.ts          — single API client, all fetch calls, JWT injection, auto-refresh on 401
  types/api.ts        — TypeScript interfaces matching backend response shapes
  hooks/useAuth.tsx   — AuthContext
  components/         — Shadcn/UI + Cult-UI components
  pages/              — Dashboard, Documents, Search, Chat, Collections, Settings, Login, Help, QdrantDashboard
  routes/index.tsx    — React Router + ProtectedRoute
```

Use Shadcn/UI for standard components. Use Cult-UI for visual flourish/animation where appropriate.

All API calls route through `fetchWithAuth()` which auto-retries with a refreshed token on 401.

### MCP Server

Expose the same 22 tools as the original (`mcp_server.py`). Authenticate via `RAG_API_KEY` env var. Transport: stdio. Can be a separate binary (`cmd/mcp/`) or a subcommand of the main server.

## Key Configuration (.env)

```env
OPENROUTER_API_KEY=         # LLM provider (OpenRouter)
SECURITY_SECRET_KEY=        # JWT signing secret
SECURITY_ADMIN_USERNAME=admin
SECURITY_ADMIN_PASSWORD=
SECURITY_API_KEY=           # Global programmatic access key
QDRANT_HOST=localhost
QDRANT_PORT=6333
DATABASE_URL=./data/rag.db
ENVIRONMENT=development     # or production
PORT=8000
```

## Testing

- All business logic must have unit tests that pass before any manual testing.
- Mock Qdrant and OpenRouter in unit tests (same pattern as the original `tests/conftest.py`).
- Frontend tests use Vitest + jsdom.
- Run backend tests with `go test ./...` and frontend tests with `npm run test:run`.
