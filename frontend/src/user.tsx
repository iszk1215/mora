import React, { useEffect, useState } from 'react'
import { LoaderFunctionArgs, useLoaderData, useSearchParams } from 'react-router'

import { Button } from '@/components/ui/button'
import { TrackerCard, PreviewData, fetchPreview } from './tracker'
import { TrackerResponse } from './core'

interface UserData {
  id: number
  username: string
  avatar_url: string
}

interface PaginatedTrackers {
  trackers: TrackerResponse[]
  total: number
  page: number
  per_page: number
}

export interface UserPageData {
  user: UserData
}

export async function loadUserPage({ params }: LoaderFunctionArgs): Promise<UserPageData> {
  if (!params.userName) throw new Error('userName is required')
  const resp = await fetch(`/api/users/${encodeURIComponent(params.userName)}`)
  if (!resp.ok) throw new Response('Not Found', { status: 404 })
  const user = (await resp.json()) as UserData
  return { user }
}

async function loadUserTrackers(userName: string, page?: number, perPage?: number, query?: string): Promise<PaginatedTrackers> {
  const params = new URLSearchParams()
  if (page) params.set('page', String(page))
  if (perPage) params.set('per_page', String(perPage))
  if (query) params.set('q', query)
  const qs = params.toString()
  const url = `/api/users/${encodeURIComponent(userName)}/trackers${qs ? `?${qs}` : ''}`
  const resp = await fetch(url)
  if (!resp.ok) throw resp
  return resp.json()
}

export const UserPage = (): React.JSX.Element => {
  const { user } = useLoaderData() as UserPageData
  const userName = user.username
  const [searchParams, setSearchParams] = useSearchParams()
  const urlQuery = searchParams.get('q') ?? ''
  const [query, setQuery] = useState(urlQuery)
  const [trackers, setTrackers] = useState<TrackerResponse[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [perPage] = useState(12)
  const [previews, setPreviews] = useState<Map<number, PreviewData>>(new Map())
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    const load = async () => {
      try {
        const data = await loadUserTrackers(userName, page, perPage, urlQuery || undefined)
        if (cancelled) return
        setTrackers(data.trackers)
        setTotal(data.total)
      } catch {
        if (cancelled) return
        setTrackers([])
        setTotal(0)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [userName, page, perPage, urlQuery])

  useEffect(() => {
    if (trackers.length === 0) {
      setPreviews(new Map())
      return
    }
    let cancelled = false
    const loadAll = async () => {
      const entries = await Promise.all(
        trackers.map(async (t) => {
          try {
            const data = await fetchPreview(t.id, t.type)
            return [t.id, data] as const
          } catch {
            return null
          }
        })
      )
      if (cancelled) return
      const map = new Map<number, PreviewData>()
      for (const entry of entries) {
        if (entry) map.set(entry[0], entry[1])
      }
      setPreviews(map)
    }
    void loadAll()
    return () => {
      cancelled = true
    }
  }, [trackers])

  const handleSearch = () => {
    const q = query.trim()
    setPage(1)
    if (q) {
      setSearchParams({ q }, { replace: true })
    } else {
      setSearchParams({}, { replace: true })
    }
  }

  const totalPages = perPage > 0 ? Math.ceil(total / perPage) : 1

  const handlePageChange = (newPage: number) => {
    setPage(newPage)
    const next: Record<string, string> = { page: String(newPage) }
    if (urlQuery) next.q = urlQuery
    setSearchParams(next, { replace: true })
  }

  return (
    <div>
      <div className="flex flex-wrap justify-center items-center gap-2 mb-6">
        <input
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          placeholder="Search trackers..."
          className="w-full sm:w-96 border rounded px-3 py-2"
        />
        <Button onClick={handleSearch}>Search</Button>
      </div>

      <h1 className="text-3xl mb-6">{user.username}</h1>

      {loading && <p className="text-muted-foreground">Loading...</p>}
      {!loading && trackers.length === 0 && (
        <p className="text-muted-foreground">No trackers found.</p>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {trackers.map((t) => (
          <TrackerCard key={t.id} tracker={t} preview={previews.get(t.id)} searchQuery={urlQuery} fromUser={userName} />
        ))}
      </div>

      {totalPages > 1 && (
        <div className="flex justify-center items-center gap-2 mt-6">
          <Button variant="outline" size="sm" disabled={page <= 1 || loading} onClick={() => handlePageChange(page - 1)}>
            Prev
          </Button>
          <span className="text-sm text-muted-foreground">
            Page {page} of {totalPages}
          </span>
          <Button variant="outline" size="sm" disabled={page >= totalPages || loading} onClick={() => handlePageChange(page + 1)}>
            Next
          </Button>
        </div>
      )}
    </div>
  )
}

export const userPageRoute = [
  {
    path: '/users/:userName',
    loader: loadUserPage,
    element: <UserPage />,
  },
]
