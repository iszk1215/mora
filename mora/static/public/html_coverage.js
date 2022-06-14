(function() {
    function Breadcrumb(items) {
        let delm = '<i class="right angle icon divider"></i>'

        let tmp = []
        for (const item of items) {
            let b
            if ("href" in item) {
                b = '<a href="' + item.href + '">' + item.name + '</a>'
            } else {
                b = item.name
            }
            tmp.push('<span class="section">' + b + "</span>")

        }
        return {
            template:
                '<div class="ui breadcrumb">' + tmp.join(delm) + "</div>"
        }
    }

    const breadcrumb = function() {
        let [_, scm, owner, repo, cov, covIndex, entry, ...rest]
            = window.location.pathname.split('/')
        let path = ["", scm, owner, repo, "coverages"].join("/")

        return [
            { href: "/", name: "Top" },
            { name: [scm, owner, repo].join("/") },
            { href: path, name: "coverages" },
            { name: "#" + covIndex }]
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
