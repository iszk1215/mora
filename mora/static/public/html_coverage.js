import { Breadcrumb } from '/public/mora.js'

(function() {
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
                entry: { "revision": "" }
            }
        },
        components: { breadcrumb: Breadcrumb(breadcrumb) },
        methods: {
            formattedTime(time) {
                return luxon.DateTime.fromISO(time).toLocaleString(
                    luxon.DateTime.DATETIME_FULL)
            },
        },
        async mounted() {
            const data = await fetch("/api" + window.location.pathname)
            const json = await data.json()
            this.entry = json
        }
    };

    Vue.createApp(app).mount("#app")
})()
