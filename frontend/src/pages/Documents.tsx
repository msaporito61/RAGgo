import { useEffect, useState, useRef } from 'react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { documents, collections } from '@/lib/api'
import type { DocumentMeta, Collection } from '@/types/api'
import { Trash2, Upload, FileText } from 'lucide-react'

export function Documents() {
  const [docs, setDocs] = useState<DocumentMeta[]>([])
  const [cols, setCols] = useState<Collection[]>([])
  const [selectedCol, setSelectedCol] = useState<string>('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const fileRef = useRef<HTMLInputElement>(null)

  const load = async () => {
    setLoading(true)
    setError(null)
    try {
      const [docsRes, colsRes] = await Promise.all([documents.list(page, 20), collections.list()])
      setDocs(docsRes.data ?? [])
      setTotal(docsRes.total ?? 0)
      setCols(colsRes.data ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load documents')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [page])

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setLoading(true)
    setError(null)
    try {
      await documents.upload(file, selectedCol || undefined)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setLoading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this document?')) return
    setError(null)
    try {
      await documents.delete(id)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Delete failed')
    }
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

      {error && (
        <div className="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {loading && docs.length === 0 && (
        <p className="text-center text-muted-foreground py-8">Loading…</p>
      )}

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
