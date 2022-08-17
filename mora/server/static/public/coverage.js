import { Breadcrumb } from "/public/mora.js";
//import {Chart} from 'https://cdn.jsdelivr.net/npm/chart.js@3.6.2/dist/chart.min.js'
//import ChartDataLabels from 'https://cdn.jsdelivr.net/npm/chartjs-plugin-datalabels@2.0.0'
(function () {
  let chart;

  const breadcrumb = function () {
    const [_, scm, owner, repo, cov, covIndex, entry, ...rest] = window.location
      .pathname.split("/");

    return [
      { href: "/", name: "Top" },
      { name: [scm, owner, repo].join("/") },
      { name: "Coverages" },
    ];
  }();

  const chartData = {
    type: "line",
    data: {
      datasets: [],
      labels: null,
    },
    options: {
      onClick: function (_ev, elements, chart) {
        // console.log(chart.data)
        if (elements.length == 1) {
          const e = elements[0];
          const dataset = chart.data.datasets[e.datasetIndex];
          if (dataset.label != "total") {
            const d = dataset.data[e.index];
            const url = `${window.location}/${d.index}/${dataset.entry}`;
            window.location = url;
          }
        }
      },
      scales: {
        x: {
          type: "time",
          position: "bottom",
          title: {},
        },
        y: {
          type: "linear",
          position: "left",
          title: {
            display: true,
            text: "Coverage %",
          },
        },
      },
      animation: {
        duration: 0,
      },
      plugins: {
        colorschemes: {
          scheme: "tableau.Classic10",
        },
        // datalabels: {
        //     align: "top",
        //     // backgroundColor: function(context) {
        //     //     return context.dataset.backgroundColor
        //     // },
        //     // borderRadius: 4,
        //     formatter: function(value, context) {
        //         return `#${value.index}`
        //     },
        // },
        tooltip: {
          callbacks: {
            label: function (context) {
              const data = context.dataset.data[context.dataIndex];
              const label = context.dataset.label;
              const y = context.raw.y;
              return `#${data.index}: ${label} ${y.toFixed(1)}%`;
            },
          },
        },
      },
    },
  };

  function preprocess(coverages) {
    // console.log(coverages)
    coverages.reverse(); // to yonger first
    for (const cov of coverages) {
      let hits = 0, lines = 0;
      for (const e of cov.entries) {
        hits += e.hits;
        lines += e.lines;
      }
      cov.hits = hits;
      cov.lines = lines;
    }
  }

  function update_chart(coverages) {
    const map = {};

    const hasMultiEntries = coverages.reduce(
      (flag, cov) => flag || cov.entries.length > 1,
      false,
    );
    if (hasMultiEntries) {
      map["total"] = [];
    }

    for (const cov of coverages) {
      for (const e of cov.entries) {
        if (!(e.name in map)) {
          map[e.name] = [];
        }
        map[e.name].push(
          { x: cov.time, y: e.hits * 100.0 / e.lines, index: cov.index },
        );
      }
      if (hasMultiEntries) {
        map["total"].push(
          { x: cov.time, y: cov.hits * 100.0 / cov.lines, index: cov.index },
        );
      }
    }
    const datasets = [];
    for (const k in map) {
      const label = k == "_default" ? "coverage" : k;
      datasets.push({ borderWidth: 1, label: label, data: map[k], entry: k });
    }

    chart.data.datasets = datasets;
    chart.update();
  }

  async function load_and_update(vm) {
    const data = await fetch("/api" + window.location.pathname);
    const json = await data.json();
    preprocess(json);
    vm.coverages = json;
    update_chart(json);
  }

  const app = {
    components: { breadcrumb: Breadcrumb(breadcrumb) },
    delimiters: ["[[", "]]"],
    data() {
      return {
        coverages: [],
      };
    },
    methods: {
      formattedRatio(hits, lines) {
        return (hits * 100.0 / lines).toFixed(1);
      },
      formattedTime(time) {
        return luxon.DateTime.fromISO(time).toLocaleString(
          luxon.DateTime.DATETIME_FULL,
        );
      },
      reload(_e) {
        load_and_update(this);
      },
    },
    mounted() {
      const ctx = document.getElementById("chart1").getContext("2d");
      // Chart.register(ChartDataLabels)
      chart = new Chart(ctx, chartData);
      load_and_update(this);
    },
  };

  Vue.createApp(app).mount("#app");
})();
