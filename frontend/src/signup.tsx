import { useLoaderData, useNavigate } from 'react-router'
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'

const MAX_USERNAME_LENGTH = 32

interface PendingSignupData {
  provider: string
  username: string
  avatar_url: string
}

export async function loadPendingSignup(): Promise<PendingSignupData | null> {
  const resp = await fetch('/api/signup/pending')
  if (!resp.ok) return null
  return resp.json()
}

// sanitizeUsernameInput mirrors the server-side sanitization rule: lowercase,
// keep only [a-z0-9_-], replace runs of other characters with '-', trim
// leading/trailing '-'/'_' and cap length. Unlike the server-side rule it
// keeps the result empty when no valid character remains, so clearing the
// input while typing does not auto-fill a fallback name.
export function sanitizeUsernameInput(input: string): string {
  let result = ''
  for (const ch of input.toLowerCase()) {
    if (/[a-z0-9_-]/.test(ch)) {
      result += ch
    } else if (result.length === 0 || result[result.length - 1] !== '-') {
      result += '-'
    }
  }

  let name = result.replace(/^[-_]+|[-_]+$/g, '')
  if (name.length > MAX_USERNAME_LENGTH) {
    name = name.slice(0, MAX_USERNAME_LENGTH).replace(/[-_]+$/, '')
  }
  return name
}

// sanitizeUsername applies the server-side "user" fallback for cases where a
// usable username must be derived from arbitrary provider data.
export function sanitizeUsername(input: string): string {
  const name = sanitizeUsernameInput(input)
  if (name === '') return 'user'
  return name
}

export const SignupPage = (): React.JSX.Element => {
  const navigate = useNavigate()
  const pending = useLoaderData() as PendingSignupData | null
  const [username, setUsername] = useState(() => sanitizeUsernameInput(pending?.username ?? ''))
  const [error, setError] = useState<string | null>(null)
  const [suggested, setSuggested] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (!pending) {
      navigate('/auth', { replace: true })
    }
  }, [pending, navigate])

  if (!pending) {
    return <div>Redirecting...</div>
  }

  const handleCancel = async () => {
    const match = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/)
    if (match) {
      const formData = new FormData()
      formData.append('csrf_token', match[1])
      await fetch('/api/signup/cancel', { method: 'POST', body: formData })
    }
    navigate('/auth', { replace: true })
  }

  const handleUsernameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setUsername(sanitizeUsernameInput(e.target.value))
    setError(null)
    setSuggested(null)
  }

  const useSuggested = () => {
    if (suggested) {
      setUsername(suggested)
      setError(null)
      setSuggested(null)
    }
  }

  const handleConfirm = async () => {
    setSubmitting(true)
    setError(null)
    setSuggested(null)

    if (username === '') {
      setError('Username is required')
      setSubmitting(false)
      return
    }

    const match = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/)
    if (!match) {
      setError('No CSRF token found. Please log in again.')
      setSubmitting(false)
      return
    }

    const formData = new FormData()
    formData.append('csrf_token', match[1])
    formData.append('username', username)

    const resp = await fetch('/api/signup/confirm', {
      method: 'POST',
      body: formData,
    })

    if (resp.ok) {
      navigate('/', { replace: true })
      return
    }

    const data = await resp.json().catch(() => null)
    if (resp.status === 409 && data?.suggested_username) {
      setSuggested(data.suggested_username)
      setError(data.message || 'Username is already taken')
    } else {
      setError(data?.message || 'Signup failed')
    }
    setSubmitting(false)
  }

  return (
    <div className="max-w-md mx-auto mt-8">
      <h1 className="text-3xl mb-4">Create Account</h1>
      <p className="mb-4 text-gray-600">
        First time logging in with <strong>{pending.provider}</strong>.
        Confirm your account details below.
      </p>

      <div className="bg-gray-100 rounded p-4 mb-4">
        <div className="flex items-center gap-3 mb-2">
          {pending.avatar_url && (
            <img src={pending.avatar_url}
                 className="w-10 h-10 rounded-full" alt="" />
          )}
          <div>
            <p className="font-semibold">{pending.username}</p>
            <p className="text-sm text-gray-500">via {pending.provider}</p>
          </div>
        </div>
      </div>

      <div className="mb-4">
        <label htmlFor="username" className="block text-sm font-medium mb-1">
          Username
        </label>
        <input
          id="username"
          type="text"
          value={username}
          onChange={handleUsernameChange}
          autoComplete="username"
          className="w-full border rounded px-3 py-2"
        />
        <p className="text-xs text-gray-500 mt-1">
          Only lowercase letters, digits, '-' and '_'.
        </p>
      </div>

      {error && (
        <div className="bg-red-100 text-red-700 rounded p-3 mb-4 text-sm">
          <span>{error}</span>
          {suggested && (
            <button type="button" className="underline ml-2" onClick={useSuggested}>
              Use suggested: {suggested}
            </button>
          )}
        </div>
      )}

      <div className="flex gap-2">
        <Button onClick={handleConfirm} disabled={submitting}>
          {submitting ? 'Creating...' : 'Create Account'}
        </Button>
        <Button variant="secondary" onClick={handleCancel}>Cancel</Button>
      </div>
    </div>
  )
}

export const signupRoute = {
  index: true,
  element: <SignupPage />,
  loader: loadPendingSignup,
}
