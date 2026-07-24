import { describe, it, expect } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import { SearchProvider, useSearch } from './search-context'

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

describe('SearchContext', () => {
  it('provides null when not wrapped in SearchProvider', () => {
    const NullComponent = () => {
      const search = useSearch()
      return <span data-testid="value">{search === null ? 'null' : 'not null'}</span>
    }
    render(<NullComponent />)
    expect(screen.getByTestId('value')).toHaveTextContent('null')
  })

  it('provides default values when wrapped in SearchProvider', () => {
    render(
      <SearchProvider>
        <TestComponent />
      </SearchProvider>
    )
    expect(screen.getByTestId('query')).toHaveTextContent('')
    expect(screen.getByTestId('results-count')).toHaveTextContent('0')
  })

  it('allows updating search state', () => {
    render(
      <SearchProvider>
        <TestComponent />
      </SearchProvider>
    )
    expect(screen.getByTestId('query')).toHaveTextContent('')
    
    // Click button to update search state
    act(() => {
      screen.getByText('Set Search').click()
    })
    
    expect(screen.getByTestId('query')).toHaveTextContent('test')
    expect(screen.getByTestId('results-count')).toHaveTextContent('0')
  })
})
