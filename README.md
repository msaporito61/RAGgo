# RAGgo

Sistema RAG (Retrieval-Augmented Generation) construido en Go + React. Reescritura completa del sistema original en Python/FastAPI, con el mismo conjunto de funcionalidades y una arquitectura más robusta.

## Tecnologías

| Capa | Tecnología |
|---|---|
| Backend | Go (Chi router, JWT, modernc.org/sqlite) |
| Base de datos vectorial | Qdrant (cliente gRPC) |
| Frontend | Vite + React + TypeScript + Tailwind CSS + shadcn/ui |
| LLM / Embeddings | OpenRouter (compatible con API de OpenAI) |
| MCP Server | Go (mark3labs/mcp-go, transporte stdio) |

## Funcionalidades

- **Ingesta de documentos**: PDF, DOCX, XLSX, MD, TXT → fragmentación → embeddings → Qdrant
- **Búsqueda híbrida**: semántica (70%) + BM25 keyword (30%)
- **Chat RAG con streaming**: SSE, sesiones de conversación, contexto multi-colección
- **Multi-tenancy**: colecciones Qdrant por usuario (`rag_<usuario>_<slug>`)
- **Gestión de colecciones**: crear, eliminar; colección por defecto auto-creada
- **Gestión de documentos**: subir, listar (paginado), ver, eliminar, mover entre colecciones
- **Web scraping**: fetch de URL → fragmentación → indexado con protección SSRF
- **Autenticación**: JWT Bearer (30 min acceso + 7 días refresh) y `X-API-Key`
- **Gestión de usuarios**: rol admin + usuarios regulares; claves API por usuario
- **MCP Server**: 22 herramientas sobre stdio para integraciones con IA
- **Endpoints de administración**: listar todas las colecciones, CRUD de usuarios, clave API

## Requisitos previos

- Go 1.22+
- Node.js 20+
- Docker y Docker Compose (para el stack completo)
- Cuenta en [OpenRouter](https://openrouter.ai) para LLM y embeddings

## Inicio rápido con Docker

```bash
# 1. Copiar y configurar variables de entorno
cp .env.example .env
# Editar .env con tus claves reales

# 2. Levantar el stack completo
make up

# 3. Verificar que todo está corriendo
make status
```

Servicios disponibles:
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080
- Qdrant: http://localhost:6333

## Desarrollo local

### Backend

```bash
# Iniciar Qdrant
docker run -d -p 6333:6333 -p 6334:6334 qdrant/qdrant

# Copiar y editar variables de entorno
cp .env.example .env

# Compilar y ejecutar
make dev

# Ejecutar tests
go test ./...

# Compilar
go build ./...
```

### Frontend

```bash
cd frontend

# Instalar dependencias
npm install

# Servidor de desarrollo (http://localhost:3000)
npm run dev

# Tests
npm run test:run

# Compilación de producción
npm run build
```

### MCP Server

```bash
# Compilar el binario MCP
go build -o bin/raggo-mcp ./cmd/mcp/

# Configurar en tu cliente MCP (ej. Claude Desktop)
# Variables de entorno necesarias:
# RAGGO_API_KEY=tu-clave-api
# RAGGO_BASE_URL=http://localhost:8080
```

## Configuración (.env)

```env
PORT=8080
ENVIRONMENT=development          # o production

# LLM (OpenRouter)
OPENROUTER_API_KEY=sk-or-...
OPENROUTER_BASE_URL=https://openrouter.ai/api/v1
LLM_MODEL=openai/gpt-4o-mini
EMBEDDING_MODEL=text-embedding-3-small

# Qdrant (gRPC)
QDRANT_HOST=localhost
QDRANT_PORT=6334

# Base de datos SQLite
DATABASE_URL=./data/rag.db

# Seguridad
SECURITY_SECRET_KEY=secreto-random-de-32-caracteres
SECURITY_ADMIN_USERNAME=admin
SECURITY_ADMIN_PASSWORD=tu-password-admin
SECURITY_API_KEY=clave-api-global
SECURITY_RATE_LIMIT_PER_MINUTE=60
```

## Arquitectura del backend

```
cmd/
  server/          — punto de entrada principal
  mcp/             — binario del servidor MCP
internal/
  api/
    handlers/      — manejadores HTTP (auth, collections, documents, search, chat, admin, scrape)
    router.go      — wiring de rutas y middlewares
  services/
    user/          — autenticación, JWT, gestión de usuarios
    collection/    — CRUD de colecciones + ciclo de vida en Qdrant
    document/      — carga de archivos, chunker, pipeline de ingesta
    embedding/     — cliente HTTP para embeddings (OpenRouter)
    search/        — búsqueda híbrida semántica + BM25
    chat/          — servicio RAG con streaming SSE
    vectorstore/   — cliente gRPC de Qdrant
  database/        — SQLite: schema, migraciones, queries
  middleware/      — autenticación JWT/API key, seguridad, CORS
  mcp/             — 22 herramientas MCP
  config/          — configuración desde variables de entorno
```

**Invariantes clave:**
- Nombres de colecciones Qdrant: `rag_<usuario>_<slug>` (máx. 128 chars)
- JWT tiene prioridad sobre `X-API-Key` cuando ambos están presentes
- Rutas públicas (sin auth): `/health`, `/auth/login`, `/auth/refresh`
- Usuario admin creado desde `.env` al primer arranque
- Listado de documentos siempre ordenado por `uploaded_at DESC`
- Sin colección seleccionada en chat/búsqueda: usa todas las colecciones del usuario

## Arquitectura del frontend

```
frontend/src/
  lib/api.ts       — cliente API tipado, inyección JWT, auto-refresh en 401
  types/api.ts     — interfaces TypeScript que reflejan las respuestas del backend
  hooks/useAuth.tsx — AuthContext con login/logout/isAdmin
  components/      — componentes shadcn/ui + AdminRoute/ProtectedRoute
  pages/           — Collections, Documents, Search, Chat, Login, Admin, Help
  routes/index.tsx — React Router + rutas protegidas
```

## API REST

### Autenticación
| Método | Ruta | Descripción |
|---|---|---|
| POST | /auth/login | Login con usuario y contraseña |
| POST | /auth/refresh | Renovar token de acceso |

### Colecciones
| Método | Ruta | Descripción |
|---|---|---|
| GET | /collections | Listar colecciones del usuario |
| POST | /collections | Crear colección |
| DELETE | /collections/{slug} | Eliminar colección |

### Documentos
| Método | Ruta | Descripción |
|---|---|---|
| GET | /documents | Listar documentos (paginado) |
| POST | /documents/upload | Subir documento (multipart) |
| DELETE | /documents/{id} | Eliminar documento |
| PATCH | /documents/{id}/move | Mover a otra colección |
| POST | /documents/scrape | Indexar URL web |

### Búsqueda y Chat
| Método | Ruta | Descripción |
|---|---|---|
| GET | /search | Búsqueda híbrida (`?q=...&collection_slug=...`) |
| POST | /chat/sessions | Crear sesión de chat |
| GET | /chat/sessions | Listar sesiones |
| DELETE | /chat/sessions/{id} | Eliminar sesión |
| POST | /chat/sessions/{id}/message | Enviar mensaje (SSE streaming) |

### Administración (requiere rol admin)
| Método | Ruta | Descripción |
|---|---|---|
| GET | /admin/collections | Listar todas las colecciones del sistema |
| GET | /admin/users | Listar usuarios |
| POST | /admin/users | Crear usuario |
| DELETE | /admin/users/{username} | Eliminar usuario |
| PUT | /admin/users/{username}/password | Cambiar contraseña |

## Herramientas MCP

El servidor MCP expone 22 herramientas sobre stdio:

`health_check`, `list_collections`, `create_collection`, `delete_collection`, `list_documents`, `upload_document`, `delete_document`, `move_document`, `scrape_url`, `search`, `create_chat_session`, `list_chat_sessions`, `delete_chat_session`, `chat`, `login`, `refresh_token`, `list_users`, `create_user`, `delete_user`, `set_user_password`, `set_api_key`, `list_all_collections`

Configuración para Claude Desktop (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "raggo": {
      "command": "/ruta/a/bin/raggo-mcp",
      "env": {
        "RAGGO_API_KEY": "tu-clave-api",
        "RAGGO_BASE_URL": "http://localhost:8080"
      }
    }
  }
}
```

## Comandos disponibles

```bash
make dev        # Ejecutar backend en modo desarrollo
make build      # Compilar binario del servidor
make test       # Ejecutar todos los tests Go
make vet        # go vet en todos los paquetes
make up         # Levantar stack Docker completo
make down       # Detener stack Docker
make logs       # Ver logs del contenedor API
make status     # Estado de los contenedores
```

## Tests

```bash
# Backend (Go)
go test ./...                         # todos los tests
go test ./internal/services/chat/...  # package específico
go test ./... -run TestNombre         # test específico

# Frontend
cd frontend
npm run test:run   # vitest single pass
```
