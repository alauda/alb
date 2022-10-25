std = 'ngx_lua'
codes = true
max_line_length = false
globals = {
    'ngx', 'ndk',
}
ignore= {"411","421","431"}
read_globals = {
    "coroutine._yield"
}
exclude_files = {"template/nginx/lua/vendor/**"}