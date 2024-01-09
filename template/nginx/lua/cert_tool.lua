-- format:on
local string_sub = string.sub
local string_find = string.find
local tonumber = tonumber
local ssl = require "ngx.ssl"
local ngx_shared = ngx.shared
local ngx_config = ngx.config
local common = require "utils.common"
local conf = require "conf"
local s_ext = require "utils.string_ext"
local cache = require "cache"
local subsystem = ngx_config.subsystem
local l2_cache = ngx_shared[subsystem .. "_certs_cache"]

local M = {}

function M.select_cert()
    local ok, err = ssl.clear_certs()
    if not ok then
        local msg = "failed to clear existing (fallback) certificates " .. tostring(err)
        ngx.log(ngx.ERR, msg)
        return
    end

    local host, port, err = M.get_host()
    if err ~= nil then
        ngx.log(ngx.ERR, err)
        return ngx.exit(ngx.ERROR)
    end

    cache.cert_cache:update(0.1)

    local key = tostring(host) .. "/" .. tostring(port)
    local cert = cache.cert_cache:get(key, nil, M.get_domain_cert, host, port)
    if cert == nil then
        ngx.log(ngx.ERR, "no cert found for ", key)
        return ngx.exit(ngx.ERROR)
    end

    M.set_cert(cert)
end

function M.get_host()
    local host_name, err = ssl.server_name()
    if err then
        return nil, nil, "get sni failed" .. tostring(err)
    end

    local port, err = ssl.server_port()
    if err then
        return nil, nil, "failed to read server port: " .. tostring(err)
    end
    -- NOTE: when the SNI name is missing from the client handshake request,
    -- we use the server IP address accessed by the client to identify the site
    if host_name ~= nil and tonumber(string_sub(host_name, -1)) ~= nil then
        host_name = nil
    end

    return host_name, port, nil
end

function M.get_domain_cert(domain, port)
    local raw_cert = M.get_domain_cert_raw(domain, port)
    if raw_cert ~= nil then
        return common.json_decode(raw_cert)
    end
    return nil, "cert not find"
end

function M.get_domain_cert_raw(domain, port)
    local cert = M.try_get_domain_cert_from_l2_cache(domain, port)
    if cert ~= nil then
        return cert
    end
    local default_strategy = conf.default_ssl_strategy
    local default_https_port = conf.default_https_port
    if (default_strategy == "Always" or default_strategy == "Both") then
        return M.get_cert(default_https_port)
    end
end

function M.try_get_domain_cert_from_l2_cache(domain_raw, port_raw)
    if domain_raw == nil and port_raw == nil then
        return nil
    end

    -- without sni,use port default cert
    if domain_raw == nil then
        local port_cert, find = M.get_cert(tostring(port_raw))
        if find then
            return port_cert
        end
        return nil
    end
    local domain_str = tostring(domain_raw)
    local port_str = tostring(port_raw)
    local cert_full_host, find = M.get_cert(domain_str)
    if find then
        return cert_full_host
    end

    local cert_full_host_in_port, find = M.get_cert(domain_str .. "/" .. port_str)
    if find then
        return cert_full_host_in_port
    end

    -- wildcard host cert
    local index = string_find(domain_str, ".", 1, true)
    if index == nil then
        ngx.log(ngx.ERR, "invalid domain " .. domain_str)
        return nil
    end

    local wildcard_host = "*" .. string.sub(domain_str, index, #domain_str)
    local cert_wildcard_host, find = M.get_cert(wildcard_host)
    if find then
        return cert_wildcard_host
    end

    local cert_wildcard_host_in_port, find = M.get_cert(wildcard_host .. "/" .. port_str)
    if find then
        return cert_wildcard_host_in_port
    end

    return M.get_cert(port_str)
end

function M.get_cert(key)
    local cert = l2_cache:get(key)
    if s_ext.is_nill(cert) then
        return nil, false
    end
    return cert, true
end

function M.set_cert(cert)
    local pem_cert_chain = cert["cert"]
    local pem_pkey = cert["key"]

    local der_cert_chain, err = ssl.cert_pem_to_der(pem_cert_chain)
    if not der_cert_chain then
        ngx.log(ngx.ERR, "failed to convert certificate chain ", "from PEM to DER: ", err)
        return ngx.exit(ngx.ERROR)
    end

    local ok, err = ssl.set_der_cert(der_cert_chain)
    if not ok then
        ngx.log(ngx.ERR, "failed to set DER cert: ", err)
        return ngx.exit(ngx.ERROR)
    end

    local der_pkey, err = ssl.priv_key_pem_to_der(pem_pkey)
    if not der_pkey then
        ngx.log(ngx.ERR, "failed to convert private key ", "from PEM to DER: ", err)
        return ngx.exit(ngx.ERROR)
    end

    local ok, err = ssl.set_der_priv_key(der_pkey)
    if not ok then
        ngx.log(ngx.ERR, "failed to set DER private key: ", err)
        return ngx.exit(ngx.ERROR)
    end
end

return M
