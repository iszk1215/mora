import { Breadcrumb } from '/public/mora.js'
import hljs from 'https://unpkg.com/@highlightjs/cdn-assets@11.5.1/es/highlight.min.js';

(function() {
    hljs.highlightAll()

    let apiBaseURL = "/api" + window.location.pathname

    const breadcrumb = function() {
        let [_, scm, owner, repo, cov, covIndex, entry, ...rest]
            = window.location.pathname.split('/')
        let path = ["", scm, owner, repo, "coverages"].join("/")

        return [
            { href: "/", name: "Top" },
            { name: [scm, owner, repo].join("/") },
            { href: path, name: "Coverages" },
            { name: "#" + covIndex }]
    }()


    async function print_code(proxy, file) {
        const url = apiBaseURL + "/files/" + file.filename
        const data = await fetch(url)
        const json = await data.json()
        // console.log(json)
        const tmp = hljs.highlightAuto(json.code)
        const lines = tmp.value.split("\n")

        const blockIter = {
            curr: 0,
            list: json.blocks, // sorted
            next() {
                if (this.curr >= this.list.length)
                    return null
                return this.list[this.curr++]
            }
        }

        let lst = []
        let block = blockIter.next()
        for (let i = 0; i < lines.length; ++i) {
            let lineno = i + 1
            let prefix = ('    ' + lineno).slice(-4)
            let text = prefix + "  " + lines[i];

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
            lst.push('<span class="' + color + '" style="display: inline-block; width: 100%; padding-left: 10px">' + text + "</span>")
        }
        proxy.src = lst.join("\n")
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
                show_code: false,
                src: null,
            }
        },
        components: {
            breadcrumb: Breadcrumb(breadcrumb),
        },
        methods: {
            formattedTime(time) {
                return luxon.DateTime.fromISO(time).toLocaleString(
                    luxon.DateTime.DATETIME_FULL)
            },
            selectFile(file) {
                // console.log(file)
                print_code(this, file)
                this.show_code = true
            },
        },
        async mounted() {
            let baseURL = "/api" + window.location.pathname
            let url = apiBaseURL + "/files"
            // console.log(url)
            const data = await fetch(url)
            const json = await data.json()
            console.log(json)

            for (let file of json.files) {
                file.ratio = file.hits * 100.0 / file.lines
                if (file.ratio < 50) {
                    file.clazz = "negative"
                } else if (file.ratio > 80) {
                    file.clazz = "positive"
                }
            }
            this.files = json.files
            this.meta = json.meta
        }
    };

    Vue.createApp(app).mount("#app")
})()
