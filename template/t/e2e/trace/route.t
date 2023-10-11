use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;


our $http_config = <<_EOC_;
server {
    listen 1880;
    location / {
       content_by_lua_block {
            require("e2e.trace.route").as_backend(1880)
      }
    }
}
_EOC_


log_level("info");
workers(4);
master_process_enabled("on");
no_shuffle();
no_root_location();
run_tests();

__DATA__

=== TEST 1: trace test
--- policy eval: ""
--- http_config eval: $::http_config
--- disable_init_worker
--- init_worker_eval: require("e2e.trace.route").init_worker()
--- timeout: 9999999
--- lua_test_eval: require("e2e.trace.route").test()
