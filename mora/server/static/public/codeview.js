import hljs from 'https://unpkg.com/@highlightjs/cdn-assets@11.5.1/es/highlight.min.js';

let darkMode = true
let linkElement = null

// hljs.highlightAll()

function loadDarkModeFromCookie() {
    darkMode = false
    let cookies = document.cookie
    // console.log("cookie:", cookies)
    if (cookies) {
        for (let cookie of cookies.split(';')) {
            let [key, value] = cookie.split('=')
            if (key == "darkMode" && value == "1")
                darkMode = true
        }
    }
}

function make_code(code, blocks) {
    code = code.replace(/\s+$/g,'') // remove trailing '\n'
    const tmp = hljs.highlightAuto(code)
    const lines = tmp.value.split("\n")
    // console.log(tmp.value)

    const blockIter = {
        curr: 0,
        list: blocks, // sorted
        next() {
            if (this.curr >= this.list.length)
                return null
            return this.list[this.curr++]
        }
    }

    const checkSpan = (line) => {
        // console.log("checkSpan: " + line)
        let spans = []
        let i = 0
        while (i < line.length) {
            const tmp = line.slice(i)
            if (tmp.startsWith("<span")) {
                const e = tmp.indexOf(">")
                spans.push(line.slice(i, i + e + 1))
                i += e + 1
            } else if (tmp.startsWith("</span>")) {
                spans.pop()
                i += "</span>".length
            } else
                ++i
        }
        //console.log(spans)
        return spans
    }


    let lst = []
    let block = blockIter.next()
    let lastSpan = ""
    for (let i = 0; i < lines.length; ++i) {
        let lineno = i + 1
        //console.log(lines[i])
        //console.log("lastSpan=" + lastSpan)
        let line = lines[i]
        if (line.length > 0) {
            line = lastSpan + line
            lastSpan = ""
            const spans = checkSpan(line)
            if (spans.length > 0) {
                for (let j = 0; j < spans.length; ++j) {
                    line = line + "</span>"
                    lastSpan += spans[j]
                }
            }
        }


        let prefix = ('    ' + lineno).slice(-4)
        let text = prefix + "  " + line

        let color = ""
        while (block && lineno > block[1])
            block = blockIter.next()
        if (block && lineno >= block[0] && lineno <= block[1]) {
            if (block[2] > 0) {
                color = "hit"
            } else {
                color = "miss"
            }
        }
        lst.push(`<span class="${color}" style="display: inline-block; width: 100%; padding-left: 10px">${text}</span>`)
    }

    return lst
}

function setStyle() {
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
    if (linkElement)
        linkElement.remove()
    head.appendChild(link)
    linkElement = link

    // source code line background
    let hit = darkMode ? "darkblue" : "palegreen"
    let miss = darkMode ? "darkred" : "pink"
    for (const e of document.querySelectorAll('.hit'))
        e.style.background = hit
    for (const e of document.querySelectorAll('.miss'))
        e.style.background = miss

    // toggle button
    const button = document.getElementById('darkModeButton')
    if (darkMode) {
        button.classList.add('active')
    } else {
        button.classList.remove('active')
    }

    // hit/miss labels
    const set_color = function(e, dark, light) {
        let src = darkMode ? light : dark
        let dst = darkMode ? dark : light
        if (!e.classList.replace(src, dst))
            e.classList.add(dst)
    }
    const hitLabel = document.getElementById('hitLabel')
    const missLabel = document.getElementById('missLabel')
    set_color(hitLabel, 'blue', 'green')
    set_color(missLabel, 'red', 'pink')
}

function _toggleDarkMode(button) {
    darkMode = !darkMode
    document.cookie = "darkMode=" + (darkMode ? "1" : "0")
    setStyle()
}

function CodeView() {
    return {
        data() {
            return {
                selectedFile: {},
                src: "",
                visible: false,
            }
        },
        methods: {
            toggleDarkMode(ev) {
                _toggleDarkMode(ev.target)
            },
        },
        props: ["file", "darkMode"],
        watch: {
            file: function(now, old) {
                //console.log(now)
                const lst = make_code(now.json.code, now.json.blocks)
                this.src = lst.join("\n")
                this.selectedFile = now.file
                this.visible = true
                this.$nextTick(() => {
                    setStyle()
                })
            },
        },
        async mounted() {
            // console.log("CodeView.mounted")
            loadDarkModeFromCookie()
        },
        template:
        `<div class="ui container" v-if="visible">
           <h4 class="ui header">{{selectedFile.path}}</h4>
           Coverage {{(selectedFile.hits*100.0/selectedFile.lines).toFixed(1)}}%
            <div id="hitLabel" class="ui label">
                Hit
                <div class="ui detail">{{selectedFile.hits}} Lines</div>
            </div>
            <div id="missLabel" class="ui label">
                Miss
                <div class="ui detail">{{selectedFile.lines-selectedFile.hits}} Lines</div>
            </div>
           <button id="darkModeButton" class="ui right floated compact toggle button" v-on:click="toggleDarkMode($event)">Dark</button>
           <pre style="padding 0px; border: solid 1px darkgray;"><code class="hljs" v-html="src" style="padding: 0px;"></code></pre>
         </div>`
    }
}

export { CodeView }

