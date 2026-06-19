import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { fetchWithAuth } from '@/lib/api'

export function Settings() {
  const [apiKey, setApiKey] = useState('')
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSaveKey = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!apiKey.trim()) return
    setError(null)
    try {
      const res = await fetchWithAuth('/users/api-key', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: apiKey }),
      })
      if (res.ok) {
        setSaved(true)
        setApiKey('')
        setTimeout(() => setSaved(false), 3000)
      } else {
        setError('Failed to save API key')
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save API key')
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Settings</h1>
      <Card>
        <CardHeader>
          <CardTitle>API Key</CardTitle>
          <CardDescription>
            Set a personal API key for programmatic access (MCP server, scripts).
          </CardDescription>
        </CardHeader>
        <CardContent>
          {error && (
            <div className="mb-4 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {error}
            </div>
          )}
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
