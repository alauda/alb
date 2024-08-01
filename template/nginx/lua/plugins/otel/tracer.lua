local resource_new = require("opentelemetry.resource").new
local our_exporter_client_new = require("plugins.otel.tracer_http_client").new
local otlp_exporter_new = require("opentelemetry.trace.exporter.otlp").new
local batch_span_processor_new = require("opentelemetry.trace.batch_span_processor").new
local tracer_provider_new = require("opentelemetry.trace.tracer_provider").new
local always_off_sampler_new = require("opentelemetry.trace.sampling.always_off_sampler").new
local always_on_sampler_new = require("opentelemetry.trace.sampling.always_on_sampler").new
local parent_base_sampler_new = require("opentelemetry.trace.sampling.parent_base_sampler").new
local trace_id_ratio_sampler_new = require("opentelemetry.trace.sampling.trace_id_ratio_sampler").new

local sampler_factory = {always_off = always_off_sampler_new, always_on = always_on_sampler_new, parent_base = parent_base_sampler_new, trace_id_ratio = trace_id_ratio_sampler_new}

local _M = {}
---comment
---@param conf OtelConf
---@return Sampler? sampler
---@return string? error
function _M.get_sampleer(conf)
    local sampler_name = conf.sampler.name
    if sampler_name == "always_on" or sampler_name == "always_off" then
        return sampler_factory[sampler_name](), nil
    end
    local s_opt = conf.sampler.options
    if s_opt == nil then
        return nil, "no opt"
    end
    local fraction = 0.5
    if s_opt.fraction ~= nil then
        fraction = tonumber(s_opt.fraction)
        if fraction == nil then
            return nil, "invalid fraction " .. tostring(s_opt.fraction)
        end
    end

    if sampler_name == "trace_id_ratio" then
        return sampler_factory[sampler_name](fraction), nil
    end

    if sampler_name ~= "parent_base" then
        return nil, "sampler not exist"
    end
    if sampler_name == "parent_base" and sampler_factory[s_opt.parent_name] == nil then
        return nil, "no parent sampler"
    end
    local root_sampler = sampler_factory[s_opt.parent_name](fraction)
    return sampler_factory[sampler_name](root_sampler), nil
end

---comment
---@param conf OtelConf
---@param resource_attrs table
---@return Tracer? tracer
---@return string? error
function _M.create_tracer(conf, resource_attrs)
    local collect_request_header = {["Content-Type"] = "application/json"}
    -- our_exporter_client_new are skip ssl_verify in default
    local exporter = otlp_exporter_new(our_exporter_client_new(conf.exporter.collector.address, conf.exporter.collector.request_timeout, collect_request_header))
    -- create span processor
    local batch_span_processor = batch_span_processor_new(exporter, conf.exporter.batch_span_processor)
    -- create sampler
    local sampler, err = _M.get_sampleer(conf)
    if err ~= nil then
        return nil, err
    end
    -- create tracer provider
    local tp = tracer_provider_new(batch_span_processor, {resource = resource_new(unpack(resource_attrs)), sampler = sampler})
    -- create tracer
    return tp:tracer("opentelemetry-lua")
end

return _M
