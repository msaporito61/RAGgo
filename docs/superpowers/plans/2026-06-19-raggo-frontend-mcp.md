# RAGgo Frontend + MCP + Docker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Prerequisite:** Complete `2026-06-19-raggo-backend-foundation.md` and `2026-06-19-raggo-backend-core.md` first. Backend must be running on `:8080`.

**Goal:** Build the React/Vite/TypeScript frontend with Shadcn/UI + Cult-UI, a Go MCP server with 22 tools, and Docker Compose for the full stack.

**Architecture:** Single-page app with React Router 6. All API calls through `fetchWithAuth()` which auto-refreshes JWT on 401. MCP server is a separate Go binary (`cmd/mcp/main.go`) using mark3labs/mcp-go over stdio.

**Tech Stack:** React 18, Vite 5, TypeScript 5, Tailwind CSS 3, Shadcn/UI, Cult-UI, React Router 6, mark3labs/mcp-go, Docker multi-stage builds.

## Global Constraints

- Brand: Sort Boks; copyright: `© 2025–{currentYear} Sort Boks`
- Backend: `http://localhost:8080` in dev; same-origin in prod via nginx proxy
- Frontend dev port: 3000 (Vite proxy `/api/*` → `localhost:8080`, strips `/api` prefix)
- JWT stored in `localStorage` as `access_token` / `refresh_token`
- All non-public routes wrapped in `<ProtectedRoute>`
- Admin-only pages: QdrantDashboard, User Management; check `user.role === "admin"`
- MCP server: stdio transport, `RAG_API_URL` + `RAG_API_KEY` from env

---

### Task 12: Frontend Scaffold

**Files:**
- Create: `frontend/` — full Vite/React/TS project
- Create: `frontend/src/lib/api.ts`
- Create: `frontend/src/types/api.ts`

**Interfaces:**
- Produces: `npm run dev` starts frontend on :3000 and proxies `/api/*` to backend
- Produces: `fetchWithAuth(url, options)` — JWT injection + auto-refresh on 401

- [ ] **Step 1: Scaffold Vite project**

```bash
cd /Users/msaporito/Development/RAGgo
npm create vite@latest frontend -- --template react-ts
cd frontend
```

- [ ] **Step 2: Install dependencies**

```bash
npm install
npm install -D tailwindcss postcss autoprefixer
npx tailwindcss init -p
npm install @radix-ui/react-dialog @radix-ui/react-dropdown-menu @radix-ui/react-label @radix-ui/react-select @radix-ui/react-separator @radix-ui/react-slot @radix-ui/react-tabs @radix-ui/react-toast
npm install class-variance-authority clsx tailwind-merge lucide-react
npm install react-router-dom
npm install axios
```

- [ ] **Step 3: Install Shadcn/UI**

```bash
npx shadcn@latest init
# When prompted:
# Style: Default
# Base color: Zinc
# CSS variables: Yes
```

Then add components:

```bash
npx shadcn@latest add button card input label badge separator sheet dialog select tabs toast
```

- [ ] **Step 4: Configure Tailwind**

Replace `frontend/tailwind.config.js`:

```js
/** @type {import('tailwindcss').Config} */
export default {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: { DEFAULT: "hsl(var(--primary))", foreground: "hsl(var(--primary-foreground))" },
        secondary: { DEFAULT: "hsl(var(--secondary))", foreground: "hsl(var(--secondary-foreground))" },
        muted: { DEFAULT: "hsl(var(--muted))", foreground: "hsl(var(--muted-foreground))" },
        accent: { DEFAULT: "hsl(var(--accent))", foreground: "hsl(var(--accent-foreground))" },
        destructive: { DEFAULT: "hsl(var(--destructive))", foreground: "hsl(var(--destructive-foreground))" },
        card: { DEFAULT: "hsl(var(--card))", foreground: "hsl(var(--card-foreground))" },
      },
      borderRadius: { lg: "var(--radius)", md: "calc(var(--radius) - 2px)", sm: "calc(var(--radius) - 4px)" },
    },
  },
  plugins: [],
}
```

- [ ] **Step 5: Configure Vite proxy**

Replace `frontend/vite.config.ts`:

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { '@': path.resolve(__dirname, './src') },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
    },
  },
})
```

- [ ] **Step 6: Create `frontend/src/types/api.ts`**

```ts
export interface User {
  id: string
  username: string
  role: 'admin' | 'user'
  created_at: string
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
}

export interface Collection {
  id: number
  slug: string
  display_name: string
  owner_username: string
  qdrant_name: string
  is_default: boolean
  document_count: number
  created_at: string
}

export interface DocumentMeta {
  id: string
  filename: string
  file_type: string
  size_bytes: number
  chunks_count: number
  status: string
  owner_username: string
  collection_id: number
  uploaded_at: string
}

export interface SearchResult {
  id: string
  text: string
  score: number
  semantic_score: number
  keyword_score: number
  payload: Record<string, string>
}

export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  meta: { page: number; page_size: number; total_pages: number }
}
```

- [ ] **Step 7: Create `frontend/src/lib/api.ts`**

```ts
const BASE = '/api'

function getToken(): string | null {
  return localStorage.getItem('access_token')
}

function getRefreshToken(): string | null {
  return localStorage.getItem('refresh_token')
}

function storeTokens(access: string, refresh: string) {
  localStorage.setItem('access_token', access)
  localStorage.setItem('refresh_token', refresh)
}

async function refreshAccessToken(): Promise<boolean> {
  const rt = getRefreshToken()
  if (!rt) return false
  try {
    const res = await fetch(`${BASE}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: rt }),
    })
    if (!res.ok) return false
    const data = await res.json()
    localStorage.setItem('access_token', data.access_token)
    return true
  } catch {
    return false
  }
}

export async function fetchWithAuth(url: string, options: RequestInit = {}): Promise<Response> {
  const token = getToken()
  const headers = new Headers(options.headers)
  if (token) headers.set('Authorization', `Bearer ${token}`)

  let res = await fetch(`${BASE}${url}`, { ...options, headers })

  if (res.status === 401) {
    const refreshed = await refreshAccessToken()
    if (refreshed) {
      headers.set('Authorization', `Bearer ${getToken()}`)
      res = await fetch(`${BASE}${url}`, { ...options, headers })
    }
  }
  return res
}

// Auth
export const auth = {
  async login(username: string, password: string) {
    const res = await fetch(`${BASE}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
    if (!res.ok) throw new Error('Invalid credentials')
    const data = await res.json()
    storeTokens(data.access_token, data.refresh_token)
    return data
  },
  logout() {
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
  },
}

// Collections
export const collections = {
  async list() {
    const res = await fetchWithAuth('/collections')
    if (!res.ok) throw new Error('Failed to list collections')
    return res.json()
  },
  async create(displayName: string) {
    const res = await fetchWithAuth('/collections', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ display_name: displayName }),
    })
    if (!res.ok) throw new Error('Failed to create collection')
    return res.json()
  },
  async delete(slug: string) {
    const res = await fetchWithAuth(`/collections/${slug}`, { method: 'DELETE' })
    if (!res.ok) throw new Error('Failed to delete collection')
  },
}

// Documents
export const documents = {
  async list(page = 1, pageSize = 20) {
    const res = await fetchWithAuth(`/documents?page=${page}&page_size=${pageSize}`)
    if (!res.ok) throw new Error('Failed to list documents')
    return res.json()
  },
  async upload(file: File, collectionSlug?: string) {
    const form = new FormData()
    form.append('file', file)
    const url = collectionSlug ? `/documents?collection_slug=${collectionSlug}` : '/documents'
    const res = await fetchWithAuth(url, { method: 'POST', body: form })
    if (!res.ok) throw new Error('Failed to upload document')
    return res.json()
  },
  async delete(id: string) {
    const res = await fetchWithAuth(`/documents/${id}`, { method: 'DELETE' })
    if (!res.ok) throw new Error('Failed to delete document')
  },
  async move(id: string, targetSlug: string, targetOwner?: string) {
    const res = await fetchWithAuth(`/documents/${id}/move`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target_slug: targetSlug, target_owner_username: targetOwner }),
    })
    if (!res.ok) throw new Error('Failed to move document')
    return res.json()
  },
}

// Search
export const search = {
  async query(query: string, collectionSlug?: string, limit = 10, useHybrid = true) {
    const res = await fetchWithAuth('/search', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query, collection_slug: collectionSlug, limit, use_hybrid: useHybrid }),
    })
    if (!res.ok) throw new Error('Search failed')
    return res.json()
  },
}

// Chat
export const chat = {
  async send(message: string, sessionId?: string, collectionSlugs?: string[]) {
    const res = await fetchWithAuth('/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message, session_id: sessionId, collection_slugs: collectionSlugs }),
    })
    if (!res.ok) throw new Error('Chat failed')
    return res.json()
  },
  streamUrl(message: string, sessionId?: string, collectionSlugs?: string[]) {
    return { url: `${BASE}/chat/stream`, body: JSON.stringify({ message, session_id: sessionId, collection_slugs: collectionSlugs }) }
  },
}

// Health
export const health = {
  async check() {
    const res = await fetch(`${BASE}/health`)
    return res.json()
  },
}
```

- [ ] **Step 8: Verify frontend runs**

```bash
cd frontend && npm run build 2>&1 | tail -5
```

Expected: no errors

- [ ] **Step 9: Commit**

```bash
cd ..
git add frontend/
git commit -m "feat: frontend scaffold with Vite/React/TS, Tailwind, Shadcn/UI, API client"
```

---

### Task 13: Auth + Layout + Routing

**Files:**
- Create: `frontend/src/hooks/useAuth.tsx`
- Create: `frontend/src/components/ProtectedRoute.tsx`
- Create: `frontend/src/components/layout/MainLayout.tsx`
- Create: `frontend/src/routes/index.tsx`
- Create: `frontend/src/pages/Login.tsx`
- Modify: `frontend/src/main.tsx`

**Interfaces:**
- Produces: `useAuth()` — `{ user, login, logout, isLoading }`
- Produces: `<ProtectedRoute>` — redirects to `/login` if not authenticated
- Produces: `<MainLayout>` — sidebar + content area

- [ ] **Step 1: Create `frontend/src/hooks/useAuth.tsx`**

```tsx
import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import { auth as authApi } from '@/lib/api'

interface User {
  username: string
  role: 'admin' | 'user'
}

interface AuthContextType {
  user: User | null
  isLoading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

function parseJWT(token: string): User | null {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return { username: payload.sub, role: payload.role }
  } catch {
    return null
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const token = localStorage.getItem('access_token')
    if (token) {
      setUser(parseJWT(token))
    }
    setIsLoading(false)
  }, [])

  const login = async (username: string, password: string) => {
    await authApi.login(username, password)
    const token = localStorage.getItem('access_token')!
    setUser(parseJWT(token))
  }

  const logout = () => {
    authApi.logout()
    setUser(null)
  }

  return <AuthContext.Provider value={{ user, isLoading, login, logout }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
```

- [ ] **Step 2: Create `frontend/src/components/ProtectedRoute.tsx`**

```tsx
import { Navigate } from 'react-router-dom'
import { useAuth } from '@/hooks/useAuth'

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { user, isLoading } = useAuth()
  if (isLoading) return <div className="flex items-center justify-center h-screen">Loading...</div>
  if (!user) return <Navigate to="/login" replace />
  return <>{children}</>
}
```

- [ ] **Step 3: Create `frontend/src/components/layout/MainLayout.tsx`**

```tsx
import { Link, useLocation } from 'react-router-dom'
import { LayoutDashboard, FileText, Search, MessageSquare, Folder, Settings, HelpCircle, Database, LogOut } from 'lucide-react'
import { useAuth } from '@/hooks/useAuth'
import { cn } from '@/lib/utils'

const navItems = [
  { href: '/', label: 'Dashboard', icon: LayoutDashboard },
  { href: '/documents', label: 'Documents', icon: FileText },
  { href: '/search', label: 'Search', icon: Search },
  { href: '/chat', label: 'Chat', icon: MessageSquare },
  { href: '/collections', label: 'Collections', icon: Folder },
  { href: '/settings', label: 'Settings', icon: Settings },
  { href: '/help', label: 'Help', icon: HelpCircle },
]

const adminNavItems = [
  { href: '/qdrant', label: 'Qdrant', icon: Database },
]

export function MainLayout({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth()
  const location = useLocation()
  const year = new Date().getFullYear()

  return (
    <div className="flex h-screen bg-background">
      {/* Sidebar */}
      <aside className="w-64 border-r flex flex-col bg-card">
        <div className="p-6 border-b">
          <h1 className="font-bold text-xl">Sort Boks RAG</h1>
          <p className="text-sm text-muted-foreground mt-1">{user?.username}</p>
        </div>

        <nav className="flex-1 p-4 space-y-1">
          {navItems.map((item) => (
            <Link
              key={item.href}
              to={item.href}
              className={cn(
                'flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors',
                location.pathname === item.href
                  ? 'bg-primary text-primary-foreground'
                  : 'hover:bg-accent hover:text-accent-foreground'
              )}
            >
              <item.icon className="h-4 w-4" />
              {item.label}
            </Link>
          ))}
          {user?.role === 'admin' && adminNavItems.map((item) => (
            <Link
              key={item.href}
              to={item.href}
              className={cn(
                'flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors',
                location.pathname === item.href
                  ? 'bg-primary text-primary-foreground'
                  : 'hover:bg-accent hover:text-accent-foreground'
              )}
            >
              <item.icon className="h-4 w-4" />
              {item.label}
            </Link>
          ))}
        </nav>

        <div className="p-4 border-t space-y-2">
          <button
            onClick={logout}
            className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground w-full"
          >
            <LogOut className="h-4 w-4" /> Logout
          </button>
          <p className="text-xs text-muted-foreground">© 2025–{year} Sort Boks</p>
        </div>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-auto">
        <div className="p-6">{children}</div>
      </main>
    </div>
  )
}
```

- [ ] **Step 4: Create `frontend/src/pages/Login.tsx`**

```tsx
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '@/hooks/useAuth'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

export function Login() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const year = new Date().getFullYear()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError('')
    try {
      await login(username, password)
      navigate('/')
    } catch {
      setError('Invalid username or password')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="w-full max-w-md space-y-6">
        <div className="text-center">
          <h1 className="text-3xl font-bold">Sort Boks</h1>
          <p className="text-muted-foreground mt-2">RAG Document System</p>
        </div>
        <Card>
          <CardHeader>
            <CardTitle>Sign In</CardTitle>
            <CardDescription>Enter your credentials to access the system</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="username">Username</Label>
                <Input id="username" value={username} onChange={(e) => setUsername(e.target.value)} required />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">Password</Label>
                <Input id="password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <Button type="submit" className="w-full" disabled={loading}>
                {loading ? 'Signing in...' : 'Sign In'}
              </Button>
            </form>
          </CardContent>
        </Card>
        <p className="text-center text-xs text-muted-foreground">© 2025–{year} Sort Boks</p>
      </div>
    </div>
  )
}
```

- [ ] **Step 5: Create `frontend/src/routes/index.tsx`**

```tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AuthProvider } from '@/hooks/useAuth'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { MainLayout } from '@/components/layout/MainLayout'
import { Login } from '@/pages/Login'
import { Dashboard } from '@/pages/Dashboard'
import { Documents } from '@/pages/Documents'
import { Search } from '@/pages/Search'
import { Chat } from '@/pages/Chat'
import { Collections } from '@/pages/Collections'
import { Settings } from '@/pages/Settings'

export function AppRouter() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/*"
            element={
              <ProtectedRoute>
                <MainLayout>
                  <Routes>
                    <Route path="/" element={<Dashboard />} />
                    <Route path="/documents" element={<Documents />} />
                    <Route path="/search" element={<Search />} />
                    <Route path="/chat" element={<Chat />} />
                    <Route path="/collections" element={<Collections />} />
                    <Route path="/settings" element={<Settings />} />
                  </Routes>
                </MainLayout>
              </ProtectedRoute>
            }
          />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
```

- [ ] **Step 6: Update `frontend/src/main.tsx`**

```tsx
import React from 'react'
import ReactDOM from 'react-dom/client'
import { AppRouter } from './routes'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AppRouter />
  </React.StrictMode>
)
```

- [ ] **Step 7: Verify build**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

Expected: no errors (pages will be stubbed in next task)

- [ ] **Step 8: Commit**

```bash
cd ..
git add frontend/src/
git commit -m "feat: auth context, routing, login page, main layout"
```

---

### Task 14: Core Pages (Dashboard, Documents, Search, Chat, Collections, Settings)

**Files:**
- Create: `frontend/src/pages/Dashboard.tsx`
- Create: `frontend/src/pages/Documents.tsx`
- Create: `frontend/src/pages/Search.tsx`
- Create: `frontend/src/pages/Chat.tsx`
- Create: `frontend/src/pages/Collections.tsx`
- Create: `frontend/src/pages/Settings.tsx`

**Interfaces:**
- Produces: All pages use `fetchWithAuth` via `@/lib/api`
- Produces: Chat page streams responses via `EventSource`-style fetch

- [ ] **Step 1: Create `frontend/src/pages/Dashboard.tsx`**

```tsx
import { useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { documents, collections, health } from '@/lib/api'
import { FileText, Folder, Activity } from 'lucide-react'

export function Dashboard() {
  const [stats, setStats] = useState({ docs: 0, collections: 0, status: 'unknown' })

  useEffect(() => {
    Promise.all([
      documents.list(1, 1),
      collections.list(),
      health.check(),
    ]).then(([docsRes, colsRes, healthRes]) => {
      setStats({
        docs: docsRes.total ?? 0,
        collections: colsRes.total ?? 0,
        status: healthRes.status ?? 'unknown',
      })
    }).catch(() => {})
  }, [])

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Dashboard</h1>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Documents</CardTitle>
            <FileText className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent><div className="text-2xl font-bold">{stats.docs}</div></CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Collections</CardTitle>
            <Folder className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent><div className="text-2xl font-bold">{stats.collections}</div></CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">System Status</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className={`text-2xl font-bold ${stats.status === 'healthy' ? 'text-green-600' : 'text-red-600'}`}>
              {stats.status}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Create `frontend/src/pages/Documents.tsx`**

```tsx
import { useEffect, useState, useRef } from 'react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { documents, collections } from '@/lib/api'
import { DocumentMeta, Collection } from '@/types/api'
import { Trash2, Upload, FileText } from 'lucide-react'

export function Documents() {
  const [docs, setDocs] = useState<DocumentMeta[]>([])
  const [cols, setCols] = useState<Collection[]>([])
  const [selectedCol, setSelectedCol] = useState<string>('')
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const fileRef = useRef<HTMLInputElement>(null)

  const load = async () => {
    setLoading(true)
    try {
      const [docsRes, colsRes] = await Promise.all([documents.list(page, 20), collections.list()])
      setDocs(docsRes.data ?? [])
      setTotal(docsRes.total ?? 0)
      setCols(colsRes.data ?? [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [page])

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setLoading(true)
    try {
      await documents.upload(file, selectedCol || undefined)
      await load()
    } catch (err) {
      alert('Upload failed')
    } finally {
      setLoading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this document?')) return
    await documents.delete(id)
    await load()
  }

  const formatBytes = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Documents</h1>
        <div className="flex items-center gap-2">
          <select
            className="border rounded px-2 py-1 text-sm"
            value={selectedCol}
            onChange={(e) => setSelectedCol(e.target.value)}
          >
            <option value="">Default collection</option>
            {cols.map((c) => <option key={c.slug} value={c.slug}>{c.display_name}</option>)}
          </select>
          <Button onClick={() => fileRef.current?.click()} disabled={loading}>
            <Upload className="h-4 w-4 mr-2" /> Upload
          </Button>
          <input ref={fileRef} type="file" className="hidden" accept=".pdf,.docx,.xlsx,.txt,.md" onChange={handleUpload} />
        </div>
      </div>

      <div className="space-y-2">
        {docs.length === 0 && !loading && (
          <Card><CardContent className="py-8 text-center text-muted-foreground">No documents yet. Upload one to get started.</CardContent></Card>
        )}
        {docs.map((doc) => (
          <Card key={doc.id}>
            <CardContent className="flex items-center justify-between py-4">
              <div className="flex items-center gap-3">
                <FileText className="h-5 w-5 text-muted-foreground" />
                <div>
                  <p className="font-medium">{doc.filename}</p>
                  <p className="text-sm text-muted-foreground">
                    {formatBytes(doc.size_bytes)} · {doc.chunks_count} chunks · {new Date(doc.uploaded_at).toLocaleDateString()}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Badge variant={doc.status === 'processed' ? 'default' : 'secondary'}>{doc.status}</Badge>
                <Button variant="ghost" size="icon" onClick={() => handleDelete(doc.id)}>
                  <Trash2 className="h-4 w-4 text-destructive" />
                </Button>
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      {total > 20 && (
        <div className="flex justify-center gap-2">
          <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>Previous</Button>
          <span className="text-sm py-2">{page} / {Math.ceil(total / 20)}</span>
          <Button variant="outline" size="sm" disabled={page >= Math.ceil(total / 20)} onClick={() => setPage(p => p + 1)}>Next</Button>
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 3: Create `frontend/src/pages/Search.tsx`**

```tsx
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { search as searchApi } from '@/lib/api'
import { SearchResult } from '@/types/api'

export function Search() {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<SearchResult[]>([])
  const [loading, setLoading] = useState(false)

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!query.trim()) return
    setLoading(true)
    try {
      const res = await searchApi.query(query)
      setResults(res.data ?? [])
    } catch {
      alert('Search failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Semantic Search</h1>
      <form onSubmit={handleSearch} className="flex gap-2">
        <Input
          placeholder="Search your documents..."
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="flex-1"
        />
        <Button type="submit" disabled={loading}>{loading ? 'Searching...' : 'Search'}</Button>
      </form>

      <div className="space-y-4">
        {results.map((r) => (
          <Card key={r.id}>
            <CardHeader className="pb-2">
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm font-medium">{r.payload?.filename ?? 'Unknown'}</CardTitle>
                <Badge variant="outline">Score: {r.score.toFixed(3)}</Badge>
              </div>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground line-clamp-4">{r.text}</p>
            </CardContent>
          </Card>
        ))}
        {results.length === 0 && query && !loading && (
          <p className="text-center text-muted-foreground">No results found.</p>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Create `frontend/src/pages/Chat.tsx`**

```tsx
import { useState, useRef, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { collections } from '@/lib/api'
import { Collection, ChatMessage } from '@/types/api'
import { Send } from 'lucide-react'
import { fetchWithAuth } from '@/lib/api'

export function Chat() {
  const [msgs, setMsgs] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [sessionId, setSessionId] = useState<string | undefined>()
  const [cols, setCols] = useState<Collection[]>([])
  const [selectedCols, setSelectedCols] = useState<string[]>([])
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    collections.list().then((res) => setCols(res.data ?? [])).catch(() => {})
  }, [])

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [msgs])

  const toggleCol = (slug: string) => {
    setSelectedCols((prev) => prev.includes(slug) ? prev.filter((s) => s !== slug) : [...prev, slug])
  }

  const sendMessage = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!input.trim() || loading) return

    const userMsg: ChatMessage = { role: 'user', content: input }
    setMsgs((prev) => [...prev, userMsg])
    setInput('')
    setLoading(true)

    const assistantMsg: ChatMessage = { role: 'assistant', content: '' }
    setMsgs((prev) => [...prev, assistantMsg])

    try {
      const res = await fetchWithAuth('/chat/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          message: userMsg.content,
          session_id: sessionId,
          collection_slugs: selectedCols.length > 0 ? selectedCols : undefined,
        }),
      })

      const reader = res.body?.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (reader) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() ?? ''
        for (const line of lines) {
          if (!line.startsWith('data: ')) continue
          const data = line.slice(6)
          if (data === '[DONE]') break
          try {
            const chunk = JSON.parse(data) as string
            setMsgs((prev) => {
              const updated = [...prev]
              updated[updated.length - 1] = { role: 'assistant', content: updated[updated.length - 1].content + chunk }
              return updated
            })
          } catch {}
        }
      }
    } catch {
      setMsgs((prev) => {
        const updated = [...prev]
        updated[updated.length - 1] = { role: 'assistant', content: 'Error: failed to get response.' }
        return updated
      })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex flex-col h-full max-h-[calc(100vh-7rem)]">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-2xl font-bold">Chat</h1>
        <Button variant="outline" size="sm" onClick={() => { setMsgs([]); setSessionId(undefined) }}>New Chat</Button>
      </div>

      {/* Collection selector */}
      {cols.length > 0 && (
        <div className="flex flex-wrap gap-2 mb-4">
          {cols.map((c) => (
            <Badge
              key={c.slug}
              variant={selectedCols.includes(c.slug) ? 'default' : 'outline'}
              className="cursor-pointer"
              onClick={() => toggleCol(c.slug)}
            >
              {c.display_name}
            </Badge>
          ))}
          {selectedCols.length > 0 && (
            <Badge variant="outline" className="cursor-pointer" onClick={() => setSelectedCols([])}>Clear</Badge>
          )}
        </div>
      )}

      {/* Messages */}
      <div className="flex-1 overflow-auto space-y-4 mb-4">
        {msgs.length === 0 && (
          <div className="text-center text-muted-foreground py-12">
            Ask a question about your documents.
          </div>
        )}
        {msgs.map((m, i) => (
          <Card key={i} className={m.role === 'user' ? 'ml-12' : 'mr-12'}>
            <CardContent className="py-3 px-4">
              <p className="text-xs font-medium text-muted-foreground mb-1">{m.role === 'user' ? 'You' : 'Assistant'}</p>
              <p className="text-sm whitespace-pre-wrap">{m.content}</p>
            </CardContent>
          </Card>
        ))}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <form onSubmit={sendMessage} className="flex gap-2">
        <Input
          placeholder="Ask about your documents..."
          value={input}
          onChange={(e) => setInput(e.target.value)}
          disabled={loading}
          className="flex-1"
        />
        <Button type="submit" disabled={loading || !input.trim()}>
          <Send className="h-4 w-4" />
        </Button>
      </form>
    </div>
  )
}
```

- [ ] **Step 5: Create `frontend/src/pages/Collections.tsx`**

```tsx
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { collections as collectionsApi } from '@/lib/api'
import { Collection } from '@/types/api'
import { Trash2, Plus, Folder } from 'lucide-react'

export function Collections() {
  const [cols, setCols] = useState<Collection[]>([])
  const [newName, setNewName] = useState('')
  const [loading, setLoading] = useState(false)

  const load = async () => {
    const res = await collectionsApi.list()
    setCols(res.data ?? [])
  }

  useEffect(() => { load() }, [])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return
    setLoading(true)
    try {
      await collectionsApi.create(newName)
      setNewName('')
      await load()
    } catch {
      alert('Failed to create collection')
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (slug: string) => {
    if (!confirm(`Delete collection "${slug}"? This will remove all its vectors.`)) return
    try {
      await collectionsApi.delete(slug)
      await load()
    } catch (err: any) {
      alert(err.message ?? 'Delete failed')
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Collections</h1>

      <form onSubmit={handleCreate} className="flex gap-2">
        <Input placeholder="New collection name" value={newName} onChange={(e) => setNewName(e.target.value)} className="flex-1" />
        <Button type="submit" disabled={loading || !newName.trim()}>
          <Plus className="h-4 w-4 mr-2" /> Create
        </Button>
      </form>

      <div className="space-y-3">
        {cols.map((col) => (
          <Card key={col.id}>
            <CardContent className="flex items-center justify-between py-4">
              <div className="flex items-center gap-3">
                <Folder className="h-5 w-5 text-muted-foreground" />
                <div>
                  <p className="font-medium">{col.display_name}</p>
                  <p className="text-sm text-muted-foreground">{col.document_count} documents · {col.qdrant_name}</p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                {col.is_default && <Badge>Default</Badge>}
                {!col.is_default && (
                  <Button variant="ghost" size="icon" onClick={() => handleDelete(col.slug)}>
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                )}
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}
```

- [ ] **Step 6: Create `frontend/src/pages/Settings.tsx`**

```tsx
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { fetchWithAuth } from '@/lib/api'

export function Settings() {
  const [apiKey, setApiKey] = useState('')
  const [saved, setSaved] = useState(false)

  const handleSaveKey = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!apiKey.trim()) return
    const res = await fetchWithAuth('/users/api-key', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ key: apiKey }),
    })
    if (res.ok) {
      setSaved(true)
      setApiKey('')
      setTimeout(() => setSaved(false), 3000)
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Settings</h1>
      <Card>
        <CardHeader>
          <CardTitle>API Key</CardTitle>
          <CardDescription>Set a personal API key for programmatic access (MCP server, scripts).</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSaveKey} className="space-y-4">
            <div className="space-y-2">
              <Label>New API Key</Label>
              <Input
                type="password"
                placeholder="Enter new API key"
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
              />
            </div>
            <Button type="submit" disabled={!apiKey.trim()}>
              {saved ? 'Saved!' : 'Save API Key'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
```

- [ ] **Step 7: Build and verify**

```bash
cd frontend && npm run build 2>&1 | tail -5
```

Expected: no errors

- [ ] **Step 8: Start backend + frontend and smoke test**

```bash
# Terminal 1 — Qdrant
docker run -d -p 6334:6334 qdrant/qdrant

# Terminal 2 — Backend (copy .env.example to .env and fill in values first)
make dev

# Terminal 3 — Frontend
cd frontend && npm run dev
```

Navigate to `http://localhost:3000`, login with admin credentials, upload a `.txt` file, search it, then chat.

- [ ] **Step 9: Commit**

```bash
cd ..
git add frontend/src/pages/
git commit -m "feat: all core pages — Dashboard, Documents, Search, Chat, Collections, Settings"
```

---

### Task 15: MCP Server

**Files:**
- Create: `cmd/mcp/main.go`
- Create: `internal/mcp/tools.go`
- Test: `internal/mcp/tools_test.go`

**Interfaces:**
- Produces: `./bin/raggo-mcp` binary — stdio MCP server with 22 tools
- Consumes: `RAG_API_URL` + `RAG_API_KEY` from env

- [ ] **Step 1: Install MCP library**

```bash
go get github.com/mark3labs/mcp-go@latest
```

- [ ] **Step 2: Write test for tool handler**

Create `internal/mcp/tools_test.go`:

```go
package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "healthy"})
	}))
	defer srv.Close()

	client := &APIClient{BaseURL: srv.URL, APIKey: "test", HTTP: &http.Client{}}
	result, err := client.get("/health")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if result["status"] != "healthy" {
		t.Errorf("unexpected status: %v", result["status"])
	}
}
```

- [ ] **Step 3: Create `internal/mcp/tools.go`** — API client + 22 tool implementations

```go
package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type APIClient struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func NewAPIClient() *APIClient {
	return &APIClient{
		BaseURL: getenv("RAG_API_URL", "http://localhost:8080"),
		APIKey:  os.Getenv("RAG_API_KEY"),
		HTTP:    &http.Client{},
	}
}

func (c *APIClient) get(path string) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *APIClient) post(path string, body any) (map[string]any, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]any
	body2, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body2, &result); err != nil {
		return result, nil
	}
	return result, nil
}

func (c *APIClient) delete(path string) error {
	req, err := http.NewRequest(http.MethodDelete, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (c *APIClient) HealthCheck() (map[string]any, error) { return c.get("/health") }

func (c *APIClient) SearchDocuments(query, collectionSlug string, limit int, useHybrid bool) (map[string]any, error) {
	return c.post("/search", map[string]any{
		"query":           query,
		"collection_slug": collectionSlug,
		"limit":           limit,
		"use_hybrid":      useHybrid,
	})
}

func (c *APIClient) ListDocuments(page, pageSize int) (map[string]any, error) {
	return c.get(fmt.Sprintf("/documents?page=%d&page_size=%d", page, pageSize))
}

func (c *APIClient) DeleteDocument(id string) error {
	return c.delete("/documents/" + id)
}

func (c *APIClient) ChatWithDocuments(message, sessionID string, collectionSlugs []string) (map[string]any, error) {
	return c.post("/chat", map[string]any{
		"message":          message,
		"session_id":       sessionID,
		"collection_slugs": collectionSlugs,
	})
}

func (c *APIClient) ListCollections() (map[string]any, error) { return c.get("/collections") }

func (c *APIClient) CreateCollection(displayName string) (map[string]any, error) {
	return c.post("/collections", map[string]any{"display_name": displayName})
}

func (c *APIClient) DeleteCollection(slug string) error { return c.delete("/collections/" + slug) }

func (c *APIClient) ScrapeURL(url string) (map[string]any, error) {
	return c.post("/scrape", map[string]any{"url": url, "index": true})
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: Create `cmd/mcp/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	mcptools "raggo/internal/mcp"
)

func main() {
	client := mcptools.NewAPIClient()

	s := server.NewMCPServer("raggo", "1.0.0",
		server.WithToolCapabilities(true),
	)

	// Tool: health_check
	s.AddTool(mcp.NewTool("health_check",
		mcp.WithDescription("Check the health status of the RAGgo system"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := client.HealthCheck()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("%v", result)), nil
	})

	// Tool: search_documents
	s.AddTool(mcp.NewTool("search_documents",
		mcp.WithDescription("Search documents using semantic and hybrid search"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithString("collection_slug", mcp.Description("Collection slug to search (empty = default)")),
		mcp.WithNumber("limit", mcp.Description("Max results (default 10)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, _ := req.Params.Arguments["query"].(string)
		slug, _ := req.Params.Arguments["collection_slug"].(string)
		limit := 10
		if l, ok := req.Params.Arguments["limit"].(float64); ok {
			limit = int(l)
		}
		result, err := client.SearchDocuments(query, slug, limit, true)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := mcp_json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// Tool: list_documents
	s.AddTool(mcp.NewTool("list_documents",
		mcp.WithDescription("List all indexed documents"),
		mcp.WithNumber("page", mcp.Description("Page number (default 1)")),
		mcp.WithNumber("page_size", mcp.Description("Items per page (default 20)")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := 1
		pageSize := 20
		if p, ok := req.Params.Arguments["page"].(float64); ok {
			page = int(p)
		}
		if ps, ok := req.Params.Arguments["page_size"].(float64); ok {
			pageSize = int(ps)
		}
		result, err := client.ListDocuments(page, pageSize)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := mcp_json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// Tool: delete_document
	s.AddTool(mcp.NewTool("delete_document",
		mcp.WithDescription("Delete a document by ID"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Document ID")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, _ := req.Params.Arguments["id"].(string)
		if err := client.DeleteDocument(id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("deleted"), nil
	})

	// Tool: chat_with_documents
	s.AddTool(mcp.NewTool("chat_with_documents",
		mcp.WithDescription("Chat with documents using RAG"),
		mcp.WithString("message", mcp.Required(), mcp.Description("User message")),
		mcp.WithString("session_id", mcp.Description("Session ID for conversation continuity")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		message, _ := req.Params.Arguments["message"].(string)
		sessionID, _ := req.Params.Arguments["session_id"].(string)
		result, err := client.ChatWithDocuments(message, sessionID, nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := mcp_json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// Tool: list_collections
	s.AddTool(mcp.NewTool("list_collections",
		mcp.WithDescription("List all collections for the authenticated user"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := client.ListCollections()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := mcp_json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// Tool: create_collection
	s.AddTool(mcp.NewTool("create_collection",
		mcp.WithDescription("Create a new document collection"),
		mcp.WithString("display_name", mcp.Required(), mcp.Description("Collection display name")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, _ := req.Params.Arguments["display_name"].(string)
		result, err := client.CreateCollection(name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := mcp_json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	// Tool: delete_collection
	s.AddTool(mcp.NewTool("delete_collection",
		mcp.WithDescription("Delete a collection by slug (cannot delete default)"),
		mcp.WithString("slug", mcp.Required(), mcp.Description("Collection slug")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug, _ := req.Params.Arguments["slug"].(string)
		if err := client.DeleteCollection(slug); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText("deleted"), nil
	})

	// Tool: scrape_and_index
	s.AddTool(mcp.NewTool("scrape_and_index",
		mcp.WithDescription("Fetch a URL and index its content"),
		mcp.WithString("url", mcp.Required(), mcp.Description("URL to scrape and index")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		url, _ := req.Params.Arguments["url"].(string)
		result, err := client.ScrapeURL(url)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := mcp_json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})

	log.Println("RAGgo MCP server starting (stdio transport)")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}
```

> **Note:** Replace `mcp_json` with `"encoding/json"` import aliased as `json`. The `main.go` imports section should be:
> ```go
> import (
>     "context"
>     "encoding/json"
>     "fmt"
>     "log"
>     "github.com/mark3labs/mcp-go/mcp"
>     "github.com/mark3labs/mcp-go/server"
>     mcptools "raggo/internal/mcp"
> )
> ```
> Then use `json.Marshal(result)` instead of `mcp_json.Marshal(result)`.

- [ ] **Step 5: Run MCP tools test**

```bash
go test ./internal/mcp/... -v
```

Expected: PASS

- [ ] **Step 6: Build MCP binary**

```bash
go build ./cmd/mcp/... && echo "MCP binary built"
```

Expected: `bin/raggo-mcp` created

- [ ] **Step 7: Commit**

```bash
git add cmd/mcp/ internal/mcp/
git commit -m "feat: MCP server with 9 core tools over stdio transport"
```

---

### Task 16: Docker Compose + Makefile + Deployment

**Files:**
- Create: `Dockerfile`
- Create: `frontend/Dockerfile`
- Create: `docker-compose.yml`
- Create: `frontend/nginx.conf`
- Modify: `Makefile` (add docker targets)

**Interfaces:**
- Produces: `docker compose up -d` brings up API + Frontend + Qdrant

- [ ] **Step 1: Create `Dockerfile`** (backend multi-stage)

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o bin/raggo ./cmd/server/...

FROM alpine:3.20
RUN adduser -D appuser
WORKDIR /app
COPY --from=builder /app/bin/raggo .
RUN mkdir -p uploads data && chown -R appuser:appuser /app
USER appuser
EXPOSE 8080
CMD ["./raggo"]
```

- [ ] **Step 2: Create `frontend/nginx.conf`**

```nginx
server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://api:8080/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_buffering off;
    }

    add_header X-Content-Type-Options nosniff;
    add_header X-Frame-Options DENY;
    add_header X-XSS-Protection "1; mode=block";
}
```

- [ ] **Step 3: Create `frontend/Dockerfile`**

```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

- [ ] **Step 4: Create `docker-compose.yml`**

```yaml
services:
  qdrant:
    image: qdrant/qdrant:latest
    ports:
      - "127.0.0.1:6334:6334"
    volumes:
      - qdrant_data:/qdrant/storage
    networks:
      - raggo

  api:
    build: .
    env_file: .env
    environment:
      QDRANT_HOST: qdrant
      QDRANT_PORT: 6334
      DATABASE_URL: /app/data/rag.db
    volumes:
      - api_data:/app/data
      - api_uploads:/app/uploads
    depends_on:
      - qdrant
    networks:
      - raggo
    ports:
      - "8080:8080"

  frontend:
    build: ./frontend
    ports:
      - "3000:80"
    depends_on:
      - api
    networks:
      - raggo

networks:
  raggo:

volumes:
  qdrant_data:
  api_data:
  api_uploads:
```

- [ ] **Step 5: Add Docker targets to `Makefile`**

```makefile
up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f api

status:
	docker compose ps
```

- [ ] **Step 6: Build and smoke test**

```bash
cp .env.example .env
# Edit .env with real OPENROUTER_API_KEY, SECURITY_SECRET_KEY, etc.
docker compose up -d --build
sleep 5
curl http://localhost:8080/health
```

Expected: `{"status":"healthy",...}`

- [ ] **Step 7: Final test run**

```bash
go test ./... -count=1 2>&1 | grep -E "ok|FAIL"
cd frontend && npm run build
```

Expected: all PASS, no build errors

- [ ] **Step 8: Commit**

```bash
cd ..
git add Dockerfile frontend/Dockerfile frontend/nginx.conf docker-compose.yml Makefile
git commit -m "feat: Docker Compose stack — API + frontend + Qdrant"
```

---

## MCP Configuration (Claude Desktop)

To use the MCP server with Claude Desktop, add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "raggo": {
      "command": "/Users/msaporito/Development/RAGgo/bin/raggo-mcp",
      "env": {
        "RAG_API_URL": "http://localhost:8080",
        "RAG_API_KEY": "your-api-key-here"
      }
    }
  }
}
```

Build the MCP binary first: `make build`
