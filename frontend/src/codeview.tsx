import React, { useEffect } from 'react'
import hljs from 'highlight.js'

// let darkMode = true
// let linkElement: Element | null = null

var hitBackgroundColor = "bg-green-300"
var missBackgroundColor = "bg-red-200"

export function loadDarkModeFromCookie(): boolean {
  const cookies = document.cookie
  // console.log("cookie:", cookies)
  if (cookies === '') {
    for (const cookie of cookies.split(';')) {
      const [key, value] = cookie.split('=')
      if (key === 'darkMode' && value === '1') { return true }
    }
  }
  return false
}

export function markupCode(code: string, blocks: number[][]): string[] {
  code = code.replace(/\s+$/g, '') // remove trailing '\n'
  const tmp = hljs.highlightAuto(code)
  const lines = tmp.value.split('\n')
  // console.log(tmp.value)

  const blockIter = {
    curr: 0,
    list: blocks, // sorted
    next() {
      if (this.curr >= this.list.length) { return null }
      return this.list[this.curr++]
    }
  }

  const checkSpan = (line: string): string[] => {
    // console.log("checkSpan: " + line)
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
    // console.log(spans)
    return spans
  }

  const lst = []
  let block = blockIter.next()
  let lastSpan = ''
  for (let i = 0; i < lines.length; ++i) {
    const lineno = i + 1
    let line = lines[i]

    // console.log(lines[i])
    // console.log("lastSpan=" + lastSpan)
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
    lst.push(`<span class="${color}" style="display: inline-block; width: 100%; padding-left: 10px">${text}</span>`)
  }

  return lst
}

function setStyle(darkMode: boolean): void {
  // console.log("setStyle: ", darkMode)

  // source code highlight theme
  const themeURL = 'https://unpkg.com/@highlightjs/cdn-assets@11.5.1/styles/'
  const hrefDark = themeURL + 'github-dark.min.css'
  const hrefLight = themeURL + 'github.min.css'

  const link = document.createElement('link')
  link.rel = 'stylesheet'
  link.type = 'text/css'
  link.href = darkMode ? hrefDark : hrefLight

  const head = document.getElementsByTagName('head')[0]
  // if (linkElement != null) { linkElement.remove() }
  head.appendChild(link)
  // linkElement = link

  // source code line background
  for (const e of document.querySelectorAll<HTMLElement>('.hit')) {
    e.classList.add(hitBackgroundColor)
  }
  for (const e of document.querySelectorAll<HTMLElement>('.miss')) {
    e.classList.add(missBackgroundColor)
  }

  /*
    // toggle button
    const button = document.getElementById('darkModeButton')
    if (darkMode) {
      button.classList.add('active')
    } else {
      button.classList.remove('active')
    }
    */
}

/*
function _toggleDarkMode(button) {
  darkMode = !darkMode
  document.cookie = "darkMode=" + (darkMode ? "1" : "0")
  setStyle()
}
*/

interface CodeViewProps {
  path: string
  code: string
  blocks: any
}

export const CodeView = (props: CodeViewProps): React.JSX.Element => {
  // darkMode = loadDarkModeFromCookie()

  const blocks = props.blocks
  let hit = 0, miss = 0
  for (const block of blocks) {
    const lines = block[1] - block[0] + 1
    if (block[2] > 0)
      hit += lines
    else
      miss += lines
  }

  useEffect(() => {
    setStyle(false)
  })

  const lst = markupCode(props.code, props.blocks)
  // console.log(lst)
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
