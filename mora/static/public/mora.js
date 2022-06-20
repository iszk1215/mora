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

export { Breadcrumb }
