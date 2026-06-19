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
    return {
      url: `${BASE}/chat/stream`,
      body: JSON.stringify({ message, session_id: sessionId, collection_slugs: collectionSlugs }),
    }
  },
}

// Health
export const health = {
  async check() {
    const res = await fetch(`${BASE}/health`)
    return res.json()
  },
}
