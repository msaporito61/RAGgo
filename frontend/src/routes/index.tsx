import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AuthProvider } from '@/hooks/useAuth'
import { ProtectedRoute, AdminRoute } from '@/components/ProtectedRoute'
import { MainLayout } from '@/components/layout/MainLayout'
import { Login } from '@/pages/Login'
import { Dashboard } from '@/pages/Dashboard'
import { Documents } from '@/pages/Documents'
import { Search } from '@/pages/Search'
import { Chat } from '@/pages/Chat'
import { Collections } from '@/pages/Collections'
import { Settings } from '@/pages/Settings'
import { Help } from '@/pages/Help'
import { Admin } from '@/pages/Admin'
import { Qdrant } from '@/pages/Qdrant'

export function AppRouter() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/*"
            element={
              <ProtectedRoute>
                <MainLayout>
                  <Routes>
                    <Route path="/" element={<Dashboard />} />
                    <Route path="/documents" element={<Documents />} />
                    <Route path="/search" element={<Search />} />
                    <Route path="/chat" element={<Chat />} />
                    <Route path="/collections" element={<Collections />} />
                    <Route path="/settings" element={<Settings />} />
                    <Route path="/help" element={<Help />} />
                    <Route
                      path="/admin"
                      element={
                        <AdminRoute>
                          <Admin />
                        </AdminRoute>
                      }
                    />
                    <Route
                      path="/qdrant"
                      element={
                        <AdminRoute>
                          <Qdrant />
                        </AdminRoute>
                      }
                    />
                  </Routes>
                </MainLayout>
              </ProtectedRoute>
            }
          />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
