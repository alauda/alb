local _M = {}

-- return the new url
local sub = string.sub
function _M.replace(url, prefix_match, replace_prefix)
    if sub(url, 1, #prefix_match) ~= prefix_match then
        -- ngx.log(ngx.ERR, "not match " .. url .. " " .. prefix_match)
        return url
    end

    local left = sub(url, #prefix_match + 1, #url)
    local prefix = replace_prefix
    local prefix_endwith_slash = sub(prefix, -1) == "/"
    local left_startwith_slash = sub(left, 1, 1) == "/"

    local sep = "/"
    if prefix_endwith_slash and left_startwith_slash then
        prefix = sub(prefix, 1, #prefix - 1)
        sep = ""
    end
    if prefix_endwith_slash or left_startwith_slash then
        sep = ""
    end
    if left == "" then
        sep = ""
    end

    -- ngx.log(ngx.ERR, "xx " .. prefix .. " | " .. sep .. " | " .. left)
    -- ngx.log(ngx.ERR, "xx " .. tostring(prefix_endwith_slash) .. " " .. tostring(left_startwith_slash))
    local new_url = prefix .. sep .. left
    if new_url == "" then
        return "/"
    end
    return new_url
end
return _M
