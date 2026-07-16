import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

interface APIKeyData {
  id: number
  name: string
  key_prefix: string
  created_at: string
}

interface APIKeyCreateResponse {
  id: number
  name: string
  key: string
  key_prefix: string
  created_at: string
}

function getCSRFToken(): string {
  const match = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/)
  return match ? match[1] : ''
}

async function listAPIKeys(): Promise<APIKeyData[]> {
  const resp = await fetch('/api/user/me/api-keys')
  if (!resp.ok) throw resp
  return resp.json()
}

async function createAPIKey(name: string): Promise<APIKeyCreateResponse> {
  const resp = await fetch('/api/user/me/api-keys', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': getCSRFToken(),
    },
    body: JSON.stringify({ name }),
  })
  if (!resp.ok) throw resp
  return resp.json()
}

async function revokeAPIKey(id: number): Promise<void> {
  const resp = await fetch(`/api/user/me/api-keys/${id}`, {
    method: 'DELETE',
    headers: {
      'X-CSRF-Token': getCSRFToken(),
    },
  })
  if (!resp.ok) throw resp
}

export const APIKeyPage = (): React.JSX.Element => {
  const navigate = useNavigate()

  const [keys, setKeys] = useState<APIKeyData[]>([])
  const [loading, setLoading] = useState(true)
  const [newKeyName, setNewKeyName] = useState('')
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [createdKey, setCreatedKey] = useState<string | null>(null)
  const [revokingId, setRevokingId] = useState<number | null>(null)

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const data = await listAPIKeys()
        if (!cancelled) setKeys(data)
      } catch {
        if (!cancelled) navigate('/auth', { replace: true })
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => { cancelled = true }
  }, [navigate])

  const handleCreate = async () => {
    if (!newKeyName.trim()) return
    setCreating(true)
    setError(null)
    setCreatedKey(null)
    try {
      const resp = await createAPIKey(newKeyName.trim())
      setCreatedKey(resp.key)
      setNewKeyName('')
      const updated = await listAPIKeys()
      setKeys(updated)
    } catch {
      setError('Failed to create API key.')
    } finally {
      setCreating(false)
    }
  }

  const handleRevoke = async (id: number) => {
    setRevokingId(id)
    try {
      await revokeAPIKey(id)
      setKeys((prev) => prev.filter((k) => k.id !== id))
    } catch {
      setError('Failed to revoke API key.')
    } finally {
      setRevokingId(null)
    }
  }

  if (loading) {
    return <div>Loading...</div>
  }

  return (
    <div className="max-w-2xl mx-auto mt-8">
      <h1 className="text-3xl mb-4">API Keys</h1>
      <p className="text-gray-600 mb-6">
        Manage API keys for programmatic access. Each key grants full access to your account.
      </p>

      {error && (
        <div className="bg-red-100 text-red-700 rounded p-3 mb-4 text-sm">
          {error}
        </div>
      )}

      {createdKey && (
        <div className="bg-green-100 text-green-800 rounded p-4 mb-4">
          <p className="font-semibold mb-1">API Key Created</p>
          <p className="text-sm mb-2">
            Copy this key now. You will not be able to see it again.
          </p>
          <div className="bg-card border rounded px-3 py-2 font-mono text-sm break-all select-all">
            {createdKey}
          </div>
          <Button
            variant="outline"
            size="sm"
            className="mt-2"
            onClick={() => {
              navigator.clipboard.writeText(createdKey).catch(() => {})
            }}
          >
            Copy to Clipboard
          </Button>
        </div>
      )}

      <div className="flex items-center gap-2 mb-6">
        <input
          type="text"
          value={newKeyName}
          onChange={(e) => setNewKeyName(e.target.value)}
          placeholder="Key name (e.g. CI script)"
          className="border rounded px-2 py-1 flex-1"
          onKeyDown={(e) => { if (e.key === 'Enter') handleCreate() }}
          disabled={creating}
        />
        <Button onClick={handleCreate} disabled={!newKeyName.trim() || creating}>
          {creating ? 'Creating...' : 'Create Key'}
        </Button>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead>Key Prefix</TableHead>
            <TableHead>Created</TableHead>
            <TableHead className="w-24">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {keys.length === 0 ? (
            <TableRow>
              <TableCell colSpan={4} className="text-center text-muted-foreground">
                No API keys yet
              </TableCell>
            </TableRow>
          ) : (
            keys.map((k) => (
              <TableRow key={k.id}>
                <TableCell className="font-medium">{k.name}</TableCell>
                <TableCell className="font-mono text-sm">{k.key_prefix}...</TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {new Date(k.created_at).toLocaleDateString()}
                </TableCell>
                <TableCell>
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={() => handleRevoke(k.id)}
                    disabled={revokingId === k.id}
                  >
                    {revokingId === k.id ? 'Revoking...' : 'Revoke'}
                  </Button>
                </TableCell>
              </TableRow>
            ))
          )}
        </TableBody>
      </Table>
    </div>
  )
}

export const apiKeyRoute = {
  index: true,
  element: <APIKeyPage />,
}
