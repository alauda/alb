https://jira.alauda.cn/browse/ACP-30564
测试
- 在发了rst之后会不会继续发包
- 在收到了rst之后后不会报502


 wrk -d3s -c1 -t1 -s ./x.lua http://127.0.0.1 -- 6
