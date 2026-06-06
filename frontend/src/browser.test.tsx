import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { list2tree, collectItems, updateTree, Browser } from './browser'
import { FileData } from './core'

function makeFile(filename: string, hits = 10, lines = 10): FileData {
  return { filename, hits, lines }
}

describe('list2tree', () => {
  it('creates root with files at top level', () => {
    const files: FileData[] = [makeFile('main.go')]
    const root = list2tree(files)
    expect(root.type).toBe('dir')
    expect(root.children).toHaveLength(1)
    expect(root.children[0].name).toBe('main.go')
    expect(root.children[0].type).toBe('file')
  })

  it('groups files into directories', () => {
    const files: FileData[] = [makeFile('src/main.go'), makeFile('src/util.go'), makeFile('README.md')]
    const root = list2tree(files)
    expect(root.children).toHaveLength(2)

    const srcDir = root.children.find((c) => c.name === 'src')
    expect(srcDir).toBeDefined()
    expect(srcDir!.type).toBe('dir')
    expect(srcDir!.children).toHaveLength(2)

    const readme = root.children.find((c) => c.name === 'README.md')
    expect(readme).toBeDefined()
    expect(readme!.type).toBe('file')
  })

  it('aggregates directory hits and lines from children', () => {
    const files: FileData[] = [
      makeFile('src/a.go', 50, 100),
      makeFile('src/b.go', 30, 50),
    ]
    const root = list2tree(files)
    const srcDir = root.children[0]
    expect(srcDir.hits).toBe(80)
    expect(srcDir.lines).toBe(150)
  })

  it('calculates directory ratio', () => {
    const files: FileData[] = [
      makeFile('src/a.go', 75, 100),
      makeFile('src/b.go', 25, 100),
    ]
    const root = list2tree(files)
    const srcDir = root.children[0]
    expect(srcDir.lines).toBe(200)
    expect(srcDir.hits).toBe(100)
    expect(srcDir.ratio).toBe(50.0)
  })

  it('sorts directories before files', () => {
    const files: FileData[] = [makeFile('z_file.go'), makeFile('a_dir/main.go')]
    const root = list2tree(files)
    expect(root.children[0].type).toBe('dir')
    expect(root.children[1].type).toBe('file')
  })
})

describe('collectItems', () => {
  it('returns all open items recursively', () => {
    const files: FileData[] = [makeFile('src/a.go'), makeFile('src/b.go')]
    const root = list2tree(files)
    const items = collectItems(root)
    expect(items).toHaveLength(3)
    expect(items[0].name).toBe('src')
    expect(items[1].name).toBe('a.go')
    expect(items[2].name).toBe('b.go')
  })
})

describe('updateTree', () => {
  it('toggles directory state from open to closed', () => {
    const files: FileData[] = [makeFile('src/a.go')]
    const root = list2tree(files)
    const srcDir = root.children[0]
    expect(srcDir.state).toBe(1)
    const toggled = updateTree(root, srcDir)
    expect(toggled.children[0].state).toBe(0)
  })

  it('toggles directory state from closed to open', () => {
    const files: FileData[] = [makeFile('src/a.go')]
    const root = list2tree(files)
    const srcDir = root.children[0]
    const once = updateTree(root, srcDir)
    const toggledDir = once.children[0]
    const twice = updateTree(once, toggledDir)
    expect(twice.children[0].state).toBe(1)
  })
})

describe('Browser', () => {
  it('renders file rows and coverage columns', () => {
    const files: FileData[] = [makeFile('main.go', 75, 100)]
    render(
      <MemoryRouter>
        <Browser files={files} />
      </MemoryRouter>
    )
    expect(screen.getByText('main.go')).toBeDefined()
    expect(screen.getByText('75')).toBeDefined()
    expect(screen.getByText('100')).toBeDefined()
    expect(screen.getByText('75.0%')).toBeDefined()
  })

  it('renders miss count', () => {
    const files: FileData[] = [makeFile('main.go', 75, 100)]
    render(
      <MemoryRouter>
        <Browser files={files} />
      </MemoryRouter>
    )
    expect(screen.getByText('25')).toBeDefined()
  })

  it('renders table header', () => {
    const files: FileData[] = [makeFile('main.go')]
    render(
      <MemoryRouter>
        <Browser files={files} />
      </MemoryRouter>
    )
    expect(screen.getByText('Filename')).toBeDefined()
    expect(screen.getByText('Hit')).toBeDefined()
    expect(screen.getByText('Miss')).toBeDefined()
    expect(screen.getByText('Total')).toBeDefined()
    expect(screen.getByText('Coverage')).toBeDefined()
  })

  it('renders directory rows', () => {
    const files: FileData[] = [makeFile('src/a.go'), makeFile('src/b.go')]
    render(
      <MemoryRouter>
        <Browser files={files} />
      </MemoryRouter>
    )
    expect(screen.getByText('src')).toBeDefined()
    expect(screen.getByText('a.go')).toBeDefined()
    expect(screen.getByText('b.go')).toBeDefined()
  })
})
