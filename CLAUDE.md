# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Qué es este proyecto

RAGgo es una reescritura completa del sistema RAG en `/Users/msaporito/Development/RAG` (Python/FastAPI). La implementación está terminada y funcional. Ver `README.md` para documentación de usuario.

| Capa | Tecnología |
|---|---|
| Frontend | Vite + React + TypeScript + Tailwind CSS + shadcn/ui |
| Backend | Go (Chi router, JWT, modernc.org/sqlite) |
| Base de datos vectorial | Qdrant (cliente gRPC, puerto 6334) |
| MCP Server | Go (mark3labs/mcp-go, stdio) |

## Comandos

### Backend (Go)

```bash
go build ./...                        # Compilar todo
go test ./...                         # Todos los tests
go test ./... -run TestNombre         # Test específico
go vet ./...                          # Vet
make dev                              # Servidor de desarrollo
```

### Frontend (Node)

```bash
cd frontend && npm run dev            # Dev server en :3000
cd frontend && npm run test:run       # Vitest single pass
cd frontend && npm run build          # Type-check + build de producción
```

### Infraestructura

```bash
docker run -d -p 6333:6333 -p 6334:6334 qdrant/qdrant   # Qdrant local
make up                               # Stack completo
make down                             # Detener stack
make logs                             # Logs del backend
make status                           # Estado de contenedores
```

## Arquitectura

### Backend (Go)

```
cmd/server/              — punto de entrada, wiring de todo
cmd/mcp/                 — binario del servidor MCP
internal/
  api/
    handlers/            — manejadores HTTP (uno por dominio)
    router.go            — registro de rutas y middlewares
  services/
    user/                — auth, JWT, usuarios
    collection/          — CRUD colecciones + Qdrant lifecycle
    document/            — loader, chunker, pipeline de ingesta
    embedding/           — cliente HTTP OpenRouter embeddings
    search/              — búsqueda híbrida semántica+BM25
    chat/                — chat RAG con streaming SSE
    vectorstore/         — cliente gRPC Qdrant
  database/              — SQLite: schema SQL embebido, queries parametrizados
  middleware/            — auth JWT/API-key, headers de seguridad, CORS
  mcp/                   — 22 herramientas MCP
  config/                — Config struct cargado desde env
```

**Invariantes clave:**
- Nombres de colecciones Qdrant: `rag_<usuario>_<slug>` (máx. 128 chars). Slug derivado de `display_name`.
- JWT tiene prioridad sobre `X-API-Key` cuando ambos están presentes.
- Rutas públicas (sin auth): `/health`, `/auth/login`, `/auth/refresh`.
- Usuario admin creado desde `.env` al primer arranque; siguientes arranques no hacen nada si ya existe.
- Listado de documentos siempre ordenado por `uploaded_at DESC, rowid DESC`.
- Sin colección seleccionada → buscar en todas las colecciones del usuario; fusionar por score, deduplicar por huella de texto.
- Prompt de chat inyecta índice de documentos + chunks recuperados como contexto del sistema.

### Frontend (React + Vite + TypeScript)

```
frontend/src/
  lib/api.ts          — cliente API único, inyección JWT, auto-refresh en 401
  types/api.ts        — interfaces TypeScript que reflejan respuestas del backend
  hooks/useAuth.tsx   — AuthContext (user, isAdmin, login, logout)
  components/         — componentes shadcn/ui + ProtectedRoute + AdminRoute
  pages/              — Dashboard, Documents, Search, Chat, Collections, Settings, Login, Admin, Help
  routes/index.tsx    — React Router con rutas protegidas
```

Todas las llamadas API van por `fetchWithAuth()` que reintenta con token renovado en 401.

### Servidor MCP

Binario en `cmd/mcp/`. 22 herramientas sobre stdio. Autenticación vía `RAGGO_API_KEY`. URL del backend vía `RAGGO_BASE_URL` (default: `http://localhost:8080`).

## Configuración (.env)

```env
PORT=8080
ENVIRONMENT=development

OPENROUTER_API_KEY=sk-or-...
OPENROUTER_BASE_URL=https://openrouter.ai/api/v1
LLM_MODEL=openai/gpt-4o-mini
EMBEDDING_MODEL=text-embedding-3-small

QDRANT_HOST=localhost
QDRANT_PORT=6334

DATABASE_URL=./data/rag.db

SECURITY_SECRET_KEY=secreto-aleatorio-32-chars
SECURITY_ADMIN_USERNAME=admin
SECURITY_ADMIN_PASSWORD=password-admin
SECURITY_API_KEY=clave-api-global
SECURITY_RATE_LIMIT_PER_MINUTE=60
```

## Tests

- Todo el código de negocio tiene tests unitarios en `*_test.go`.
- Tests de Go usan `:memory:` SQLite y stubs HTTP para Qdrant/OpenRouter (sin llamadas reales).
- Tests de frontend usan Vitest + jsdom + Testing Library.
- Correr `go test ./...` antes de cualquier prueba manual.
- El test `TestScraper_BlocksPrivateIP` verifica protección SSRF con subtests de IPs privadas y loopback.
