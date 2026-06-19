import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { collections as collectionsApi } from '@/lib/api'
import type { Collection } from '@/types/api'
import { Trash2, Plus, Folder } from 'lucide-react'

export function Collections() {
  const [cols, setCols] = useState<Collection[]>([])
  const [newName, setNewName] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    setError(null)
    try {
      const res = await collectionsApi.list()
      setCols(res.data ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load collections')
    }
  }

  useEffect(() => { load() }, [])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return
    setLoading(true)
    setError(null)
    try {
      await collectionsApi.create(newName)
      setNewName('')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create collection')
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (slug: string) => {
    if (!confirm(`Delete collection "${slug}"? This will remove all its vectors.`)) return
    setError(null)
    try {
      await collectionsApi.delete(slug)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Collections</h1>

      <form onSubmit={handleCreate} className="flex gap-2">
        <Input
          placeholder="New collection name"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          className="flex-1"
        />
        <Button type="submit" disabled={loading || !newName.trim()}>
          <Plus className="h-4 w-4 mr-2" /> Create
        </Button>
      </form>

      {error && (
        <div className="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      <div className="space-y-3">
        {cols.length === 0 && !loading && (
          <Card>
            <CardContent className="py-8 text-center text-muted-foreground">
              No collections yet. Create one to get started.
            </CardContent>
          </Card>
        )}
        {cols.map((col) => (
          <Card key={col.id}>
            <CardContent className="flex items-center justify-between py-4">
              <div className="flex items-center gap-3">
                <Folder className="h-5 w-5 text-muted-foreground" />
                <div>
                  <p className="font-medium">{col.display_name}</p>
                  <p className="text-sm text-muted-foreground">
                    {col.document_count} documents · {col.qdrant_name}
                  </p>
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
