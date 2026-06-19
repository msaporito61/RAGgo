import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi } from 'vitest'
import { Login } from '../pages/Login'
import { AuthProvider } from '../hooks/useAuth'

// Mock the api module so tests don't make real network calls
vi.mock('../lib/api', () => ({
  auth: {
    login: vi.fn(),
    logout: vi.fn(),
  },
}))

// Mock localStorage since Node 26 experimental localStorage is undefined
const localStorageMock = (() => {
  let store: Record<string, string> = {}
  return {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => { store[key] = value },
    removeItem: (key: string) => { delete store[key] },
    clear: () => { store = {} },
  }
})()
Object.defineProperty(globalThis, 'localStorage', { value: localStorageMock, writable: true })

function renderWithProviders(ui: React.ReactElement) {
  return render(
    <MemoryRouter>
      <AuthProvider>{ui}</AuthProvider>
    </MemoryRouter>
  )
}

describe('Login page smoke test', () => {
  it('renders the Sort Boks heading', () => {
    renderWithProviders(<Login />)
    expect(screen.getByText('Sort Boks')).toBeInTheDocument()
  })

  it('renders the Sign In button', () => {
    renderWithProviders(<Login />)
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
  })

  it('renders username and password fields', () => {
    renderWithProviders(<Login />)
    expect(screen.getByLabelText(/username/i)).toBeInTheDocument()
    expect(screen.getByLabelText(/password/i)).toBeInTheDocument()
  })
})
