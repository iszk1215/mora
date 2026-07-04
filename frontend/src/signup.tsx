import { useLoaderData, useNavigate } from 'react-router'
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'

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

export const SignupPage = (): React.JSX.Element => {
  const navigate = useNavigate()
  const pending = useLoaderData() as PendingSignupData | null
  const [error, setError] = useState<string | null>(null)
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

  const handleConfirm = async () => {
    setSubmitting(true)
    setError(null)

    const match = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/)
    if (!match) {
      setError('No CSRF token found. Please log in again.')
      setSubmitting(false)
      return
    }

    const formData = new FormData()
    formData.append('csrf_token', match[1])

    const resp = await fetch('/api/signup/confirm', {
      method: 'POST',
      body: formData,
    })

    if (resp.ok) {
      navigate('/', { replace: true })
    } else {
      const data = await resp.json().catch(() => ({ message: 'Signup failed' }))
      setError(data.message || 'Signup failed')
      setSubmitting(false)
    }
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

      {error && (
        <div className="bg-red-100 text-red-700 rounded p-3 mb-4 text-sm">
          {error}
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
