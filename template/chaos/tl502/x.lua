---@diagnostic disable: undefined-global, lowercase-global
local counter = 1
local threads = {}

function setup(thread)
    -- 给每个线程设置一个 id 参数
    thread:set("id", counter)
    -- 将线程添加到 table 中
    table.insert(threads, thread)
    counter = counter + 1
end

function init(args)
    -- init and request use same lua env
    -- request responses reqs defined in global could be used in request ,and could be accessed via thread.get

   requests  = 0
   responses = 0
   reqs={}
   prefix=""
   if args[2] ~=nil then
    prefix=tostring(args[2])
   end
    for i = 1, args[1] do
        reqs[i] = wrk.format("GET", "/"..prefix.."/" .. tostring(id) .. "/" .. tostring(i))
        -- print(string.format("init req %d %s", i, reqs[i]))
    end
    -- 打印线程被创建的消息，打印完后，线程正式启动运行
    local msg = "thread %d created %d"
    print(msg:format(id, #reqs))
end

function request()
    -- 每发起一次请求 +1
    local count = requests + 1
    local req = reqs[count]
    -- print("req send " .. tostring(count) .. "/" .. #reqs .. " " .. tostring(req))
    if req ~= nil then
        requests = count
        return req
    end
    return nil
end

function response(status, headers, body)
    -- 每得到一次请求的响应 +1
    responses = responses + 1
end

function done(summary, latency, requests)
    -- 循环线程 table
    for index, thread in ipairs(threads) do
        local id = thread:get("id")
        local requests = thread:get("requests")
        local responses = thread:get("responses")
        -- 打印每个线程发起了多少个请求，得到了多少次响应
        print(string.format("thread %d made %s requests and got %s responses", id, tostring(requests), tostring(responses)))
    end
end
