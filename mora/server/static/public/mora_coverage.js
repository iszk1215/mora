import { Breadcrumb, Browser } from '/public/mora.js'
import { CodeView } from '/public/codeview.js'

(function() {
    let apiBaseURL = "/api" + window.location.pathname

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
                source_file: {},
            }
        },
        computed: {
            FormattedRevision() {
                return this.meta.revision.substring(0, 10)
            },
            FormattedRatio() {
                return (this.meta.hits * 100 / this.meta.lines).toFixed(1)
            },
            FormattedTime() {
                return luxon.DateTime.fromISO(this.meta.time).toLocaleString(
                    luxon.DateTime.DATETIME_FULL)
            },
        },
        components: {
            breadcrumb: Breadcrumb(breadcrumb),
            browser: Browser(),
            codeview: CodeView(),
        },
        methods: {
            formattedTime(time) {
                return luxon.DateTime.fromISO(time).toLocaleString(
                    luxon.DateTime.DATETIME_FULL)
            },
            async selectFile(file) {
                // console.log("selectFile")
                const url = apiBaseURL + "/files/" + file.path
                const data = await fetch(url)
                const json = await data.json()
                this.source_file = {file: file, json: json}
            },
        },
        async mounted() {
            let baseURL = "/api" + window.location.pathname
            let url = apiBaseURL + "/files"
            const data = await fetch(url)
            const json = await data.json()
            this.files = json.files
            this.meta = json.meta
        }
    };

    Vue.createApp(app).mount("#app")
})()
