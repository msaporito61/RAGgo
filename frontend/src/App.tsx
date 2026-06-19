import { Button } from '@/components/ui/button'

function App() {
  return (
    <div className="min-h-screen bg-background flex items-center justify-center">
      <div className="text-center space-y-4">
        <h1 className="text-4xl font-bold text-foreground">RAGgo</h1>
        <p className="text-muted-foreground">Retrieval-Augmented Generation Platform</p>
        <Button>Get Started</Button>
      </div>
    </div>
  )
}

export default App
