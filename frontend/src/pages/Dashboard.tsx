import { useEffect, useState } from 'react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { documents, collections, health } from '@/lib/api'
import { FileText, Folder, Activity } from 'lucide-react'

export function Dashboard() {
  const [stats, setStats] = useState({ docs: 0, collections: 0, status: 'unknown' })

  useEffect(() => {
    Promise.all([
      documents.list(1, 1),
      collections.list(),
      health.check(),
    ]).then(([docsRes, colsRes, healthRes]) => {
      setStats({
        docs: docsRes.total ?? 0,
        collections: colsRes.total ?? 0,
        status: healthRes.status ?? 'unknown',
      })
    }).catch(() => {})
  }, [])

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Dashboard</h1>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Documents</CardTitle>
            <FileText className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent><div className="text-2xl font-bold">{stats.docs}</div></CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Collections</CardTitle>
            <Folder className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent><div className="text-2xl font-bold">{stats.collections}</div></CardContent>
        </Card>
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">System Status</CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className={`text-2xl font-bold ${stats.status === 'healthy' ? 'text-green-600' : 'text-red-600'}`}>
              {stats.status}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
