local _M = {}
-- {
--    "mode": "http",
--    "session_affinity_attribute": "",
--    "name": "calico-new-yz-alb-09999-eb1a18d0-8d44-4100-bbb8-93db3c02c482",
--    "session_affinity_policy": "",
--    "backends": [
--    {
--        "ns": "default",
--        "svc": "xx",
--        "port": 80,
--        "address": "10.16.12.11",
--        "weight": 100
--    }
--    ]
-- }

function _M.node_key(ep)
    return ep.address .. ":" .. ep.port
end
function _M.get_nodes(backend)
    local nodes = {}

    for _, endpoint in ipairs(backend.backends) do
        local endpoint_string = _M.node_key(endpoint)
        nodes[endpoint_string] = endpoint.weight
    end
    return nodes
end

return _M
