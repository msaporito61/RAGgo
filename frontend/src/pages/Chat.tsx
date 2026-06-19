import { useState, useRef, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { collections, chat as chatApi } from '@/lib/api'
import type { Collection, ChatMessage } from '@/types/api'
import { Send } from 'lucide-react'

export function Chat() {
  const [msgs, setMsgs] = useState<ChatMessage[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
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
    setError(null)

    const assistantMsg: ChatMessage = { role: 'assistant', content: '' }
    setMsgs((prev) => [...prev, assistantMsg])

    try {
      // Create a session on first message
      let activeSessionId = sessionId
      if (!activeSessionId) {
        const sessionRes = await chatApi.createSession()
        if (!sessionRes.ok) throw new Error(`Failed to create session: ${sessionRes.status}`)
        const sessionData = await sessionRes.json()
        activeSessionId = sessionData.session_id
        setSessionId(activeSessionId)
      }

      const res = await chatApi.streamMessage(
        activeSessionId!,
        userMsg.content,
        selectedCols.length > 0 ? selectedCols : undefined,
        'stream',
      )

      if (!res.ok) {
        throw new Error(`Server error: ${res.status}`)
      }

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
          if (data === '[DONE]') {
            reader.cancel()
            break
          }
          try {
            const chunk = JSON.parse(data)
            const delta: string = typeof chunk === 'string' ? chunk : (chunk.delta ?? chunk.content ?? '')
            if (delta) {
              setMsgs((prev) => {
                const updated = [...prev]
                updated[updated.length - 1] = {
                  role: 'assistant',
                  content: updated[updated.length - 1].content + delta,
                }
                return updated
              })
            }
          } catch {
            // non-JSON line, skip
          }
        }
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to get response.'
      setError(msg)
      setMsgs((prev) => {
        const updated = [...prev]
        updated[updated.length - 1] = { role: 'assistant', content: `Error: ${msg}` }
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
        <Button
          variant="outline"
          size="sm"
          onClick={() => { setMsgs([]); setSessionId(undefined); setError(null) }}
        >
          New Chat
        </Button>
      </div>

      {/* Collection selector */}
      {cols.length > 0 && (
        <div className="flex flex-wrap gap-2 mb-4">
          {cols.map((c) => (
            <Badge
              key={c.slug}
              variant={selectedCols.includes(c.slug) ? 'default' : 'outline'}
              className="cursor-pointer select-none"
              onClick={() => toggleCol(c.slug)}
            >
              {c.display_name}
            </Badge>
          ))}
          {selectedCols.length > 0 && (
            <Badge
              variant="outline"
              className="cursor-pointer select-none"
              onClick={() => setSelectedCols([])}
            >
              Clear
            </Badge>
          )}
        </div>
      )}

      {error && (
        <div className="mb-4 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
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
              <p className="text-xs font-medium text-muted-foreground mb-1">
                {m.role === 'user' ? 'You' : 'Assistant'}
              </p>
              <p className="text-sm whitespace-pre-wrap">
                {m.content || (m.role === 'assistant' && loading && i === msgs.length - 1 ? '…' : '')}
              </p>
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
