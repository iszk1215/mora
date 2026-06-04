import React from 'react'
import { FileData } from './core'
import { DefaultLink } from './util'
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome'
import { faFolder, faFolderOpen } from '@fortawesome/free-regular-svg-icons'

export interface Item {
  name: string
  type: string
  depth: number
  state: number
  children: Item[]

  hits: number
  lines: number
  ratio: number

  file?: FileData
}

export const forEachItem = (item: Item, func: (item: Item) => boolean): void => {
  const flag = func(item)
  if (flag) {
    for (const child of item.children) {
      forEachItem(child, func)
    }
  }
}

export const list2tree = (files: FileData[]): Item => {
  // console.log('list2tree')
  const makeItem = (name: string, type: string, depth: number): Item => {
    return {
      name,
      type,
      hits: 0,
      lines: 0,
      ratio: 0.0,
      state: 0,
      depth,
      children: [],
      file: undefined
    }
  }

  const root = makeItem('', 'dir', 0)
  root.state = 1

  for (const f of files) {
    const tmp = f.filename.split('/')
    let parentDir = root
    let depth = 0
    for (const dirName of tmp.slice(0, -1)) {
      const tmp = parentDir.children.find((x: Item) => x.name === dirName)
      if (tmp !== undefined) {
        parentDir = tmp
      } else {
        const dir = makeItem(dirName, 'dir', depth)
        parentDir.children.push(dir)
        parentDir = dir
      }
      depth++
    }
    const item = makeItem(tmp[tmp.length - 1], 'file', depth)
    item.hits = f.hits
    item.lines = f.lines
    item.file = f
    parentDir.children.push(item)
  }

  // directory first
  const cmpItem = (a: Item, b: Item): number => {
    const cmp = (c: string, d: string): number => {
      return c === d ? 0 : (c < d ? -1 : 1)
    }
    return a.type !== b.type ? cmp(a.type, b.type) : cmp(a.name, b.name)
  }

  forEachItem(root, (item) => {
    item.children.sort(cmpItem)
    return true
  })

  const calcDirCoverage = (item: Item): void => {
    if (item.type !== 'dir') {
      item.ratio = item.hits * 100.0 / item.lines
      return
    }
    item.hits = 0
    item.lines = 0
    for (const child of item.children) {
      calcDirCoverage(child) // depth first
      item.hits += child.hits
      item.lines += child.lines
    }
    item.ratio = item.hits * 100.0 / item.lines
  }

  calcDirCoverage(root)

  forEachItem(root, (item) => {
    item.state = 1
    return true
  })

  return root
}

interface TableProp {
  items: Item[]
  selectItem: (item: Item) => void
}

interface Config {
  positiveThreshold: number
  negativeThreshold: number
}

const Table = (props: TableProp): JSX.Element => {
  const rows: JSX.Element[] = []

  // TODO: user config
  const config: Config = { positiveThreshold: 90, negativeThreshold: 70 }

  const getClass = (item: Item): string => {
    if (item.ratio > config.positiveThreshold)
      return "border bg-green-100"
    else if (item.ratio < config.negativeThreshold)
      return "border bg-red-100"
    return ""
  }

  props.items.forEach((item: Item, i: number) => {
    // console.log(item)
    const elems: JSX.Element[] = []
    for (let j = 0; j < item.depth; j++) {
      elems.push(<FontAwesomeIcon key={j} icon={faFolder} fixedWidth className="opacity-0 mr-1" />)
    }
    if (item.type === 'dir') {
      if (item.state === 1) {
        elems.push(<FontAwesomeIcon key={99} icon={faFolderOpen} fixedWidth className="mr-1" />)
      } else {
        elems.push(<FontAwesomeIcon key={99} icon={faFolder} fixedWidth className="mr-1" />)
      }
      elems.push(
        <a style={{ cursor: 'pointer' }} key={100}
          onClick={() => { props.selectItem(item) }}>
          {item.name}
        </a>)
    } else {
      elems.push(
        <DefaultLink key={100} to={item.file!.filename}>{item.name}</DefaultLink>)
    }

    const elem = (<tr key={i} className={getClass(item)}>
      <td className="border">{elems}</td>
      <td className="border">{item.hits}</td>
      <td className="border">{item.lines - item.hits}</td>
      <td className="border">{item.lines}</td>
      <td className="border">{item.ratio.toFixed(1)}%</td>
    </tr>)
    rows.push(elem)
  })

  return (<div>
    <table className="w-full border table-auto">
      <thead>
        <tr className="border">
          <th>Filename</th>
          <th>Hit</th>
          <th>Miss</th>
          <th>Total</th>
          <th>Coverage</th>
        </tr>
      </thead>
      <tbody>
        {rows}
      </tbody>
    </table>
  </div>)
}

interface BrowserProp {
  files: FileData[]
}

export const collectItems = (root: Item): Item[] => {
  const items: Item[] = []
  forEachItem(root, (item) => {
    if (item.name !== '') {
      items.push(item)
    }
    return item.type === 'dir' && item.state === 1
  })
  return items
}

export const updateTree = (src: Item, selectedItem: Item): Item => {
  const children: Item[] = []
  for (const child of src.children) {
    children.push(updateTree(child, selectedItem))
  }

  let state = src.state
  if (src === selectedItem) {
    state = state === 0 ? 1 : 0
  }

  return { ...src, state, children }
}

export const Browser = (props: BrowserProp): JSX.Element => {
  const files = props.files

  const [root, setRoot] = React.useState(() => { return list2tree(files) })
  const items = collectItems(root)

  const selectItem = (item: Item): void => {
    console.log('selectItem')
    if (item.type === 'dir') {
      const newRoot = updateTree(root, item)
      setRoot(newRoot)
    }
  }

  return <Table items={items} selectItem={selectItem} />
}
