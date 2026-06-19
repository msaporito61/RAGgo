import { useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { admin as adminApi } from '@/lib/api'
import { Database, Folder, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { Collection } from '@/types/api'

export function Qdrant() {
  const [collections, setCollections] = useState<Collection[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const totalDocs = collections.reduce((sum, c) => sum + c.document_count, 0)

  const load = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await adminApi.listAllCollections()
      setCollections(res.data ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load collections')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  return (
    <div className="space-y-6 max-w-4xl">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Database className="h-6 w-6" />
          <h1 className="text-2xl font-bold">Qdrant Collections</h1>
        </div>
        <Button variant="outline" size="sm" onClick={load} disabled={loading}>
          <RefreshCw className={`h-4 w-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Summary */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Total Collections</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{collections.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Total Documents</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">{totalDocs}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Unique Owners</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-bold">
              {new Set(collections.map((c) => c.owner_username)).size}
            </p>
          </CardContent>
        </Card>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {/* Collection table */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">All Collections</CardTitle>
        </CardHeader>
        <CardContent>
          {loading && collections.length === 0 && (
            <p className="text-sm text-muted-foreground text-center py-6">Loading…</p>
          )}
          {!loading && collections.length === 0 && !error && (
            <p className="text-sm text-muted-foreground text-center py-6">No collections found.</p>
          )}
          <div className="space-y-2">
            {collections.map((col) => (
              <div
                key={col.id}
                className="flex items-center justify-between rounded-md border px-4 py-3"
              >
                <div className="flex items-center gap-3 min-w-0">
                  <Folder className="h-4 w-4 shrink-0 text-muted-foreground" />
                  <div className="min-w-0">
                    <p className="font-medium text-sm truncate">{col.display_name}</p>
                    <p className="text-xs text-muted-foreground font-mono truncate">{col.qdrant_name}</p>
                  </div>
                </div>
                <div className="flex items-center gap-3 shrink-0 ml-4">
                  <span className="text-sm text-muted-foreground">{col.document_count} docs</span>
                  <Badge variant="outline" className="text-xs">{col.owner_username}</Badge>
                  {col.is_default && <Badge className="text-xs">default</Badge>}
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
