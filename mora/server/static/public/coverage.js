import { Breadcrumb } from '/public/mora.js'

(function() {
    let chart

    const breadcrumb = function() {
        let [_, scm, owner, repo, cov, covIndex, entry, ...rest]
            = window.location.pathname.split('/')

        return [
            { href: "/", name: "Top" },
            { name: [scm, owner, repo].join("/") },
            { name: "Coverages" },
        ]
    }()


    let chartData = {
        "type": "line",
        "data": {
            "datasets": [],
            "labels": null
        },
        "options": {
            "scales": {
                "x": {
                    "type": "time",
                    "position": "bottom",
                    "title": {}
                },
                "y": {
                    "type": "linear",
                    "position": "left",
                    "title": {
                        "display": true,
                        "text": "Coverage %"
                    }
                }
            },
            "animation": {
                "duration": 0
            },
            "plugins": {
                "colorschemes": {
                    "scheme": "tableau.Classic10"
                }
            }
        }

    }

    function preprocess(coverages) {
        // console.log(coverages)
        coverages.reverse() // to yonger first
        for (const cov of coverages) {
            let hits = 0, lines = 0
            for (const e of cov.entries) {
                hits += e.hits
                lines += e.lines
            }
            cov.hits = hits
            cov.lines = lines
        }
    }

    function update_chart(coverages) {
        let map = {}

        let hasMultiEntries = coverages.reduce(
            (flag, cov) => flag || cov.entries.length > 1, false)
        if (hasMultiEntries)
            map["total"] = []

        for (const cov of coverages) {
            for (const e of cov.entries) {
                if (!(e.name in map))
                    map[e.name] = []
                map[e.name].push({ "x": cov.time, "y": e.hits * 100.0 / e.lines })
            }
            if (hasMultiEntries)
                map["total"].push({ "x": cov.time, "y": cov.hits * 100.0 / cov.lines })
        }
        let datasets = []
        for (const k in map) {
            let label = k == "_default" ? "coverage" : k
            datasets.push({ "borderWidth": 1, "label": label, "data": map[k] })
        }

        chart.data.datasets = datasets
        chart.update()
    }

    async function load_and_update(proxy) {
        const data = await fetch("/api" + window.location.pathname)
        const json = await data.json()
        preprocess(json)
        proxy.coverages = json
        update_chart(json)
    }

    const app = {
        components: { breadcrumb: Breadcrumb(breadcrumb) },
        delimiters: ['[[', ']]'],
        data() {
            return {
                coverages: [],
            }
        },
        methods: {
            formattedRatio(hits, lines) {
                return (hits * 100.0 / lines).toFixed(1)
            },
            formattedTime(time) {
                return luxon.DateTime.fromISO(time).toLocaleString(
                    luxon.DateTime.DATETIME_FULL)
            },
            async reload(e) {
                load_and_update(this)
            }
        },
        mounted() {
            const ctx = document.getElementById("chart1").getContext("2d")
            chart = new Chart(ctx, chartData)
            load_and_update(this)
        }
    };

    Vue.createApp(app).mount("#app")
})()
