local metrics = require("metrics")
local pm = require("plugins.core.plugin_manager")

metrics.log()
pm.log_hook(ngx.ctx.alb_ctx)
