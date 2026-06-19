import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { HelpCircle, Upload, Search, MessageSquare, Folder, Key, Terminal } from 'lucide-react'

function Section({ icon: Icon, title, children }: {
  icon: React.ElementType
  title: string
  children: React.ReactNode
}) {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base flex items-center gap-2">
          <Icon className="h-4 w-4" />
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className="text-sm text-muted-foreground space-y-2">
        {children}
      </CardContent>
    </Card>
  )
}

export function Help() {
  return (
    <div className="space-y-6 max-w-3xl">
      <div className="flex items-center gap-3">
        <HelpCircle className="h-6 w-6" />
        <h1 className="text-2xl font-bold">Help & Documentation</h1>
      </div>

      <Section icon={Upload} title="Uploading Documents">
        <p>Go to <strong>Documents</strong> and click <strong>Upload</strong>. Supported formats: PDF, DOCX, XLSX, TXT, Markdown.</p>
        <p>Choose a collection before uploading to organize documents. Documents are automatically chunked and embedded into the vector database.</p>
        <p>Status progresses: <code className="bg-muted px-1 rounded">processing</code> → <code className="bg-muted px-1 rounded">processed</code>. Reload the page if a document stays in processing.</p>
      </Section>

      <Section icon={Folder} title="Collections">
        <p>Collections group documents for targeted search and chat. Every user gets a <strong>default</strong> collection automatically.</p>
        <p>Create additional collections for different topics or projects. The default collection cannot be deleted.</p>
        <p>In Search and Chat you can filter by one or multiple collections, or leave unselected to search across all your documents.</p>
      </Section>

      <Section icon={Search} title="Semantic Search">
        <p>Search uses <strong>hybrid search</strong> — combining dense vector similarity with BM25 keyword matching for best results.</p>
        <p>Toggle <em>Hybrid search</em> off to use pure vector similarity only. Results include a relevance score and the matched text chunk.</p>
        <p>Tip: phrase your query as a question or statement, not just keywords, for better semantic matches.</p>
      </Section>

      <Section icon={MessageSquare} title="RAG Chat">
        <p>Chat uses Retrieval-Augmented Generation: your question is used to find relevant chunks, which are injected into the LLM context.</p>
        <p>Select collections to scope the context, or leave unselected to draw from all documents. Each conversation has a session; click <strong>New Chat</strong> to start fresh.</p>
        <p>Responses stream token-by-token via SSE. If a question can't be answered from the documents, the assistant will say so.</p>
      </Section>

      <Section icon={Key} title="API Keys">
        <p>Set a personal API key in <strong>Settings</strong>. The key can be used with the <code className="bg-muted px-1 rounded">X-API-Key</code> header to authenticate API calls without JWT.</p>
        <p>This is required to use the MCP server with external tools like Claude Desktop.</p>
      </Section>

      <Section icon={Terminal} title="MCP Server (Claude Desktop)">
        <p>Build the MCP binary: <code className="bg-muted px-1 rounded">make build</code> (produces <code className="bg-muted px-1 rounded">bin/raggo-mcp</code>).</p>
        <p>Add to <code className="bg-muted px-1 rounded">~/Library/Application Support/Claude/claude_desktop_config.json</code>:</p>
        <pre className="bg-muted rounded p-3 text-xs overflow-x-auto">{`{
  "mcpServers": {
    "raggo": {
      "command": "/path/to/raggo/bin/raggo-mcp",
      "env": {
        "RAGGO_BASE_URL": "http://localhost:8080",
        "RAGGO_API_KEY": "your-api-key"
      }
    }
  }
}`}</pre>
        <p>Available tools: health check, search, list/delete documents, chat, list/create/delete collections, scrape URL, and more.</p>
      </Section>
    </div>
  )
}
