--- @alias CJSON_NULL userdata
--- @class NgxPolicy
--- @field backend_group BackendGroup[]
--- @field certificate_map table<string, Certificate>
--- @field config table<string, RefBox>
--- @field http HttpPolicy
--- @field stream StreamPolicy


--- @class HttpPolicy
--- @field tcp table<number, Policy[]>


--- @class StreamPolicy
--- @field tcp table<number, Policy[]>
--- @field udp table<number, Policy[]>


--- @class BackendGroup
--- @field backends Backend[]
--- @field mode string
--- @field name string
--- @field session_affinity_attribute string
--- @field session_affinity_policy string


--- @class Certificate
--- @field cert string
--- @field key string


--- @class RefBox
--- @field note string?
--- @field type string
--- @field auth AuthPolicy?
--- @field otel OtelConf?
--- @field redirect RedirectCr?
--- @field rewrite_request RewriteRequestConfig?
--- @field rewrite_response RewriteResponseConfig?
--- @field timeout TimeoutCr?


--- @class PolicyExt
--- @field auth AuthPolicy?
--- @field otel OtelConf?
--- @field redirect RedirectCr?
--- @field rewrite_request RewriteRequestConfig?
--- @field rewrite_response RewriteResponseConfig?
--- @field timeout TimeoutCr?


--- @class Backend
--- @field address string
--- @field ns string
--- @field otherclusters boolean
--- @field port number
--- @field svc string
--- @field weight number


--- @class Policy
--- @field backend_protocol string
--- @field config PolicyExtCfg
--- @field internal_dsl any[]
--- @field plugins string[]
--- @field rule string
--- @field source Source
--- @field subsystem string
--- @field to_location string?
--- @field upstream string
--- @field cors_allow_headers string
--- @field cors_allow_origin string
--- @field enable_cors boolean
--- @field rewrite_base string
--- @field rewrite_prefix_match string?
--- @field rewrite_replace_prefix string?
--- @field rewrite_target string
--- @field url string
--- @field vhost string


--- @class AuthPolicy
--- @field basic_auth BasicAuthPolicy?
--- @field forward_auth ForwardAuthPolicy?


--- @class LegacyExtInPolicy
--- @field cors_allow_headers string
--- @field cors_allow_origin string
--- @field enable_cors boolean
--- @field rewrite_base string
--- @field rewrite_prefix_match string?
--- @field rewrite_replace_prefix string?
--- @field rewrite_target string
--- @field url string
--- @field vhost string


--- @class OtelConf
--- @field exporter Exporter?
--- @field flags Flags?
--- @field resource table<string, string>
--- @field sampler Sampler?


--- @class PolicyExtCfg
--- @field refs table<string, string>
--- @field auth AuthPolicy?
--- @field otel OtelConf?
--- @field redirect RedirectCr?
--- @field rewrite_request RewriteRequestConfig?
--- @field rewrite_response RewriteResponseConfig?
--- @field timeout TimeoutCr?


--- @class RedirectCr
--- @field code number?
--- @field host string
--- @field port number?
--- @field prefix_match string
--- @field replace_prefix string
--- @field scheme string
--- @field url string


--- @class RewriteRequestConfig
--- @field headers table<string, string>
--- @field headers_add table<string, string[]>
--- @field headers_add_var table<string, string[]>
--- @field headers_remove string[]
--- @field headers_var table<string, string>


--- @class RewriteResponseConfig
--- @field headers table<string, string>
--- @field headers_add table<string, string[]>
--- @field headers_remove string[]


--- @class Source
--- @field source_name string
--- @field source_ns string
--- @field source_type string


--- @class TimeoutCr
--- @field proxy_connect_timeout_ms number?
--- @field proxy_read_timeout_ms number?
--- @field proxy_send_timeout_ms number?


--- @class Cors
--- @field cors_allow_headers string
--- @field cors_allow_origin string
--- @field enable_cors boolean


--- @class RewriteConf
--- @field rewrite_base string
--- @field rewrite_prefix_match string?
--- @field rewrite_replace_prefix string?
--- @field rewrite_target string
--- @field url string


--- @class Vhost
--- @field vhost string


--- @class BasicAuthPolicy
--- @field auth_type string
--- @field err string
--- @field realm string
--- @field secret table<string, BasicAuthHash>


--- @class Exporter
--- @field batch_span_processor BatchSpanProcessor?
--- @field collector Collector?


--- @class Flags
--- @field hide_upstream_attrs boolean
--- @field notrust_incoming_span boolean
--- @field report_http_request_header boolean
--- @field report_http_response_header boolean


--- @class ForwardAuthPolicy
--- @field always_set_cookie boolean
--- @field auth_headers table<string, string[]>
--- @field auth_request_redirect string[]
--- @field invalid_auth_req_cm_ref boolean
--- @field method string
--- @field signin_url string[]
--- @field upstream_headers string[]
--- @field url string[]


--- @class Sampler
--- @field name string
--- @field options (SamplerOptions|CJSON_NULL)


--- @class BasicAuthHash
--- @field algorithm string
--- @field hash string
--- @field name string
--- @field salt string


--- @class BatchSpanProcessor
--- @field inactive_timeout number
--- @field max_queue_size number


--- @class Collector
--- @field address string
--- @field request_timeout number


--- @class SamplerOptions
--- @field fraction (string|CJSON_NULL)
--- @field parent_name (string|CJSON_NULL)


