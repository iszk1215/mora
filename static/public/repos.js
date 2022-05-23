(function() {
    const app = {
        delimiters: ['[[', ']]'],
        data() {
            return {
                repos: []
            }
        },
        async mounted() {
            const data = await fetch("/api/repos")
            const json = await data.json()
            this.repos = json
        },
    }
    Vue.createApp(app).mount("#app")
})()
