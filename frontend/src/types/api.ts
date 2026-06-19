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
