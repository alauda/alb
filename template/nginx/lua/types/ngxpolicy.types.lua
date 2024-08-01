---@class NgxPolicy
---@field certificate_map table<string,Certificate>
---@field http HttpPolicy
---@field stream StreamPolicy
---@field config CommonPolicyConfig
---@field backend_group 


---@class HttpPolicy
---@field tcp table<number,Policies>


---@alias CommonPolicyConfig table<string,CommonPolicyConfigVal>

---@class CommonPolicyConfigVal
---@field type string
---@field otel? OtelInCommon


---@class StreamPolicy
---@field tcp table<number,Policies>
---@field udp table<number,Policies>


---@alias Policies (Policy | nil)[]
---@class Certificate
---@field cert string
---@field key string


---@class OtelInCommon
---@field otel OtelConf


---@class OtelConf
---@field exporter? Exporter
---@field sampler? Sampler
---@field flags? Flags
---@field resource? table<string,string>


---@class Flags
---@field hide_upstream_attrs boolean
---@field report_http_reqeust_header boolean
---@field report_http_response_header boolean
---@field notrust_incoming_span boolean


---@class Exporter
---@field collector? Collector
---@field batch_span_processor? BatchSpanProcessor


---@class Collector
---@field address string
---@field request_timeout number /* int */


---@class BatchSpanProcessor
---@field max_queue_size number /* int */
---@field scheduled_delay number /* int */
---@field export_timeout number /* int */


---@class Sampler
---@field name string
---@field options? SamplerOptions


---@class SamplerOptions
---@field parent_name? string
---@field fraction? string


---@class Policy
---@field internal_dsl 
---@field upstream string
---@field rule string
---@field config? RuleConfigInPolicy
---@field SameInRuleCr SameInRuleCr
---@field SameInPolicy SameInPolicy
---@field source_type? string
---@field source_name? string
---@field source_ns? string


---@class SameInRuleCr
---@field url string
---@field rewrite_base string
---@field rewrite_target string
---@field enable_cors boolean
---@field cors_allow_headers string
---@field cors_allow_origin string
---@field backend_protocol string
---@field redirect_url string
---@field vhost string
---@field redirect_code number /* int */
---@field source? Source


---@class SameInPolicy
---@field rewrite_prefix_match? string
---@field rewrite_replace_prefix? string
---@field redirect_scheme? string
---@field redirect_host? string
---@field redirect_port? number /* int */
---@field redirect_prefix_match? string
---@field redirect_replace_prefix? string


---@class RuleConfigInPolicy
---@field rewrite_response? RewriteResponseConfig
---@field rewrite_request? RewriteRequestConfig
---@field timeout? TimeoutPolicyConfig
---@field otel? OtelInPolicy


---@class RewriteResponseConfig
---@field headers? table<string,string>
---@field headers_remove? 
---@field headers_add? table<string,>


---@class RewriteRequestConfig
---@field headers? table<string,string>
---@field headers_var? table<string,string>
---@field headers_remove? 
---@field headers_add? table<string,>
---@field headers_add_var? table<string,>


---@class TimeoutPolicyConfig
---@field proxy_connect_timeout_ms? number /* uint */
---@field proxy_send_timeout_ms? number /* uint */
---@field proxy_read_timeout_ms? number /* uint */


---@class Source
---@field name string
---@field namespace string
---@field type string


---@class OtelInPolicy
---@field otel_ref? string
---@field otel? OtelConf


