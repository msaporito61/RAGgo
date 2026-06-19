import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { search as searchApi, collections as collectionsApi } from '@/lib/api'
import type { SearchResult, Collection, PaginatedResponse } from '@/types/api'

export function Search() {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<SearchResult[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [searched, setSearched] = useState(false)
  const [collections, setCollections] = useState<Collection[]>([])
  const [selectedCollectionSlug, setSelectedCollectionSlug] = useState<string | undefined>(undefined)
  const [useHybrid, setUseHybrid] = useState(true)

  useEffect(() => {
    const loadCollections = async () => {
      try {
        const res = await collectionsApi.list() as PaginatedResponse<Collection>
        setCollections(res.data ?? [])
      } catch (err) {
        console.error('Failed to load collections:', err)
      }
    }
    loadCollections()
  }, [])

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!query.trim()) return
    setLoading(true)
    setError(null)
    try {
      const res = await searchApi.query(query, selectedCollectionSlug, 10, useHybrid)
      setResults(res.data ?? [])
      setSearched(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Semantic Search</h1>
      <form onSubmit={handleSearch} className="space-y-4">
        <div className="flex gap-2">
          <Input
            placeholder="Search your documents..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="flex-1"
          />
          <Button type="submit" disabled={loading || !query.trim()}>
            {loading ? 'Searching…' : 'Search'}
          </Button>
        </div>

        <div className="flex flex-col gap-4 sm:flex-row sm:items-end">
          <div className="flex-1">
            <label className="mb-2 block text-sm font-medium">Collection</label>
            <select
              value={selectedCollectionSlug ?? ''}
              onChange={(e) => setSelectedCollectionSlug(e.target.value || undefined)}
              className="w-full rounded border border-input bg-background px-3 py-2 text-sm"
            >
              <option value="">All collections</option>
              {collections.map((col) => (
                <option key={col.slug} value={col.slug}>
                  {col.display_name}
                </option>
              ))}
            </select>
          </div>

          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="hybrid-toggle"
              checked={useHybrid}
              onChange={(e) => setUseHybrid(e.target.checked)}
              className="h-4 w-4 rounded border-gray-300"
            />
            <label htmlFor="hybrid-toggle" className="text-sm font-medium">
              Hybrid search
            </label>
          </div>
        </div>
      </form>

      {error && (
        <div className="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

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
        {results.length === 0 && searched && !loading && (
          <p className="text-center text-muted-foreground">No results found.</p>
        )}
      </div>
    </div>
  )
}
