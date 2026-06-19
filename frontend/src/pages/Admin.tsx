import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { admin as adminApi } from '@/lib/api'
import { Trash2, Plus, KeyRound, Users } from 'lucide-react'

interface UserRow {
  id: string
  username: string
  role: 'admin' | 'user'
  created_at: string
}

export function Admin() {
  const [users, setUsers] = useState<UserRow[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Create form
  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [newRole, setNewRole] = useState<'admin' | 'user'>('user')
  const [creating, setCreating] = useState(false)

  // Set password
  const [pwTarget, setPwTarget] = useState<string | null>(null)
  const [newPw, setNewPw] = useState('')
  const [pwSaving, setPwSaving] = useState(false)

  const load = async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await adminApi.listUsers()
      setUsers(res.data ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load users')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newUsername.trim() || !newPassword.trim()) return
    setCreating(true)
    setError(null)
    try {
      await adminApi.createUser(newUsername.trim(), newPassword, newRole)
      setNewUsername('')
      setNewPassword('')
      setNewRole('user')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create user')
    } finally {
      setCreating(false)
    }
  }

  const handleDelete = async (username: string) => {
    if (!confirm(`Delete user "${username}"? This cannot be undone.`)) return
    setError(null)
    try {
      await adminApi.deleteUser(username)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete user')
    }
  }

  const handleSetPassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!pwTarget || !newPw.trim()) return
    setPwSaving(true)
    setError(null)
    try {
      await adminApi.setPassword(pwTarget, newPw)
      setPwTarget(null)
      setNewPw('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update password')
    } finally {
      setPwSaving(false)
    }
  }

  return (
    <div className="space-y-6 max-w-3xl">
      <div className="flex items-center gap-3">
        <Users className="h-6 w-6" />
        <h1 className="text-2xl font-bold">User Management</h1>
      </div>

      {error && (
        <div className="rounded-md border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {error}
        </div>
      )}

      {/* Create user */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base flex items-center gap-2">
            <Plus className="h-4 w-4" /> Create User
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleCreate} className="grid grid-cols-1 sm:grid-cols-4 gap-3 items-end">
            <div className="space-y-1">
              <Label>Username</Label>
              <Input
                value={newUsername}
                onChange={(e) => setNewUsername(e.target.value)}
                placeholder="johndoe"
                required
              />
            </div>
            <div className="space-y-1">
              <Label>Password</Label>
              <Input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="••••••••"
                required
              />
            </div>
            <div className="space-y-1">
              <Label>Role</Label>
              <select
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={newRole}
                onChange={(e) => setNewRole(e.target.value as 'admin' | 'user')}
              >
                <option value="user">user</option>
                <option value="admin">admin</option>
              </select>
            </div>
            <Button type="submit" disabled={creating || !newUsername.trim() || !newPassword.trim()}>
              {creating ? 'Creating…' : 'Create'}
            </Button>
          </form>
        </CardContent>
      </Card>

      {/* User list */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Users ({users.length})</CardTitle>
          <CardDescription>Click the key icon to reset a user's password.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          {users.length === 0 && !loading && (
            <p className="text-sm text-muted-foreground py-4 text-center">No users found.</p>
          )}
          {users.map((u) => (
            <div key={u.id} className="flex items-center justify-between rounded-md border px-4 py-3">
              <div className="flex items-center gap-3">
                <div>
                  <p className="font-medium text-sm">{u.username}</p>
                  <p className="text-xs text-muted-foreground">
                    {new Date(u.created_at).toLocaleDateString()}
                  </p>
                </div>
                <Badge variant={u.role === 'admin' ? 'default' : 'secondary'}>{u.role}</Badge>
              </div>
              <div className="flex items-center gap-1">
                <Button
                  variant="ghost"
                  size="icon"
                  title="Reset password"
                  onClick={() => { setPwTarget(u.username); setNewPw('') }}
                >
                  <KeyRound className="h-4 w-4 text-muted-foreground" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  title="Delete user"
                  onClick={() => handleDelete(u.username)}
                >
                  <Trash2 className="h-4 w-4 text-destructive" />
                </Button>
              </div>
            </div>
          ))}
        </CardContent>
      </Card>

      {/* Reset password (inline) */}
      {pwTarget && (
        <Card className="border-primary/50">
          <CardHeader>
            <CardTitle className="text-base">Reset password for {pwTarget}</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSetPassword} className="flex gap-3 items-end">
              <div className="flex-1 space-y-1">
                <Label>New Password</Label>
                <Input
                  type="password"
                  value={newPw}
                  onChange={(e) => setNewPw(e.target.value)}
                  placeholder="••••••••"
                  autoFocus
                  required
                />
              </div>
              <Button type="submit" disabled={pwSaving || !newPw.trim()}>
                {pwSaving ? 'Saving…' : 'Save'}
              </Button>
              <Button type="button" variant="outline" onClick={() => setPwTarget(null)}>
                Cancel
              </Button>
            </form>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
