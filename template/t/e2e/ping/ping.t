use strict;
use warnings;
use t::Alauda;
use Test::Nginx::Socket 'no_plan';
use Test::Nginx::Socket;

log_level("info");
master_process_enabled("on");
no_shuffle();
no_root_location();
run_tests();

__DATA__

=== TEST 1: ping
--- mock_backend: 1880 e2e.ping.ping
--- disable_init_worker
--- init_worker_eval: require("mock_worker_init").init_worker()
--- timeout: 9999999
--- lua_test_eval: require("e2e.ping.ping").test()
