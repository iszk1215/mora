import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('highlight.js', () => ({
  default: {
    highlightAuto: (code: string) => ({
      value: code
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;'),
    }),
  },
}))

import { markupCode } from './codeview'

describe('markupCode', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns lines with line numbers', () => {
    const code = 'line1\nline2\nline3'
    const result = markupCode(code, [])
    expect(result).toHaveLength(3)
    expect(result[0]).toContain('   1')
    expect(result[1]).toContain('   2')
    expect(result[2]).toContain('   3')
  })

  it('marks hit lines with hit class', () => {
    const code = 'a\nb\nc'
    const blocks = [[2, 2, 1]] // line 2, hit
    const result = markupCode(code, blocks)
    expect(result[0]).not.toContain('class="hit"')
    expect(result[1]).toContain('class="hit"')
    expect(result[2]).not.toContain('class="hit"')
  })

  it('marks miss lines with miss class', () => {
    const code = 'a\nb\nc'
    const blocks = [[2, 2, 0]] // line 2, miss
    const result = markupCode(code, blocks)
    expect(result[1]).toContain('class="miss"')
  })

  it('handles range blocks (multiple lines)', () => {
    const code = 'a\nb\nc\nd'
    const blocks = [[2, 3, 1]] // lines 2-3, hit
    const result = markupCode(code, blocks)
    expect(result[0]).not.toContain('class="hit"')
    expect(result[1]).toContain('class="hit"')
    expect(result[2]).toContain('class="hit"')
    expect(result[3]).not.toContain('class="hit"')
  })

  it('handles empty code string', () => {
    const result = markupCode('', [])
    expect(result).toHaveLength(1)
    expect(result[0]).toContain('   1')
  })

  it('strips trailing whitespace from code', () => {
    const code = 'a\nb\n'
    const result = markupCode(code, [])
    expect(result).toHaveLength(2)
  })
})
