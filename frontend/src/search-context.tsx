import { createContext, useContext } from 'react'
import { TrackerResponse } from './core'
import type { PreviewData } from './tracker'

export interface SearchState {
  query: string
  results: TrackerResponse[]
  previews: Map<number, PreviewData>
}

export interface SearchContextValue extends SearchState {
  setSearch: (state: SearchState) => void
}

export const SearchContext = createContext<SearchContextValue | null>(null)

export const SearchProvider = SearchContext.Provider

export function useSearch(): SearchContextValue | null {
  return useContext(SearchContext)
}
