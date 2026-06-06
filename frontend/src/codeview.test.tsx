import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { markupCode, CodeView } from './codeview'

describe('markupCode', () => {
  it('returns one entry per line of code', () => {
    const lines = markupCode('line1\nline2\nline3', [])
    expect(lines).toHaveLength(3)
  })

  it('returns one line for empty code (empty string split yields one element)', () => {
    const lines = markupCode('', [])
    expect(lines).toHaveLength(1)
  })

  it('marks hit blocks with hit class', () => {
    const lines = markupCode('line1\nline2\nline3', [[1, 2, 1]])
    expect(lines[0]).toContain('class="hit bg-green-300"')
    expect(lines[1]).toContain('class="hit bg-green-300"')
    expect(lines[2]).not.toContain('class="hit')
  })

  it('marks miss blocks with miss class', () => {
    const lines = markupCode('line1\nline2\nline3', [[1, 1, 0]])
    expect(lines[0]).toContain('class="miss bg-red-200"')
  })

  it('includes line numbers', () => {
    const lines = markupCode('line1\nline2', [])
    expect(lines[0]).toContain('   1')
    expect(lines[1]).toContain('   2')
  })
})

describe('CodeView', () => {
  it('renders file path', () => {
    render(<CodeView path="src/main.go" code="hello" blocks={[]} />)
    expect(screen.getByText('src/main.go')).toBeDefined()
  })

  it('renders hit count', () => {
    render(<CodeView path="f.go" code="a\nb\nc" blocks={[[1, 2, 1]]} />)
    expect(screen.getByText('2')).toBeDefined()
  })

  it('renders miss count', () => {
    render(<CodeView path="f.go" code="a\nb\nc" blocks={[[1, 1, 0]]} />)
    expect(screen.getByText('1')).toBeDefined()
  })

  it('renders pre element with highlighted code', () => {
    const { container } = render(<CodeView path="f.go" code="hello" blocks={[]} />)
    const pre = container.querySelector('pre')
    expect(pre).not.toBeNull()
    expect(pre!.innerHTML).toContain('hello')
  })
})
