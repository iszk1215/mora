import { describe, it, expect, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { Footer } from './main'
import { PrivacyPage } from './privacy'

vi.mock('react-dom/client', () => ({
  default: { createRoot: () => ({ render: vi.fn() }) },
}))

describe('PrivacyPage', () => {
  it('renders the cookie usage table with all cookie names', () => {
    render(<PrivacyPage />)
    expect(screen.getByRole('heading', { name: 'Privacy Policy' })).toBeInTheDocument()
    expect(screen.getByText('morasessionid')).toBeInTheDocument()
    expect(screen.getByText('csrf_token')).toBeInTheDocument()
    expect(screen.getByText('oauth_state')).toBeInTheDocument()
  })
})

describe('Footer', () => {
  it('renders a link to the privacy policy', () => {
    render(
      <MemoryRouter>
        <Footer />
      </MemoryRouter>
    )
    const footer = document.querySelector('footer')
    expect(footer).not.toBeNull()
    const link = within(footer as HTMLElement).getByRole('link', { name: 'Privacy Policy' })
    expect(link).toHaveAttribute('href', '/privacy')
  })

  it('renders a link to the API documentation', () => {
    render(
      <MemoryRouter>
        <Footer />
      </MemoryRouter>
    )
    const footer = document.querySelector('footer')
    expect(footer).not.toBeNull()
    const link = within(footer as HTMLElement).getByRole('link', { name: 'API Doc' })
    expect(link).toHaveAttribute('href', '/swagger/')
  })
})