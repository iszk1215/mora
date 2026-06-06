import React from 'react'
import hljs from 'highlight.js'

export function markupCode(code: string, blocks: number[][]): string[] {
  code = code.replace(/\s+$/g, '') // remove trailing '\n'
  const tmp = hljs.highlightAuto(code)
  const lines = tmp.value.split('\n')

  const blockIter = {
    curr: 0,
    list: blocks, // sorted
    next() {
      if (this.curr >= this.list.length) { return null }
      return this.list[this.curr++]
    }
  }

  const checkSpan = (line: string): string[] => {
    const spans = []
    let i = 0
    while (i < line.length) {
      const tmp = line.slice(i)
      if (tmp.startsWith('<span')) {
        const e = tmp.indexOf('>')
        spans.push(line.slice(i, i + e + 1))
        i += e + 1
      } else if (tmp.startsWith('</span>')) {
        spans.pop()
        i += '</span>'.length
      } else { ++i }
    }
    return spans
  }

  const lst = []
  let block = blockIter.next()
  let lastSpan = ''
  for (let i = 0; i < lines.length; ++i) {
    const lineno = i + 1
    let line = lines[i]

    if (line.length > 0) {
      line = lastSpan + line
      lastSpan = ''
      const spans = checkSpan(line)
      if (spans.length > 0) {
        for (let j = 0; j < spans.length; ++j) {
          line = line + '</span>'
          lastSpan += spans[j]
        }
      }
    }

    const prefix = `    ${lineno}`.slice(-4)
    const text = prefix + '  ' + line

    let color = ''
    while ((block != null) && lineno > block[1]) { block = blockIter.next() }
    if ((block != null) && lineno >= block[0] && lineno <= block[1]) {
      if (block[2] > 0) {
        color = 'hit'
      } else {
        color = 'miss'
      }
    }
    const spanClass = color === 'hit' ? 'hit bg-green-300' : color === 'miss' ? 'miss bg-red-200' : ''
    lst.push(`<span class="${spanClass}" style="display: inline-block; width: 100%; padding-left: 10px">${text}</span>`)
  }

  return lst
}

interface CodeViewProps {
  path: string
  code: string
  blocks: any
}

export const CodeView = (props: CodeViewProps): React.JSX.Element => {
  const blocks = props.blocks
  let hit = 0, miss = 0
  for (const block of blocks) {
    const lines = block[1] - block[0] + 1
    if (block[2] > 0)
      hit += lines
    else
      miss += lines
  }

  const lst = markupCode(props.code, props.blocks)
  return <div>
    <h1 className="text-3xl my-2">{props.path}</h1>
    <div className="flex my-2">
      <div className="rounded bg-green-300 px-2 mr-2">
        <span className="pr-2 font-bold">Hit</span>
        {hit}
      </div>
      <div className="rounded bg-pink-300 px-2 mr-2">
        <span className="pr-2 font-bold">Miss</span>
        {miss}
      </div>
    </div>
    <pre dangerouslySetInnerHTML={{ __html: lst.join('\n') }}
      style={{ border: 'solid 1px darkgray', padding: '0px' }}
    />
  </div>
}
