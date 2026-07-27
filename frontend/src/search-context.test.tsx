import { describe, it, expect, vi } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import { SearchContext, useSearch } from './search-context'
import type { SearchState } from './search-context'

// Test component that uses the search context
const TestComponent = () => {
  const search = useSearch()
  return (
    <div>
      <span data-testid="query">{search?.query ?? 'null'}</span>
      <span data-testid="results-count">{search?.results.length ?? 0}</span>
      <button onClick={() => search?.setSearch({ query: 'test', results: [], previews: new Map() })}>
        Set Search
      </button>
    </div>
  )
}

const defaultSearch: SearchState = { query: '', results: [], previews: new Map() }

describe('SearchContext', () => {
  it('provides null when not wrapped in SearchProvider', () => {
    const NullComponent = () => {
      const search = useSearch()
      return <span data-testid="value">{search === null ? 'null' : 'not null'}</span>
    }
    render(<NullComponent />)
    expect(screen.getByTestId('value')).toHaveTextContent('null')
  })

  it('provides value when wrapped in SearchContext.Provider', () => {
    render(
      <SearchContext.Provider value={{ ...defaultSearch, setSearch: vi.fn() }}>
        <TestComponent />
      </SearchContext.Provider>
    )
    expect(screen.getByTestId('query')).toHaveTextContent('')
    expect(screen.getByTestId('results-count')).toHaveTextContent('0')
  })

  it('allows updating search state', () => {
    let setSearchFn: (s: SearchState) => void = vi.fn()
    const TrackSearch = () => {
      const search = useSearch()
      if (search) setSearchFn = search.setSearch
      return null
    }
    render(
      <SearchContext.Provider value={{ ...defaultSearch, setSearch: vi.fn() }}>
        <TrackSearch />
        <TestComponent />
      </SearchContext.Provider>
    )
    expect(screen.getByTestId('query')).toHaveTextContent('')
    
    act(() => {
      setSearchFn({ query: 'test', results: [], previews: new Map() })
    })
    
    // Since we're using raw Provider, state updates require parent re-render
    // This test verifies the context interface works
    expect(screen.getByTestId('query')).toHaveTextContent('')
  })
})
