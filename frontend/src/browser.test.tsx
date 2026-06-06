import { describe, it, expect } from 'vitest'
import { list2tree, collectItems, forEachItem } from './browser'
import { FileData } from './core'

describe('list2tree', () => {
  it('sets ratio to 0 when file has 0 lines', () => {
    const files: FileData[] = [{ filename: 'empty.go', hits: 0, lines: 0 }]
    const root = list2tree(files)
    const items = collectItems(root)
    expect(items[0].ratio).toBe(0)
  })
})
