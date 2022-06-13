(function() {
    function preprocess(coverages) {
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

    function update(coverages) {
        let map = { "total": [] }

        for (const cov of coverages) {
            for (const e of cov.entries) {
                if (!(e.name in map)) {
                    map[e.name] = []
                }
                map[e.name].push(
                    { "x": cov.time, "y": e.hits * 100.0 / e.lines })
            }
            map["total"].push({ "x": cov.time, "y": cov.hits * 100.0 / cov.lines })
        }
        let datasets = []
        for (const k in map) {
            // console.log(map[k])
            datasets.push({
                "borderWidth": 1,
                "label": k,
                /*
                "pointBorderWidth": 0,
                "pointRadius": 5,
                "pointHitRadius": 0,
                "pointHoverRadius": 0,
                "pointHoverBorderWidth": 0,
                */
                "data": map[k],
            })
        }

        let chart = {
            "type": "line",
            "data": {
                "datasets": datasets,
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

        const ctx = document.getElementById("chart1").getContext("2d")
        new Chart(ctx, chart)
    }

    const app = {
        delimiters: ['[[', ']]'],
        data() {
            return {
                coverages: [],
                breadcrumbs: [],
            }
        },
        methods: {
            formattedRatio(hits, lines) {
                return (hits * 100.0 / lines).toFixed(1)
            },
            formattedTime(time) {
                //console.log(luxon)
                return luxon.DateTime.fromISO(time).toLocaleString(
                    luxon.DateTime.DATETIME_FULL)
            },
        },
        async mounted() {
            const data = await fetch("/api" + window.location.pathname)
            const json = await data.json()
            preprocess(json)
            this.coverages = json

            let [_, scm, owner, repo, ...rest] = window.location.pathname.split('/')
            console.log(scm, owner, repo)

            this.breadcrumbs = [{ href: "/", name: "Top" },
            { name: [scm, owner, repo].join("/") },
            { name: "coverages" }]

            update(json)
        }
    };

    Vue.createApp(app).mount("#app")
})()
