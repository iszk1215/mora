import { Breadcrumb, Browser } from '/public/mora.js'
import hljs from 'https://unpkg.com/@highlightjs/cdn-assets@11.5.1/es/highlight.min.js';

(function() {
    let darkMode = true
    let cssElment = null
    let apiBaseURL = "/api" + window.location.pathname

    hljs.highlightAll()

    const breadcrumb = function() {
        let [_, scm, owner, repo, cov, covIndex, entry, ...rest]
            = window.location.pathname.split('/')
        let path = ["", scm, owner, repo, "coverages"].join("/")

        return [
            { href: "/", name: "Top" },
            { name: [scm, owner, repo].join("/") },
            { href: path, name: "Coverages" },
            { name: "#" + covIndex },
            { name: entry },
        ]
    }()

    async function print_code(proxy, file) {
        const url = apiBaseURL + "/files/" + file.path
        const data = await fetch(url)
        const json = await data.json()
        // console.log(json)
        if (json.message) { // error
            return
        }
        json.code = json.code.replace(/\s+$/g,'') // remove trailing '\n'
        const tmp = hljs.highlightAuto(json.code)
        const lines = tmp.value.split("\n")
        //console.log(tmp.value)

        const blockIter = {
            curr: 0,
            list: json.blocks, // sorted
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
        proxy.selectedFile = file
        proxy.src = lst.join("\n")
        proxy.$nextTick(() => {
            proxy.show_code = true
            proxy.$nextTick(() => {
                setStyle()
            })
        })
    }

    function setStyle() {
        // source code highlight theme
        const themeURL = 'https://unpkg.com/@highlightjs/cdn-assets@11.5.1/styles/'
        const hrefDark = themeURL + 'github-dark.min.css'
        const hrefLight = themeURL + 'github.min.css'

        const link = document.createElement('link')
        link.rel = 'stylesheet'
        link.type = 'text/css'
        link.href = darkMode ? hrefDark : hrefLight

        const head = document.getElementsByTagName('head')[0]
        if (cssElment)
            cssElment.remove()
        head.appendChild(link)
        cssElment = link

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

    const app = {
        delimiters: ['[[', ']]'],
        data() {
            return {
                meta: {
                    hits: 0,
                    lines: 0,
                    time: "",
                    revision_url: "",
                    revision: ""
                },
                files: [],
                //root: [],
                show_code: false,
                selectedFile: {},
                src: null,
            }
        },
        components: {
            breadcrumb: Breadcrumb(breadcrumb),
            browser: Browser(),
        },
        methods: {
            formattedTime(time) {
                return luxon.DateTime.fromISO(time).toLocaleString(
                    luxon.DateTime.DATETIME_FULL)
            },
            selectFile(file) {
                console.log("selectFile")
                // console.log(file)
                print_code(this, file)
            },
            toggleDarkMode(ev) {
                _toggleDarkMode(ev.target)
            },
        },
        async mounted() {
            let baseURL = "/api" + window.location.pathname
            let url = apiBaseURL + "/files"
            // console.log(url)
            const data = await fetch(url)
            const json = await data.json()
            // console.log(json)

            this.files = json.files
            this.meta = json.meta


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
            // console.log("darkMode =", darkMode)
        }
    };

    Vue.createApp(app).mount("#app")
})()
