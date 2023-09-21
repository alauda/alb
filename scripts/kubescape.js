const fs = require('fs')
res = JSON.parse(fs.readFileSync('./results.json', 'utf8'))
out = []
for (i of res.results) {
    cs = []
    for (c of i.controls) {
        if (c.status.status != "passed") {
            // console.log(c.status);
            cs.push(c)
        }
    }
    i.controls = cs
    if (cs.length > 0) {
        out.push(i)
    }
}
console.log(JSON.stringify(out, null, 4));