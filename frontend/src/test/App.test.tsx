import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import App from '../App'

describe('App smoke test', () => {
  it('renders the RAGgo heading', () => {
    render(<App />)
    expect(screen.getByText('RAGgo')).toBeInTheDocument()
  })

  it('renders the Get Started button', () => {
    render(<App />)
    expect(screen.getByRole('button', { name: /get started/i })).toBeInTheDocument()
  })
})
