import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import { auth as authApi } from '@/lib/api'

interface User {
  username: string
  role: 'admin' | 'user'
}

interface AuthContextType {
  user: User | null
  isLoading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

function parseJWT(token: string): User | null {
  try {
    const payload = JSON.parse(atob(token.split('.')[1]))
    return { username: payload.sub, role: payload.role }
  } catch {
    return null
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const token = localStorage.getItem('access_token')
    if (token) {
      setUser(parseJWT(token))
    }
    setIsLoading(false)
  }, [])

  const login = async (username: string, password: string) => {
    await authApi.login(username, password)
    const token = localStorage.getItem('access_token')!
    setUser(parseJWT(token))
  }

  const logout = () => {
    authApi.logout()
    setUser(null)
  }

  return <AuthContext.Provider value={{ user, isLoading, login, logout }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
