local string_lower = string.lower
local string_sub = string.sub
local string_find = string.find
local tonumber = tonumber
local ssl = require "ngx.ssl"
local ngx_shared = ngx.shared
local ngx_config = ngx.config
local common = require "utils.common"
local cache = require "cache"
local conf = require "conf"
local s_ext = require "utils.string_ext"

local subsystem = ngx_config.subsystem

-- clear the fallback certificates and private keys
-- set by the ssl_certificate and ssl_certificate_key
-- directives above:
local ok, err = ssl.clear_certs()
if not ok then
    ngx.log(ngx.ERR, "failed to clear existing (fallback) certificates " .. tostring(err))
    return ngx.exit(ngx.ERROR)
end

local host_name, err = ssl.server_name()
if err then
    ngx.log(ngx.ERR, "get sni failed", err)
    return ngx.exit(ngx.ERROR)
end
-- NOTE: when the SNI name is missing from the client handshake request,
-- we use the server IP address accessed by the client to identify the site
if host_name ~= nil and tonumber(string_sub(host_name, -1)) ~= nil then
    host_name = nil
end

local cert
local pem_cert_chain
local pem_pkey

if host_name == nil then
    -- no sni, try default cert
    -- host_name = server_port

    local port, err = ssl.server_port()
    if err then
        ngx.log(ngx.ERR, "failed to read server port: ", err)
        return ngx.exit(ngx.ERROR)
    end
    -- host_name = "443"
    host_name = port
end

host_name = string_lower(host_name)

local function get_domain_cert_raw(domain)
    local cache = ngx_shared[subsystem .. "_certs_cache"]
    local raw_cert = cache:get(domain)

    if not s_ext.is_nill(raw_cert) then
        return raw_cert
    end

    local index = string_find(domain, ".", 1, true)
    if index ~= nil then
        local wildcard = "*" .. string.sub(domain, index, #domain)
        local raw_cert = cache:get(wildcard)
        if not s_ext.is_nill(raw_cert) then
            return raw_cert
        end
    end

    if (conf.default_ssl_strategy == "Always" or conf.default_ssl_strategy == "Both") and domain ~= conf.default_https_port then
        return cache:get(conf.default_https_port)
    end
end

local function get_domain_cert(domain)
    local raw_cert = get_domain_cert_raw(domain)
    if raw_cert ~= nil then
        local ret = common.json_decode(raw_cert)
        return ret
    end
    return nil, "cert not find"
end

cache.cert_cache:update(0.1)
cert = cache.cert_cache:get(host_name, nil, get_domain_cert, host_name)
if cert ~= nil then
    pem_cert_chain = cert["cert"]
    pem_pkey = cert["key"]
else
    -- no cert found, abort
    ngx.log(ngx.ERR, "no cert found for ", host_name)
    ngx.exit(ngx.ERROR)
end

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
