local json = require "cjson"

local _M = {}

function _M.json_encode(data, empty_table_as_object )
  local json_value = nil
  if json.encode_empty_table_as_object then
    json.encode_empty_table_as_object(empty_table_as_object or false) -- 空的table默认为array
  end
  json.encode_sparse_array(true)
  pcall(function (data) json_value = json.encode(data) end, data)
  return json_value
end

function _M.json_decode(str)
	local json_value = nil
	pcall(function (str) json_value = json.decode(str) end,str)
	return json_value
end

return _M
