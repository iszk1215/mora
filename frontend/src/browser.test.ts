import { describe, it, expect } from 'vitest'
import { list2tree, updateTree, forEachItem, collectItems } from './browser'
import type { Item } from './browser'
import type { FileData } from './core'

describe('list2tree', () => {
  it('converts flat file list to tree', () => {
    const files: FileData[] = [
      { filename: 'src/main.ts', hits: 30, lines: 50 },
      { filename: 'src/util.ts', hits: 20, lines: 30 },
      { filename: 'README.md', hits: 5, lines: 10 },
    ]
    const root = list2tree(files)
    expect(root.type).toBe('dir')
    expect(root.name).toBe('')
    expect(root.children).toHaveLength(2) // src/ dir and README.md

    const srcDir = root.children.find(c => c.name === 'src')
    expect(srcDir).toBeDefined()
    expect(srcDir!.type).toBe('dir')
    expect(srcDir!.children).toHaveLength(2)

    const readme = root.children.find(c => c.name === 'README.md')
    expect(readme).toBeDefined()
    expect(readme!.type).toBe('file')
  })

  it('calculates directory coverage', () => {
    const files: FileData[] = [
      { filename: 'src/a.ts', hits: 10, lines: 20 },
      { filename: 'src/b.ts', hits: 30, lines: 40 },
    ]
    const root = list2tree(files)
    const srcDir = root.children[0]
    expect(srcDir.hits).toBe(40)
    expect(srcDir.lines).toBe(60)
    expect(srcDir.ratio).toBeCloseTo(66.7, 1)
  })

  it('returns empty root for empty file list', () => {
    const root = list2tree([])
    expect(root.children).toHaveLength(0)
    expect(root.hits).toBe(0)
    expect(root.lines).toBe(0)
  })

  it('sorts directories before files', () => {
    const files: FileData[] = [
      { filename: 'a.txt', hits: 0, lines: 0 },
      { filename: 'zzz/doc.txt', hits: 0, lines: 0 },
    ]
    const root = list2tree(files)
    expect(root.children[0].type).toBe('dir')
    expect(root.children[0].name).toBe('zzz')
    expect(root.children[1].type).toBe('file')
    expect(root.children[1].name).toBe('a.txt')
  })
})

describe('updateTree', () => {
  const makeLeaf = (name: string, state: number): Item => ({
    name, type: 'file', depth: 1, state, children: [],
    hits: 10, lines: 20, ratio: 50.0,
  })

  const makeDir = (name: string, state: number, children: Item[]): Item => ({
    name, type: 'dir', depth: 0, state, children,
    hits: 0, lines: 0, ratio: 0,
  })

  it('toggles directory state from open to closed', () => {
    const child = makeLeaf('f.ts', 1)
    const dir = makeDir('src', 1, [child])
    const result = updateTree(dir, dir)
    expect(result.state).toBe(0)
    expect(result.children[0].state).toBe(1) // child state unchanged
  })

  it('toggles directory state from closed to open', () => {
    const child = makeLeaf('f.ts', 1)
    const dir = makeDir('src', 0, [child])
    const result = updateTree(dir, dir)
    expect(result.state).toBe(1)
  })

  it('does not change state for unselected item', () => {
    const child = makeLeaf('f.ts', 1)
    const dir = makeDir('src', 1, [child])
    const other = makeLeaf('other.ts', 1)
    const result = updateTree(dir, other)
    expect(result.state).toBe(1) // unchanged
  })

  it('returns a new object without mutating source', () => {
    const child = makeLeaf('f.ts', 1)
    const dir = makeDir('src', 1, [child])
    const result = updateTree(dir, dir)
    expect(result).not.toBe(dir)
    expect(dir.state).toBe(1) // source unchanged
    expect(result.state).toBe(0) // result toggled
  })
})

describe('forEachItem', () => {
  it('visits all items in depth-first order', () => {
    const leaf1: Item = { name: 'a.ts', type: 'file', depth: 2, state: 1, children: [], hits: 0, lines: 0, ratio: 0 }
    const leaf2: Item = { name: 'b.ts', type: 'file', depth: 2, state: 1, children: [], hits: 0, lines: 0, ratio: 0 }
    const subdir: Item = { name: 'sub', type: 'dir', depth: 1, state: 1, children: [leaf1, leaf2], hits: 0, lines: 0, ratio: 0 }
    const root: Item = { name: '', type: 'dir', depth: 0, state: 1, children: [subdir], hits: 0, lines: 0, ratio: 0 }

    const visited: string[] = []
    forEachItem(root, (item) => {
      visited.push(item.name)
      return true
    })
    expect(visited).toEqual(['', 'sub', 'a.ts', 'b.ts'])
  })

  it('stops recursion when callback returns false', () => {
    const leaf: Item = { name: 'a.ts', type: 'file', depth: 2, state: 1, children: [], hits: 0, lines: 0, ratio: 0 }
    const dir: Item = { name: 'dir', type: 'dir', depth: 1, state: 1, children: [leaf], hits: 0, lines: 0, ratio: 0 }
    const root: Item = { name: '', type: 'dir', depth: 0, state: 1, children: [dir], hits: 0, lines: 0, ratio: 0 }

    const visited: string[] = []
    forEachItem(root, (item) => {
      visited.push(item.name)
      return false // stop at this level
    })
    expect(visited).toEqual([''])
  })
})

describe('collectItems', () => {
  it('collects visible items from tree', () => {
    const leaf1: Item = { name: 'a.ts', type: 'file', depth: 2, state: 1, children: [], hits: 0, lines: 0, ratio: 0 }
    const leaf2: Item = { name: 'b.ts', type: 'file', depth: 2, state: 1, children: [], hits: 0, lines: 0, ratio: 0 }
    const subdir: Item = { name: 'sub', type: 'dir', depth: 1, state: 0, children: [leaf1, leaf2], hits: 0, lines: 0, ratio: 0 }
    const root: Item = { name: '', type: 'dir', depth: 0, state: 1, children: [subdir], hits: 0, lines: 0, ratio: 0 }

    const items = collectItems(root)
    const names = items.map(i => i.name)
    expect(names).toContain('sub')
    expect(names).not.toContain('a.ts') // hidden because sub is closed
    expect(names).not.toContain('b.ts')
  })
})
