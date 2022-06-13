(function() {
    const app = {
        delimiters: ['[[', ']]'],
        data() {
            return {
                scms: [],
            }
        },
        async mounted() {
            const data = await fetch('/api/scms')
            const json = await data.json()
            this.scms = json
        }
    };

    Vue.createApp(app).mount("#app")
})()
