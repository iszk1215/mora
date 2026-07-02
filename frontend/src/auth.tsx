import { useState } from 'react'
import { useNavigate } from 'react-router'
import { Button } from '@/components/ui/button'

async function fetchCSRFToken(): Promise<string | null> {
  const resp = await fetch('/api/auth/csrf')
  if (!resp.ok) return null
  const data = await resp.json()
  return data.csrf_token
}

export const PasswordLoginForm = (): React.JSX.Element => {
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    setError(null)

    const csrfToken = await fetchCSRFToken()
    if (!csrfToken) {
      setError('Failed to get CSRF token. Please refresh the page.')
      setSubmitting(false)
      return
    }

    const resp = await fetch('/api/auth/login', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': csrfToken,
      },
      body: JSON.stringify({ username, password }),
    })

    if (resp.ok) {
      navigate('/', { replace: true })
    } else {
      const data = await resp.json().catch(() => ({ message: 'Request failed' }))
      setError(data.message || 'Request failed')
      setSubmitting(false)
    }
  }

  return (
    <div className="border-t pt-6 mt-6">
      <h2 className="text-xl font-semibold mb-4">Sign in with Password</h2>
      <form onSubmit={handleSubmit} className="space-y-3">
        <div>
          <label htmlFor="auth-username" className="block text-sm font-medium mb-1">
            Username
          </label>
          <input
            id="auth-username"
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            className="w-full px-3 py-2 border rounded text-sm"
            required
            autoComplete="username"
          />
        </div>
        <div>
          <label htmlFor="auth-password" className="block text-sm font-medium mb-1">
            Password
          </label>
          <input
            id="auth-password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="w-full px-3 py-2 border rounded text-sm"
            required
            autoComplete="current-password"
          />
        </div>
        {error && (
          <div className="bg-red-100 text-red-700 rounded p-3 text-sm">
            {error}
          </div>
        )}
        <Button type="submit" disabled={submitting} className="w-full">
          {submitting ? 'Signing in...' : 'Sign In'}
        </Button>
      </form>
    </div>
  )
}
