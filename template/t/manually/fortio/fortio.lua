-- format:on
local _M = {}
local h = require "test-helper"
local u = require "util"

local default_443_cert =
    "-----BEGIN CERTIFICATE-----\nMIIFFTCCAv2gAwIBAgIUNcaMWCswms56XCvj8nxC/5AKxtUwDQYJKoZIhvcNAQEL\nBQAwGjEYMBYGA1UEAwwPNDQzLmRlZmF1bHQuY29tMB4XDTIyMDUxOTA5MjEzMVoX\nDTMyMDUxNjA5MjEzMVowGjEYMBYGA1UEAwwPNDQzLmRlZmF1bHQuY29tMIICIjAN\nBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAvMqeEYs9K/DrbZ3FXj7yZaVRexub\na4OF0S/jg2qXrTK8FwPQ1MJiVxPL2jNeE7PT1fCNujb3+fZ/way99FJ0KpmbqEeP\nGt8490oqHZl7LuiEklrSiNp5qOJERsxgNrtq5RILIC1wH9eu0dNilwnCldzEXqFJ\n4+vZqPQfNxk//0vSOxsapl/nEPze6aMy+sUnyFJoq3ti/O02sV/p5sOQX3NcPoXU\n23PTr1xMDVQ7IpuR4GkxbmIVdAMuGWA2udYN0H3ou1VVy+je3RVF7xD2V/lMI3RL\nzLinfWxyNBUOoswylWRjdgwfrz5EkGuN58uT+o28Lx0APw06gwZ0eXb0cKdaYM0X\n04p4d3r//KLgm5WZpvDrjCC3aP02Yk1rITAu9owx+fNjIuEuPJtfin6r9Cjed7xL\n9CgdlFONDkNPxMz52qnf9Jbuf4HPTa/jDw7ICG8FAR8RwljJ7ohFCmullfXtumpX\nbT8+4DK1+H1fqkLV4lCWtQwn8ULqqCDQJZszco5KcnNnenqKgNPLVe7t6/ZxDZ8j\nMYAyGIR+DMDp0tLjfHD26IjEzF/n3E0pZiTXFaRirjKcFd523qEWvZeZc0nSykhH\nBUYXgxh2Nqi3Cv9VxA6sHVto5GvQBWq0kl6Qo9IGof51+4HCm++8/bCpf2Gcv/kI\ny39JIHMSGCa4ztcCAwEAAaNTMFEwHQYDVR0OBBYEFDawzxBvztJOekhp/DU9GKo+\nsnc9MB8GA1UdIwQYMBaAFDawzxBvztJOekhp/DU9GKo+snc9MA8GA1UdEwEB/wQF\nMAMBAf8wDQYJKoZIhvcNAQELBQADggIBACd2Z9XyESvQ4MfYMUID2DCmuVGBDhyo\n8cN88nuy+plrcYSpsthp55C+dhhfJRocES0NkpIVojpiQiPQdAyLFKb1M1Mcd9bg\n+qtYrOH2lS0Uem2s366D8LLJSOzWv/f75wUHe3eyivzW73zcM3znr5TrAFrCkUBF\npkK90G1VEznpD+VDvXYfcXklTZ7lMVZJ1ck2MDYPkh3nGtCyY6z+r41vJo/OcW8A\ncxicgsKXjEiXOH42B8ugad5gK27gA/FKwtTNPPU4K0UeDCAJaY+L7USjbrUgeQ17\nmjCOrY53OjyjjD4YjsE9EqsU/Hc9lqIUdCktZEDrLKfjGT1raaqDlSzEYYcs/oai\n0Ka3MXao2czYEJz6YZIOtp7FatRUBajCZ3NJeTgPFMZn10g7CktJR5QJDvvqbUBs\nHCddmahNPdgQwjxGVfoAI5SDH2QnIlj3bLivU+4oqR7hO7Nmhx9BtNRdHhM+M+wp\nsLvVETvtZdHC3RX4rX4pAl/r7pjhC7n0tbn3XyK96yZ4Yu/E+d/Cqhs0+rssqLzH\nDtMZCMOsaZi1AUEtc2cmZweOXEHeEoyPn3nJeVLfW2+dThlK/i9RaZbPThTS/GdK\nCU530BEDG+y/I5p6dndYySm2+LJiC0Xso1S1gLa7NccV8Y1E9Y8026J3lpvMilhP\nBwA4jE77yBPI\n-----END CERTIFICATE-----";
local default_443_key =
    "-----BEGIN RSA PRIVATE KEY-----\n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\n-----END RSA PRIVATE KEY-----";
---@param size number number of rules
---@param http boolean  true->http, false->https
function _M.gen_policy(size, http)
    local rules = {}
    for i = 1, size - 1, 1 do
        rules[i] = {rule = "r" .. tostring(i), internal_dsl = {{"STARTS_WITH", "URL", "/match" .. tostring(i)}}, upstream = "test-upstream-1"}
    end
    rules[size] = {rule = "target", internal_dsl = {{"STARTS_WITH", "URL", "/"}}, upstream = "test-upstream-1"}
    local port = "80"
    if not http then
        port = "443"
    end
    local policy = {
        certificate_map = {["443"] = {cert = default_443_cert, key = default_443_key}},
        http = {tcp = {[port] = rules}},
        backend_group = {
            -- start fortio server via yourself
            {name = "test-upstream-1", mode = "http", backends = {{address = "127.0.0.1", port = 8080, weight = 100}}}
        }
    }
    return policy
end

function _M.gen_case(case)
    if case == "http_r1" then
        return 1, true
    end
    if case == "https_r1" then
        return 1, false
    end
    if case == "http_r500" then
        return 500, true
    end
    if case == "https_r500" then
        return 500, true
    end
    return 1, true
end

function _M.test(case, time)
    u.logs(case, time)
    local this = _M
    local policy = this.gen_policy(this.gen_case(case))
    u.logs(policy)
    require("policy_helper").set_policy_lua(policy)
    ngx.sleep(tonumber(time) - 3)
end

return _M
