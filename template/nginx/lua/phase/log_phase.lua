local metrics = require("metrics")
local pm = require("plugins.core.plugin_manager")

-- request may jump to log phase directly (such like jumped via modsecurity)
if not ngx.ctx.alb_ctx then
    return
end
metrics.log()
pm.log_hook(ngx.ctx.alb_ctx)
