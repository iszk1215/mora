import React, { createContext, useContext, useState, useCallback } from 'react'
import { TrackerResponse } from './core'
import type { PreviewData } from './tracker'

export interface SearchState {
  query: string
  results: TrackerResponse[]
  previews: Map<number, PreviewData>
}

interface SearchContextValue extends SearchState {
  setSearch: (state: SearchState) => void
}

const SearchContext = createContext<SearchContextValue | null>(null)

export const SearchProvider = ({ children }: { children: React.ReactNode }): React.JSX.Element => {
  const [state, setState] = useState<SearchState>({
    query: '',
    results: [],
    previews: new Map(),
  })
  const setSearch = useCallback((s: SearchState) => setState(s), [])
  return (
    <SearchContext.Provider value={{ ...state, setSearch }}>
      {children}
    </SearchContext.Provider>
  )
}

export function useSearch(): SearchContextValue | null {
  return useContext(SearchContext)
}
