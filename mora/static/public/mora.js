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

function Browser() {
    function forEachItem(item, func) {
        let flag = func(item)
        if (flag) {
            for (const child of item.children) {
                forEachItem(child, func)
            }
        }
    }

    function collectItems(root) {
        let items = []
        forEachItem(root, (item) => {
            if (item.name != "")
                items.push(item)
            return item.type == "dir" && item.state == 1
        })
        return items
    }

    function list2tree(files) {
        const Item = (name, type, depth) => {
            return {name: name, type: type, hits: 0, lines: 0, state: 0,
                children: {}, depth: depth}
        }

        let root = Item("", "dir", 0)
        root.state = 1

        for (const f of files) {
            let tmp = f.filename.split("/")
            let parentDir = root
            let depth = 0
            for (let dirName of tmp.slice(0, -1)) {
                if (dirName in parentDir.children) {
                    parentDir = parentDir.children[dirName]
                } else {
                    const dir = Item(dirName, "dir", depth)
                    parentDir.children[dirName] = dir
                    parentDir = dir
                }
                depth++
            }
            const item = Item(tmp[tmp.length-1], "file", depth)
            item.hits = f.hits
            item.lines = f.lines
            item.ratio = f.ratio
            item.path = f.filename
            parentDir.children[f.filename] = item
        }

        // directory first
        const cmpItem = (a, b) => {
            const cmp = (c, d) => {
                return c == d ? 0 : (c < d ? -1 : 1)
            }
            return a.type != b.type ? cmp(a.type, b.type) : cmp(a.name, b.name)
        }

        forEachItem(root, (item) => {
            item.children = Object.values(item.children).sort(cmpItem)
            return true
        })

        const calcDirCoverage = (item) => {
            if (item.type != "dir")
                return
            item.hits = 0
            item.lines = 0
            for (const child of item.children) {
                calcDirCoverage(child) // depth first
                item.hits += child.hits
                item.lines += child.lines
            }
            item.ratio = item.hits * 100.0 / item.lines
        }
        calcDirCoverage(root)

        forEachItem(root, (item) => {
            //console.log(item.name, item.children.length)
            if (item.type == "dir" && item.children.length == 1) {
                item.state = 1
                item.children[0].state = 1
            }
            return true
        })

        return root
    }


    return {
        data() {
            return {
                items: [],
                root: {},
            }
        },
        emits: ["selectFile"],
        methods: {
            selectItem(item) {
                //console.log("selectItem")
                //console.log(item)
                if (item.type == "file") {
                    this.$emit("selectFile", item)
                } else { // "dir"
                    item.state = item.state == 0 ? 1 : 0
                    this.update()
                }
            },
            update() {
                this.items = collectItems(this.root)
            },
            setData(files) {
                const root = list2tree(files)
                forEachItem(root, (item) => {
                    if (item.ratio < 50) 
                        item.clazz = "negative"
                    else if (item.ratio > 80)
                        item.clazz = "positive"
                    return true
                })
                this.root = root
                this.update()
            },
        },
        props: ["files"],
        watch: {
            files: function(now, old) {
                this.setData(now)
            },
        },
        template:
        `<div>
        <table class="ui fixed very compact selectable celled striped table" style="line-height: 1.0;">
        <thead>
                    <tr style="line-height: 0.0;">
                        <th>Filename</th>
                        <th>Hit</th>
                        <th>Total</th>
                        <th>Coverage</th>
                    </tr>
        </thead>
        <tbody>
        <tr v-for="item in items" :class="item.clazz">
            <td>
            <i class="icon" v-for="n in item.depth"></i>
            <i class="folder open outline icon" v-if="item.type == 'dir' && item.state == 1"></i>
            <i class="folder outline icon" v-if="item.type == 'dir' && item.state == 0"></i>
            <!--
            <i class="file outline icon" v-if="item.type == 'file'"></i>
            -->
            <a href="javascript:void(0)" @click="selectItem(item)">{{item.name}}</a></td>
            <td class="right aligned">{{item.hits}}</td>
            <td class="right aligned">{{item.lines}}</td>
            <td class="right aligned">{{item.ratio.toFixed(1)}}%</td>
        </tr>
        </tbody>
        </table>
        </div>`
    }
}

export { Breadcrumb, Browser }
